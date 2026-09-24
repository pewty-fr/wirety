//go:build e2e

package e2e

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/netip"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/moby/moby/api/types/container"
	mobynet "github.com/moby/moby/api/types/network"
	"github.com/testcontainers/testcontainers-go"
	tcexec "github.com/testcontainers/testcontainers-go/exec"
	tcnetwork "github.com/testcontainers/testcontainers-go/network"
	"github.com/testcontainers/testcontainers-go/wait"
)

// Timeouts shared by the harness and assertions (kept in a non-test file so
// non-test helpers like iptables.go can reference them).
const (
	defaultSyncTimeout  = 60 * time.Second
	defaultPollInterval = 2 * time.Second
)

// Fixed values shared across the harness.
const (
	dexClientID     = "wirety"
	dexClientSecret = "wirety-secret"
	dexIssuer       = "http://dex:5556/dex"
	dexRedirectURI  = "http://localhost:5173/"
	adminEmail      = "admin@example.com"
	adminUser       = "admin"
	adminPassword   = "password"
	userEmail       = "user@example.com"
	userName        = "user"
	userPassword    = "password"

	pgUser = "wirety"
	pgPass = "wirety"
	pgDB   = "wirety"
)

// stack holds a running Wirety environment and the endpoints the test needs.
type stack struct {
	net    *testcontainers.DockerNetwork
	pg     testcontainers.Container
	dex    testcontainers.Container
	server testcontainers.Container

	// Host-reachable endpoints (via mapped ports).
	apiBaseURL   string // http://127.0.0.1:<port>/api/v1
	dexTokenURL  string // http://127.0.0.1:<port>/dex/token
	dexAuthURL   string // http://127.0.0.1:<port>/dex/auth
	serverOrigin string // http://127.0.0.1:<port>  (server root, for /auth/token & captive portal)

	// Host-side host:port of the in-network "dex:5556" / "server:8080", used by
	// inNetworkClient to speak in-network URLs from the test process.
	dexHostPort    string
	serverHostPort string

	admin *apiClient

	// ipv6 is set when the docker network is dual-stack (withIPv6).
	ipv6 bool
}

// stackOption customises setupStack.
type stackOption func(*stack)

// withIPv6 makes the docker network dual-stack (ULA subnet e2eIPv6Subnet), so
// containers get IPv6 addresses and IPv6 can be routed through the jump peer.
func withIPv6() stackOption {
	return func(s *stack) { s.ipv6 = true }
}

// e2eIPv6Subnet is the docker network's IPv6 subnet in dual-stack stacks.
const e2eIPv6Subnet = "fd00:e2e:6::/64"

// repoRoot resolves the repository root from this test file's location
// (test/e2e -> ../..), so Dockerfile build contexts are stable regardless of cwd.
func repoRoot(t *testing.T) string {
	t.Helper()
	abs, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	return abs
}

