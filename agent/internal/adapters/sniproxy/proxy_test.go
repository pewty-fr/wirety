package sniproxy

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// startProxy runs a proxy in front of a TLS test server and returns it with
// its listen address and the upstream server.
func startProxy(t *testing.T, hosts ...string) (*Proxy, string, *httptest.Server) {
	t.Helper()
	upstream := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "upstream "+r.Host)
	}))
	t.Cleanup(upstream.Close)

	p := New(strings.TrimPrefix(upstream.URL, "https://"), hosts)
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = l.Close() })
	go func() { _ = p.Serve(l) }()
	return p, l.Addr().String(), upstream
}

// get performs an HTTPS GET through the proxy with the given SNI (empty = no
// SNI) and returns the response body and the certificate the client saw.
func get(proxyAddr, sni string) (string, []byte, error) {
	conn, err := net.DialTimeout("tcp", proxyAddr, 5*time.Second)
	if err != nil {
		return "", nil, err
	}
	defer func() { _ = conn.Close() }()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))

	cfg := &tls.Config{InsecureSkipVerify: true} // #nosec G402 - test server certificate
	host := "example"
	if sni != "" {
		cfg.ServerName, host = sni, sni
	}
	tc := tls.Client(conn, cfg)
	if err := tc.Handshake(); err != nil {
		return "", nil, err
	}
	req, _ := http.NewRequest(http.MethodGet, "https://"+host+"/", nil)
	if err := req.Write(tc); err != nil {
		return "", nil, err
	}
	resp, err := http.ReadResponse(bufio.NewReader(tc), req)
	if err != nil {
		return "", nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	return string(body), tc.ConnectionState().PeerCertificates[0].Raw, err
}

func TestAllowedSNIIsRelayedEndToEnd(t *testing.T) {
	_, addr, upstream := startProxy(t, "vpn.example.internal")

	body, cert, err := get(addr, "VPN.example.internal.") // case and trailing dot are ignored
	if err != nil {
		t.Fatalf("allowed SNI: %v", err)
	}
	if body != "upstream VPN.example.internal." {
		t.Errorf("body = %q", body)
	}
	// TLS is end-to-end: the client sees the upstream's own certificate.
	if !bytes.Equal(cert, upstream.Certificate().Raw) {
		t.Error("client did not get the upstream certificate — the proxy must not terminate TLS")
	}
}

func TestOtherSNIIsRefused(t *testing.T) {
	_, addr, _ := startProxy(t, "vpn.example.internal")

	for _, sni := range []string{"docs.example.internal", ""} {
		if _, _, err := get(addr, sni); err == nil {
			t.Errorf("SNI %q was relayed; want the connection closed", sni)
		}
	}
}

func TestDynamicHosts(t *testing.T) {
	p, addr, _ := startProxy(t, "vpn.example.internal")

	if _, _, err := get(addr, "sso.example.internal"); err == nil {
		t.Fatal("issuer host relayed before being allowed")
	}
	p.SetDynamicHosts([]string{"sso.example.internal"})
	if _, _, err := get(addr, "sso.example.internal"); err != nil {
		t.Fatalf("issuer host after SetDynamicHosts: %v", err)
	}
	p.SetDynamicHosts(nil)
	if _, _, err := get(addr, "sso.example.internal"); err == nil {
		t.Fatal("issuer host still relayed after being removed")
	}
}

func TestNonTLSIsClosed(t *testing.T) {
	_, addr, _ := startProxy(t, "vpn.example.internal")

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	_, _ = io.WriteString(conn, "GET / HTTP/1.1\r\nHost: vpn.example.internal\r\n\r\n")
	if n, _ := conn.Read(make([]byte, 64)); n != 0 {
		t.Errorf("plain HTTP got %d response bytes; want the connection closed", n)
	}
}

func TestIPsAreNotAllowlisted(t *testing.T) {
	p := New("127.0.0.1:1", []string{"10.0.0.13", "vpn.example.internal"})
	if p.allowed("10.0.0.13") {
		t.Error("an IP literal must not be an allowed server name")
	}
}
