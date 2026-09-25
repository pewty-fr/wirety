// Package sniproxy is the pre-authentication gateway to the Wirety server.
//
// Unauthenticated peers must reach the Wirety server (captive portal, OIDC
// callback) before they are allowed anywhere else. The server often sits
// behind a shared reverse proxy / ingress, so allowing its IP:port would also
// expose every other virtual host on it. iptables cannot filter on the TLS
// server name (the SNI travels after the TCP handshake, once conntrack already
// considers the connection established), so the jump peer redirects those
// connections here instead.
//
// The proxy reads only the TLS ClientHello, checks its server name against an
// allowlist, and relays the connection byte for byte to the server. It never
// decrypts anything: TLS is end-to-end between the peer and the server.
package sniproxy

import (
	"bytes"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

const (
	helloTimeout = 10 * time.Second
	dialTimeout  = 10 * time.Second
)

// Proxy relays TLS connections whose SNI is allowlisted to a fixed upstream.
type Proxy struct {
	upstream string // host:port of the Wirety server

	mu      sync.RWMutex
	static  map[string]struct{}
	dynamic map[string]struct{}
}

// New returns a proxy forwarding to upstream ("host:port") the connections
// whose SNI is one of hosts.
func New(upstream string, hosts []string) *Proxy {
	return &Proxy{upstream: upstream, static: hostSet(hosts), dynamic: map[string]struct{}{}}
}

// SetDynamicHosts replaces the allowlisted names learnt at runtime (the OIDC
// issuer pushed by the server), on top of those given to New.
func (p *Proxy) SetDynamicHosts(hosts []string) {
	set := hostSet(hosts)
	p.mu.Lock()
	p.dynamic = set
	p.mu.Unlock()
}

func (p *Proxy) allowed(name string) bool {
	name = normalizeHost(name)
	p.mu.RLock()
	defer p.mu.RUnlock()
	_, ok := p.static[name]
	if !ok {
		_, ok = p.dynamic[name]
	}
	return ok
}

// Serve accepts connections on l until it is closed.
func (p *Proxy) Serve(l net.Listener) error {
	for {
		c, err := l.Accept()
		if err != nil {
			var ne net.Error
			if errors.As(err, &ne) && ne.Timeout() {
				continue
			}
			return err
		}
		go p.handle(c)
	}
}

func (p *Proxy) handle(client net.Conn) {
	defer func() { _ = client.Close() }()

	_ = client.SetReadDeadline(time.Now().Add(helloTimeout))
	name, hello, err := readClientHello(client)
	_ = client.SetReadDeadline(time.Time{})
	peer := client.RemoteAddr().String()
	if err != nil {
		log.Debug().Err(err).Str("peer", peer).Msg("sni proxy: no TLS ClientHello — closing")
		return
	}
	if !p.allowed(name) {
		log.Info().Str("peer", peer).Str("sni", name).Msg("sni proxy: server name not allowed before authentication — closing")
		return
	}

	upstream, err := net.DialTimeout("tcp", p.upstream, dialTimeout)
	if err != nil {
		log.Warn().Err(err).Str("upstream", p.upstream).Msg("sni proxy: cannot reach the Wirety server")
		return
	}
	defer func() { _ = upstream.Close() }()

	if _, err := upstream.Write(hello); err != nil {
		return
	}
	log.Debug().Str("peer", peer).Str("sni", name).Str("upstream", p.upstream).Msg("sni proxy: relaying")
	relay(client, upstream)
}

// relay copies both directions until both are done, propagating half-closes.
func relay(a, b net.Conn) {
	var wg sync.WaitGroup
	wg.Add(2)
	pipe := func(dst, src net.Conn) {
		defer wg.Done()
		_, _ = io.Copy(dst, src)
		if cw, ok := dst.(interface{ CloseWrite() error }); ok {
			_ = cw.CloseWrite()
		} else {
			_ = dst.Close()
		}
	}
	go pipe(a, b)
	go pipe(b, a)
	wg.Wait()
}

var errHelloRead = errors.New("client hello read")

// readClientHello reads the TLS ClientHello from c and returns its server
// name together with every byte consumed, to be replayed upstream. It lets
// crypto/tls do the parsing (multi-record and multi-segment hellos included)
// and aborts the handshake as soon as the hello has been parsed; nothing is
// ever written back to the client.
func readClientHello(c net.Conn) (string, []byte, error) {
	rec := &recordingConn{Conn: c}
	var name string
	var gotHello bool
	err := tls.Server(rec, &tls.Config{
		GetConfigForClient: func(h *tls.ClientHelloInfo) (*tls.Config, error) {
			name, gotHello = h.ServerName, true
			return nil, errHelloRead
		},
	}).Handshake()
	if !gotHello {
		if err == nil {
			err = errors.New("handshake ended without a ClientHello")
		}
		return "", nil, err
	}
	return name, rec.buf.Bytes(), nil
}

// recordingConn records what is read and refuses to write, so the aborted
// handshake above cannot send an alert to the client.
type recordingConn struct {
	net.Conn
	buf bytes.Buffer
}

func (r *recordingConn) Read(b []byte) (int, error) {
	n, err := r.Conn.Read(b)
	r.buf.Write(b[:n])
	return n, err
}

func (r *recordingConn) Write([]byte) (int, error) { return 0, errors.New("read-only") }

func hostSet(hosts []string) map[string]struct{} {
	set := make(map[string]struct{}, len(hosts))
	for _, h := range hosts {
		if h = normalizeHost(h); h != "" && net.ParseIP(h) == nil {
			set[h] = struct{}{}
		}
	}
	return set
}

func normalizeHost(h string) string {
	return strings.TrimSuffix(strings.ToLower(strings.TrimSpace(h)), ".")
}