// setupStack brings up network + postgres + dex + server, waits for readiness,
// and returns a stack with an admin API client. Cleanup is registered via
// t.Cleanup.
func setupStack(ctx context.Context, t *testing.T, opts ...stackOption) *stack {
	t.Helper()
	root := repoRoot(t)
	st := &stack{}
	for _, o := range opts {
		o(st)
	}

	var netOpts []tcnetwork.NetworkCustomizer
	if st.ipv6 {
		netOpts = append(netOpts,
			tcnetwork.WithEnableIPv6(),
			tcnetwork.WithIPAM(&mobynet.IPAM{Config: []mobynet.IPAMConfig{
				{Subnet: netip.MustParsePrefix(e2eIPv6Subnet)},
			}}),
		)
	}
	nw, err := tcnetwork.New(ctx, netOpts...)
	if err != nil {
		t.Fatalf("create network: %v", err)
	}
	t.Cleanup(func() { _ = nw.Remove(context.Background()) })

	// --- postgres -----------------------------------------------------------
	pg, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:          "postgres:16-alpine",
			Networks:       []string{nw.Name},
			NetworkAliases: map[string][]string{nw.Name: {"postgres"}},
			Env: map[string]string{
				"POSTGRES_USER":     pgUser,
				"POSTGRES_PASSWORD": pgPass,
				"POSTGRES_DB":       pgDB,
			},
			WaitingFor: wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).WithStartupTimeout(60 * time.Second),
		},
		Started: true,
	})
	if err != nil {
		t.Fatalf("start postgres: %v", err)
	}
	t.Cleanup(func() { _ = pg.Terminate(context.Background()) })

	// --- dex ----------------------------------------------------------------
	// Reuse the repo's Dex image (ENTRYPOINT ["dex","serve","/app/config.yaml"])
	// but mount the e2e config (issuer http://dex:5556/dex) over the baked one.
	dexCfg := filepath.Join(root, "test", "e2e", "images", "dex-config.yaml")
	dex, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			FromDockerfile: testcontainers.FromDockerfile{
				Context:    filepath.Join(root, "dex"),
				Dockerfile: "Dockerfile",
				KeepImage:  true,
			},
			Networks:       []string{nw.Name},
			NetworkAliases: map[string][]string{nw.Name: {"dex"}},
			ExposedPorts:   []string{"5556/tcp"},
			Files: []testcontainers.ContainerFile{{
				HostFilePath:      dexCfg,
				ContainerFilePath: "/app/config.yaml",
				FileMode:          0o444,
			}},
			WaitingFor: wait.ForHTTP("/dex/.well-known/openid-configuration").
				WithPort("5556/tcp").WithStartupTimeout(60 * time.Second),
		},
		Started: true,
	})
	if err != nil {
		t.Fatalf("start dex: %v", err)
	}
	t.Cleanup(func() { _ = dex.Terminate(context.Background()) })

	// --- server -------------------------------------------------------------
	pgDSN := fmt.Sprintf("postgres://%s:%s@postgres:5432/%s?sslmode=disable", pgUser, pgPass, pgDB)
	server, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			FromDockerfile: testcontainers.FromDockerfile{
				Context:    filepath.Join(root, "server"),
				Dockerfile: "Dockerfile",
				KeepImage:  true,
			},
			Networks:       []string{nw.Name},
			NetworkAliases: map[string][]string{nw.Name: {"server"}},
			ExposedPorts:   []string{"8080/tcp"},
			Env: map[string]string{
				"HTTP_PORT":          "8080",
				"DB_ENABLED":         "true",
				"DB_DSN":             pgDSN,
				"AUTH_ENABLED":       "true",
				"AUTH_ISSUER_URL":    dexIssuer,
				"AUTH_CLIENT_ID":     dexClientID,
				"AUTH_CLIENT_SECRET": dexClientSecret,
				// The server is reached by the agent at http://server:8080 (in-network).
				"SERVER_URL": "http://server:8080",
				// Captive portal page — not actually loaded in tests, but must be a valid URL.
				"CAPTIVE_PORTAL_URL": "http://server:8080/captive-portal",
				// Plain HTTP in tests, so the session cookie must not require Secure.
				"COOKIE_SECURE": "false",
				"LOG_LEVEL":     "info",
			},
			WaitingFor: wait.ForHTTP("/api/v1/health").WithPort("8080/tcp").
				WithStartupTimeout(90 * time.Second),
		},
		Started: true,
	})
	if err != nil {
		t.Fatalf("start server: %v", err)
	}
	t.Cleanup(func() { _ = server.Terminate(context.Background()) })

	st.net, st.pg, st.dex, st.server = nw, pg, dex, server

	// Resolve host-reachable endpoints.
	serverHost, _ := server.Host(ctx)
	serverPort, err := server.MappedPort(ctx, "8080/tcp")
	if err != nil {
		t.Fatalf("server mapped port: %v", err)
	}
	st.serverHostPort = net.JoinHostPort(serverHost, serverPort.Port())
	st.serverOrigin = "http://" + st.serverHostPort
	st.apiBaseURL = st.serverOrigin + "/api/v1"

	dexHost, _ := dex.Host(ctx)
	dexPort, err := dex.MappedPort(ctx, "5556/tcp")
	if err != nil {
		t.Fatalf("dex mapped port: %v", err)
	}
	st.dexHostPort = net.JoinHostPort(dexHost, dexPort.Port())
	st.dexTokenURL = "http://" + st.dexHostPort + "/dex/token"
	st.dexAuthURL = "http://" + st.dexHostPort + "/dex/auth"

	// Obtain the admin Bearer (first user seen → administrator) and build the client.
	token, err := dexPasswordToken(ctx, st.dexTokenURL, dexClientID, dexClientSecret, adminEmail, adminPassword)
	if err != nil {
		t.Fatalf("dex admin token: %v", err)
	}
	st.admin = newAPIClient(st.apiBaseURL, token)
	if err := st.admin.health(ctx); err != nil {
		t.Fatalf("server health: %v", err)
	}
	// Register the admin now: users are created on their first authenticated
	// call, and only the first one is promoted to administrator.
	if _, err := st.admin.me(ctx); err != nil {
		t.Fatalf("register admin: %v", err)
	}
	return st
}

