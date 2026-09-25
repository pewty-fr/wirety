//go:build e2e

package e2e

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	sniVPNHost  = "vpn.e2e.test"  // the Wirety server's virtual host
	sniDocsHost = "docs.e2e.test" // another app behind the same ingress
)

// TestE2ESNIProxy reproduces a Wirety server behind a shared TLS ingress: one
// IP:443 serves both the Wirety host and another application. The jump agent
// reaches the server by IP with --server-host, as in production. Before
// authentication a peer must reach the Wirety host (to sign in) but not the
// other application; after authentication both are reachable.
func TestE2ESNIProxy(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	st := setupStack(ctx, t)
	ingressIP := st.startIngress(ctx, t)

	userTok, err := dexPasswordToken(ctx, st.dexTokenURL, dexClientID, dexClientSecret, userEmail, userPassword)
	if err != nil {
		t.Fatalf("dex user token: %v", err)
	}
	owner, err := newAPIClient(st.apiBaseURL, userTok).me(ctx)
	if err != nil {
		t.Fatalf("user /me: %v", err)
	}

	net, err := st.admin.createNetwork(ctx, network{Name: "sni", CIDR: "10.95.0.0/24"})
	if err != nil {
		t.Fatalf("create network: %v", err)
	}
	jumpPeer, err := st.admin.createPeer(ctx, net.ID, createPeerReq{
		Name: "jump-1", IsJump: true, UseAgent: true, Endpoint: "10.255.255.254", ListenPort: 51820,
	})
	if err != nil {
		t.Fatalf("create jump peer: %v", err)
	}
	regPeer, err := st.admin.createPeer(ctx, net.ID, createPeerReq{Name: "peer-a", OwnerID: owner.ID})
	if err != nil {
		t.Fatalf("create peer: %v", err)
	}
	// Route the ingress through the jump so peer-a's traffic to it is gated.
	rt, err := st.admin.createRoute(ctx, net.ID, createRouteReq{
		Name: "to-ingress", DestinationCIDR: ingressIP + "/32", JumpPeerID: jumpPeer.ID,
	})
	if err != nil {
		t.Fatalf("create route: %v", err)
	}
	grp, err := st.admin.createGroup(ctx, net.ID, "users")
	if err != nil {
		t.Fatalf("create group: %v", err)
	}
	if err := st.admin.addPeerToGroup(ctx, net.ID, grp.ID, regPeer.ID); err != nil {
		t.Fatalf("add peer to group: %v", err)
	}
	if err := st.admin.attachRouteToGroup(ctx, net.ID, grp.ID, rt.ID); err != nil {
		t.Fatalf("attach route: %v", err)
	}

	// Configured like a production agent behind an ingress.
	jump := st.startJumpAgent(ctx, t, jumpPeer.Token,
		"-server", "https://"+ingressIP,
		"-server-host", sniVPNHost,
		"-portal-url", "https://"+sniVPNHost+"/captive-portal",
		"-skip-tls-verify",
	)

	jumpIP, err := jump.ContainerIP(ctx)
	if err != nil {
		t.Fatalf("jump container ip: %v", err)
	}
	cfg, err := st.admin.peerConfig(ctx, net.ID, regPeer.ID)
	if err != nil {
		t.Fatalf("fetch peer config: %v", err)
	}
	peer := st.startPeer(ctx, t, "peer-a")
	writeFileInContainer(ctx, t, peer, "/etc/wireguard/wg0.conf", clientConfig(cfg, fmt.Sprintf("%s:%d", jumpIP, jumpPeer.ListenPort)))
	mustExec(ctx, t, peer, "wg-quick", "up", "wg0")

	// --- before authentication ----------------------------------------------
	// The Wirety host is reachable through the proxy (TLS end to end)...
	eventually(t, defaultSyncTimeout, defaultPollInterval, func() error {
		if status := httpsProbe(ctx, t, peer, sniVPNHost, ingressIP, "/api/v1/health"); status != "200" {
			return fmt.Errorf("https://%s via the ingress before auth: want 200, got %s", sniVPNHost, status)
		}
		return nil
	})
	// ...but not the other application sharing its IP:443.
	if status := httpsProbe(ctx, t, peer, sniDocsHost, ingressIP, "/"); status != "000" {
		t.Fatalf("https://%s answered %s before authentication; the SNI proxy should refuse it", sniDocsHost, status)
	}
	// That is the agent's SNI proxy at work: unauthenticated peers' connections
	// to the server's IP:443 are redirected to it.
	if _, out, err := execInContainer(ctx, jump, "iptables", "-t", "nat", "-S", "WIRETY_SNI"); err != nil {
		t.Fatalf("dump WIRETY_SNI: %v", err)
	} else if !hasRule(out, "-d "+ingressIP+"/32", "--dport 443", "REDIRECT", "--to-ports 3129") {
		t.Fatalf("no SNI proxy redirect in WIRETY_SNI:\n%s", out)
	}

	// --- authenticate through the captive portal -----------------------------
	var startURL string
	eventually(t, defaultSyncTimeout, defaultPollInterval, func() error {
		status, location, err := httpProbe(ctx, peer, "http://"+ingressIP+"/")
		if err != nil {
			return err
		}
		if status != "302" || !strings.Contains(location, "/api/v1/captive-portal/start?token=") {
			return fmt.Errorf("want 302 to the captive portal, got %s %q", status, location)
		}
		startURL = location
		return nil
	})
	// The redirect targets the public host name; the test reaches the same
	// server directly.
	u, err := url.Parse(startURL)
	if err != nil {
		t.Fatalf("parse start url: %v", err)
	}
	u.Scheme, u.Host = "http", "server:8080"
	session, err := st.wiretySession(ctx, userEmail, userPassword)
	if err != nil {
		t.Fatalf("oidc login: %v", err)
	}
	if err := st.captivePortalLogin(ctx, u.String(), session); err != nil {
		t.Fatalf("captive portal login: %v", err)
	}

	// --- after authentication -------------------------------------------------
	eventually(t, defaultSyncTimeout, defaultPollInterval, func() error {
		if status := httpsProbe(ctx, t, peer, sniDocsHost, ingressIP, "/"); status != "200" {
			return fmt.Errorf("https://%s after auth: want 200, got %s", sniDocsHost, status)
		}
		return nil
	})
	if status := httpsProbe(ctx, t, peer, sniVPNHost, ingressIP, "/api/v1/health"); status != "200" {
		t.Fatalf("https://%s after auth: want 200, got %s", sniVPNHost, status)
	}
}

