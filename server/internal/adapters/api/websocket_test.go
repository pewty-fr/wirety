package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"wirety/internal/adapters/api/middleware"
	"wirety/internal/adapters/db/memory"
	"wirety/internal/application/ipam"
	appnetwork "wirety/internal/application/network"
	"wirety/internal/config"
	domain "wirety/internal/domain/network"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// jumpAgent is a jump peer's agent connecting to the server's WebSocket.
type jumpAgent struct {
	h         *Handler
	wsURL     string
	token     string
	networkID string
	jumpID    string
}

func newJumpAgent(t *testing.T) *jumpAgent {
	t.Helper()
	gin.SetMode(gin.TestMode)
	ctx := context.Background()
	users := memory.NewUserRepository()
	service := appnetwork.NewService(memory.NewRepository(), memory.NewIPAMRepository(ctx), users, nil, nil, nil, nil)
	authCfg := &config.AuthConfig{}
	h := NewHandler(service, ipam.NewService(), nil, nil, nil, nil, nil, nil, users, authCfg)
	r := gin.New()
	h.RegisterRoutes(r, middleware.AuthMiddleware(nil, users, authCfg), middleware.RequireAdmin(), middleware.RequireNetworkAccess())
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)

	n, err := service.CreateNetwork(ctx, &domain.NetworkCreateRequest{Name: "dev", CIDR: "10.10.0.0/24"})
	if err != nil {
		t.Fatal(err)
	}
	jump, err := service.AddPeer(ctx, n.ID, &domain.PeerCreateRequest{Name: "jump", IsJump: true, UseAgent: true}, "")
	if err != nil {
		t.Fatal(err)
	}
	return &jumpAgent{h: h, wsURL: "ws" + strings.TrimPrefix(srv.URL, "http") + "/api/v1/ws", token: jump.Token, networkID: n.ID, jumpID: jump.ID}
}

// connect opens a WebSocket as the jump peer's agent and reads the initial
// configuration the server sends.
func (a *jumpAgent) connect(t *testing.T) *websocket.Conn {
	t.Helper()
	conn, _, err := websocket.DefaultDialer.Dial(a.wsURL, http.Header{"Authorization": {"Bearer " + a.token}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if _, msg := readWithin(t, conn, 2*time.Second); !strings.Contains(string(msg), `"config"`) {
		t.Fatalf("initial message: %s", msg)
	}
	return conn
}

func readWithin(t *testing.T, conn *websocket.Conn, d time.Duration) (int, []byte) {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(d))
	kind, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("no message within %s: %v", d, err)
	}
	return kind, msg
}

// An agent that reconnects before the server notices its old connection drop
// (load balancer timeout, network blip) must keep receiving pushes: when the
// old connection finally closes, it must not take the new one with it — the
// jump peer would then never learn of new sign-ins, and redirect every user
// with consumed tokens ("invalid token") until the agent restarts.
func TestPushesReachAnAgentThatReconnected(t *testing.T) {
	a := newJumpAgent(t)
	old := a.connect(t)
	fresh := a.connect(t)

	_ = old.Close() // the server notices the old connection drop late
	time.Sleep(200 * time.Millisecond)

	if !a.h.wsManager.IsConnected(a.networkID, a.jumpID) {
		t.Fatal("the jump peer is no longer registered after its old connection closed")
	}
	a.h.wsManager.NotifyPeerUpdate(a.networkID, a.jumpID)
	if _, msg := readWithin(t, fresh, 2*time.Second); !strings.Contains(string(msg), `"config"`) {
		t.Errorf("push: %s", msg)
	}
}

// The replaced connection is closed by the server, so it stops reading on it.
func TestReconnectingClosesThePreviousConnection(t *testing.T) {
	a := newJumpAgent(t)
	old := a.connect(t)
	a.connect(t)

	_ = old.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, _, err := old.ReadMessage(); err == nil || strings.Contains(err.Error(), "timeout") {
		t.Errorf("old connection still open: %v", err)
	}
}

// Pushes come from many goroutines; gorilla/websocket allows a single writer.
func TestConcurrentPushesToOneAgent(t *testing.T) {
	a := newJumpAgent(t)
	conn := a.connect(t)

	const pushes = 20
	var wg sync.WaitGroup
	for i := 0; i < pushes; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			a.h.wsManager.NotifyPeerUpdate(a.networkID, a.jumpID)
		}()
	}
	for i := 0; i < pushes; i++ {
		readWithin(t, conn, 5*time.Second)
	}
	wg.Wait()
}