// startJumpAgent builds the root agent image and runs it as a privileged jump
// peer enrolled with the given token. It shares the harness network with alias
// "jump".
func (s *stack) startJumpAgent(ctx context.Context, t *testing.T, token string) testcontainers.Container {
	t.Helper()
	root := repoRoot(t)
	logs := &logBuffer{}
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			FromDockerfile: testcontainers.FromDockerfile{
				Context:    filepath.Join(root, "agent"),
				Dockerfile: "Dockerfile.e2e",
				KeepImage:  true,
			},
			Networks:           []string{s.net.Name},
			NetworkAliases:     map[string][]string{s.net.Name: {"jump"}},
			HostConfigModifier: s.wgHostConfig("NET_ADMIN", "SYS_MODULE"),
			Env: map[string]string{
				"LOG_LEVEL": "debug",
			},
			// Explicit flags: server URL + enrollment token + syncconf apply method.
			Cmd: []string{
				"-server", "http://server:8080",
				"-token", token,
				"-log-level", "debug",
			},
			// Stream logs continuously into a buffer: a one-shot Logs() read at
			// cleanup time proved to come back truncated in CI.
			LogConsumerCfg: &testcontainers.LogConsumerConfig{Consumers: []testcontainers.LogConsumer{logs}},
			WaitingFor:     wait.ForLog("starting DNS server").WithStartupTimeout(60 * time.Second),
		},
		Started: true,
	})
	if err != nil {
		t.Logf("===== jump-agent logs =====\n%s", logs.String())
		t.Fatalf("start jump agent: %v", err)
	}
	t.Cleanup(func() {
		// Agent logs are the first thing needed to debug a CI failure.
		if t.Failed() {
			t.Logf("===== jump-agent logs =====\n%s===== end jump-agent logs =====", logs.String())
		}
		_ = c.Terminate(context.Background())
	})
	return c
}

// wgHostConfig is the host config of containers that create WireGuard
// interfaces: privileged with the given capabilities and, in a dual-stack
// stack, IPv6 enabled for interfaces created after start (Docker leaves it
// disabled, so wg-quick could not assign the tunnel's IPv6 address).
//
// Privileges must be set here rather than through ContainerRequest.Privileged
// / CapAdd: testcontainers only applies those deprecated fields when no
// HostConfigModifier is given.
func (s *stack) wgHostConfig(caps ...string) func(*container.HostConfig) {
	return func(hc *container.HostConfig) {
		hc.Privileged = true
		hc.CapAdd = caps
		if s.ipv6 {
			hc.Sysctls = map[string]string{
				"net.ipv6.conf.all.disable_ipv6":     "0",
				"net.ipv6.conf.default.disable_ipv6": "0",
			}
		}
	}
}

// containerIPv6 returns a container's IPv6 address on the stack network.
func (s *stack) containerIPv6(ctx context.Context, t *testing.T, c testcontainers.Container) string {
	t.Helper()
	dc, ok := c.(*testcontainers.DockerContainer)
	if !ok {
		t.Fatalf("containerIPv6: unexpected container type %T", c)
	}
	inspect, err := dc.Inspect(ctx)
	if err != nil {
		t.Fatalf("inspect container: %v", err)
	}
	ep, ok := inspect.NetworkSettings.Networks[s.net.Name]
	if !ok || !ep.GlobalIPv6Address.IsValid() {
		t.Fatalf("container has no IPv6 address on network %s", s.net.Name)
	}
	return ep.GlobalIPv6Address.String()
}

