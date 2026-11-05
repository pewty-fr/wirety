package ws

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestNewClient(t *testing.T) {
	client := NewClient()
	if client == nil {
		t.Error("Expected client to be created, got nil")
		return
	}
	if client.conn != nil {
		t.Error("Expected connection to be nil initially")
	}
}

func TestClient_ConnectAndClose(t *testing.T) {
	// Create a test WebSocket server
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true // Allow all origins for testing
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("Failed to upgrade connection: %v", err)
			return
		}
		defer func() { _ = conn.Close() }()

		// Echo messages back
		for {
			messageType, message, err := conn.ReadMessage()
			if err != nil {
				break
			}
			err = conn.WriteMessage(messageType, message)
			if err != nil {
				break
			}
		}
	}))
	defer server.Close()

	// Convert HTTP URL to WebSocket URL
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	client := NewClient()

	// Test Connect
	err := client.Connect(wsURL, nil)
	if err != nil {
		t.Errorf("Failed to connect: %v", err)
	}

	if client.conn == nil {
		t.Error("Expected connection to be established")
	}

	// Test Close
	err = client.Close()
	if err != nil {
		t.Errorf("Failed to close connection: %v", err)
	}
}

func TestClient_WriteAndReadMessage(t *testing.T) {
	// Create a test WebSocket server that echoes messages
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("Failed to upgrade connection: %v", err)
			return
		}
		defer func() { _ = conn.Close() }()

		// Echo messages back
		for {
			messageType, message, err := conn.ReadMessage()
			if err != nil {
				break
			}
			err = conn.WriteMessage(messageType, message)
			if err != nil {
				break
			}
		}
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	client := NewClient()
	err := client.Connect(wsURL, nil)
	if err != nil {
		t.Errorf("Failed to connect: %v", err)
	}
	defer func() { _ = client.Close() }()

	// Test WriteMessage
	testMessage := []byte("Hello, WebSocket!")
	err = client.WriteMessage(testMessage)
	if err != nil {
		t.Errorf("Failed to write message: %v", err)
	}

	// Test ReadMessage
	receivedMessage, err := client.ReadMessage()
	if err != nil {
		t.Errorf("Failed to read message: %v", err)
	}

	if string(receivedMessage) != string(testMessage) {
		t.Errorf("Expected message '%s', got '%s'", string(testMessage), string(receivedMessage))
	}
}

func TestClient_WriteMessageWithoutConnection(t *testing.T) {
	client := NewClient()

	// Test WriteMessage without connection
	err := client.WriteMessage([]byte("test"))
	if err == nil {
		t.Error("Expected error when writing message without connection")
	}

	if err != websocket.ErrBadHandshake {
		t.Errorf("Expected ErrBadHandshake, got %v", err)
	}
}

func TestClient_ReadMessageWithoutConnection(t *testing.T) {
	client := NewClient()

	// Test ReadMessage without connection
	_, err := client.ReadMessage()
	if err == nil {
		t.Error("Expected error when reading message without connection")
	}

	if err != websocket.ErrBadHandshake {
		t.Errorf("Expected ErrBadHandshake, got %v", err)
	}
}

func TestClient_CloseWithoutConnection(t *testing.T) {
	client := NewClient()

	// Test Close without connection (should not error)
	err := client.Close()
	if err != nil {
		t.Errorf("Unexpected error when closing without connection: %v", err)
	}
}

func TestClient_ConnectInvalidURL(t *testing.T) {
	client := NewClient()

	// Test Connect with invalid URL
	err := client.Connect("invalid-url", nil)
	if err == nil {
		t.Error("Expected error when connecting to invalid URL")
	}
}

func TestClient_MultipleMessages(t *testing.T) {
	// Create a test WebSocket server that echoes messages
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("Failed to upgrade connection: %v", err)
			return
		}
		defer func() { _ = conn.Close() }()

		// Echo messages back
		for {
			messageType, message, err := conn.ReadMessage()
			if err != nil {
				break
			}
			err = conn.WriteMessage(messageType, message)
			if err != nil {
				break
			}
		}
	}))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	client := NewClient()
	err := client.Connect(wsURL, nil)
	if err != nil {
		t.Errorf("Failed to connect: %v", err)
	}
	defer func() { _ = client.Close() }()

	// Test multiple messages
	messages := []string{"message1", "message2", "message3"}

	for _, msg := range messages {
		err = client.WriteMessage([]byte(msg))
		if err != nil {
			t.Errorf("Failed to write message '%s': %v", msg, err)
		}

		receivedMessage, err := client.ReadMessage()
		if err != nil {
			t.Errorf("Failed to read message: %v", err)
		}

		if string(receivedMessage) != msg {
			t.Errorf("Expected message '%s', got '%s'", msg, string(receivedMessage))
		}
	}
}

// wsServer upgrades every request and hands the connection to serve.
func wsServer(t *testing.T, serve func(*websocket.Conn)) string {
	t.Helper()
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		serve(conn)
	}))
	t.Cleanup(srv.Close)
	return "ws" + strings.TrimPrefix(srv.URL, "http")
}

func withReadTimeout(t *testing.T, d time.Duration) {
	t.Helper()
	previous := readTimeout
	readTimeout = d
	t.Cleanup(func() { readTimeout = previous })
}

// A server that stops answering (half-open connection behind a load balancer)
// makes ReadMessage fail, so the agent reconnects instead of waiting forever.
func TestClient_ReadFailsWhenTheServerGoesSilent(t *testing.T) {
	withReadTimeout(t, 300*time.Millisecond)
	url := wsServer(t, func(conn *websocket.Conn) {
		conn.SetPingHandler(func(string) error { return nil }) // never answers pings
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	})
	client := NewClient()
	if err := client.Connect(url, nil); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = client.Close() }()

	done := make(chan error, 1)
	go func() {
		_, err := client.ReadMessage()
		done <- err
	}()
	for i := 0; i < 5; i++ {
		_ = client.Ping()
		time.Sleep(100 * time.Millisecond)
	}
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("ReadMessage returned a message from a silent server")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("ReadMessage still blocked on a silent connection")
	}
}

// The server's pongs keep a quiet connection alive.
func TestClient_PongsKeepTheConnectionAlive(t *testing.T) {
	withReadTimeout(t, 300*time.Millisecond)
	url := wsServer(t, func(conn *websocket.Conn) {
		go func() { // default ping handler: answers each ping with a pong
			for {
				if _, _, err := conn.ReadMessage(); err != nil {
					return
				}
			}
		}()
		time.Sleep(time.Second) // longer than readTimeout
		_ = conn.WriteMessage(websocket.TextMessage, []byte("push"))
		time.Sleep(200 * time.Millisecond)
	})
	client := NewClient()
	if err := client.Connect(url, nil); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = client.Close() }()

	stop := make(chan struct{})
	defer close(stop)
	go func() {
		for {
			select {
			case <-stop:
				return
			case <-time.After(100 * time.Millisecond):
				_ = client.Ping()
			}
		}
	}()
	msg, err := client.ReadMessage()
	if err != nil || string(msg) != "push" {
		t.Fatalf("ReadMessage = %q, %v; want the push", msg, err)
	}
}