// httpsProbe GETs https://host/path from inside c, resolving host to ip, and
// returns the status code ("000" when the connection fails).
func httpsProbe(ctx context.Context, t *testing.T, c testcontainers.Container, host, ip, path string) string {
	t.Helper()
	_, out, err := execInContainer(ctx, c, "curl", "-sk", "-o", "/dev/null", "--max-time", "5",
		"-w", "%{http_code}", "--resolve", host+":443:"+ip, "https://"+host+path)
	if err != nil {
		t.Fatalf("curl %s: %v", host, err)
	}
	return strings.TrimSpace(out)
}

// startIngress runs a TLS reverse proxy standing in for a shared ingress:
// sniVPNHost proxies to the Wirety server (websocket included), sniDocsHost
// is another application. Returns its IP.
func (s *stack) startIngress(ctx context.Context, t *testing.T) string {
	t.Helper()
	certPEM, keyPEM := selfSignedCert(t, sniVPNHost, sniDocsHost)
	conf := `events {}
http {
  map $http_upgrade $connection_upgrade { default upgrade; '' close; }
  ssl_certificate     /etc/nginx/tls.crt;
  ssl_certificate_key /etc/nginx/tls.key;
  server {
    listen 443 ssl default_server;
    server_name ` + sniVPNHost + `;
    location / {
      proxy_pass http://server:8080;
      proxy_http_version 1.1;
      proxy_set_header Upgrade $http_upgrade;
      proxy_set_header Connection $connection_upgrade;
      proxy_set_header Host $host;
      proxy_read_timeout 1h;
    }
  }
  server {
    listen 443 ssl;
    server_name ` + sniDocsHost + `;
    location / { return 200 "docs\n"; }
  }
}
`
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:          "nginx:1.27-alpine",
			Networks:       []string{s.net.Name},
			NetworkAliases: map[string][]string{s.net.Name: {"ingress"}},
			ExposedPorts:   []string{"443/tcp"},
			Files: []testcontainers.ContainerFile{
				{Reader: strings.NewReader(conf), ContainerFilePath: "/etc/nginx/nginx.conf", FileMode: 0o644},
				{Reader: bytes.NewReader(certPEM), ContainerFilePath: "/etc/nginx/tls.crt", FileMode: 0o644},
				{Reader: bytes.NewReader(keyPEM), ContainerFilePath: "/etc/nginx/tls.key", FileMode: 0o600},
			},
			WaitingFor: wait.ForListeningPort("443/tcp").WithStartupTimeout(30 * time.Second),
		},
		Started: true,
	})
	if err != nil {
		t.Fatalf("start ingress: %v", err)
	}
	t.Cleanup(func() { _ = c.Terminate(context.Background()) })
	ip, err := c.ContainerIP(ctx)
	if err != nil {
		t.Fatalf("ingress ip: %v", err)
	}
	return ip
}

func selfSignedCert(t *testing.T, names ...string) (certPEM, keyPEM []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: names[0]},
		DNSNames:     names,
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
}