// logBuffer is a testcontainers.LogConsumer that accumulates container output.
type logBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (l *logBuffer) Accept(entry testcontainers.Log) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.buf.Write(entry.Content)
}

func (l *logBuffer) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.String()
}

// startPrivateService runs an nginx container on the harness network to stand in
// for a private resource reachable through a route. Returns the container and
// its in-network IP.
func (s *stack) startPrivateService(ctx context.Context, t *testing.T, alias string) (testcontainers.Container, string) {
	t.Helper()
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:          "nginx:1.27-alpine",
			Networks:       []string{s.net.Name},
			NetworkAliases: map[string][]string{s.net.Name: {alias}},
			WaitingFor:     wait.ForListeningPort("80/tcp").WithStartupTimeout(30 * time.Second),
		},
		Started: true,
	})
	if err != nil {
		t.Fatalf("start private service: %v", err)
	}
	t.Cleanup(func() { _ = c.Terminate(context.Background()) })
	ip, err := c.ContainerIP(ctx)
	if err != nil {
		t.Fatalf("private service ip: %v", err)
	}
	return c, ip
}

// startPeer runs a bare WireGuard client container (privileged) that sleeps
// forever; the test brings its tunnel up with the given config and execs probes.
func (s *stack) startPeer(ctx context.Context, t *testing.T, alias string) testcontainers.Container {
	t.Helper()
	root := repoRoot(t)
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			FromDockerfile: testcontainers.FromDockerfile{
				Context:    filepath.Join(root, "test", "e2e", "images"),
				Dockerfile: "peer.Dockerfile",
				KeepImage:  true,
			},
			Networks:           []string{s.net.Name},
			NetworkAliases:     map[string][]string{s.net.Name: {alias}},
			HostConfigModifier: s.wgHostConfig("NET_ADMIN"),
			// `sleep infinity` emits no logs; readiness = able to exec.
			WaitingFor: wait.ForExec([]string{"true"}).WithStartupTimeout(20 * time.Second),
		},
		Started: true,
	})
	if err != nil {
		t.Fatalf("start peer: %v", err)
	}
	t.Cleanup(func() { _ = c.Terminate(context.Background()) })
	return c
}

// execInContainer runs a command in a container and returns combined output.
func execInContainer(ctx context.Context, c testcontainers.Container, cmd ...string) (int, string, error) {
	// Multiplexed strips Docker's 8-byte stream-frame headers so callers get
	// plain stdout+stderr text they can compare exactly.
	code, reader, err := c.Exec(ctx, cmd, tcexec.Multiplexed())
	if err != nil {
		return code, "", err
	}
	out, _ := io.ReadAll(reader)
	return code, string(out), nil
}

// mustExec runs a command and fails the test on a non-zero exit or error.
func mustExec(ctx context.Context, t *testing.T, c testcontainers.Container, cmd ...string) string {
	t.Helper()
	code, out, err := execInContainer(ctx, c, cmd...)
	if err != nil {
		t.Fatalf("exec %v: %v\noutput:\n%s", cmd, err, out)
	}
	if code != 0 {
		t.Fatalf("exec %v exited %d\noutput:\n%s", cmd, code, out)
	}
	return out
}

// writeFile writes content to a path inside a container (via a shell heredoc-free
// approach using `sh -c` and base64 to survive arbitrary bytes/newlines).
func writeFileInContainer(ctx context.Context, t *testing.T, c testcontainers.Container, path, content string) {
	t.Helper()
	// Use `printf %s` piped through the container by writing via `sh -c` with the
	// content passed as an argument is unsafe for special chars; instead copy.
	err := c.CopyToContainer(ctx, []byte(content), path, 0o600)
	if err != nil {
		t.Fatalf("copy file %s: %v", path, err)
	}
}

// eventually polls fn until it returns nil or the timeout elapses.
func eventually(t *testing.T, timeout, interval time.Duration, fn func() error) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var last error
	for time.Now().Before(deadline) {
		if last = fn(); last == nil {
			return
		}
		time.Sleep(interval)
	}
	t.Fatalf("condition not met within %s: %v", timeout, last)
}

// containsAll reports whether s contains every substring in subs.
func containsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		if !strings.Contains(s, sub) {
			return false
		}
	}
	return true
}
