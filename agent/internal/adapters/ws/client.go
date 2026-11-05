package ws

import (
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// writeTimeout bounds a write: a connection whose other end stopped reading
// must not block the heartbeat goroutine (which also refreshes the endpoint
// table the captive portal relies on) forever.
const writeTimeout = 10 * time.Second

// readTimeout is how long the connection may stay silent: the agent pings
// every 5 s and the server answers each ping with a pong, so a connection
// without any frame for this long is dead (e.g. half-open behind a load
// balancer) and ReadMessage fails, which makes the agent reconnect. A variable
// for tests.
var readTimeout = 45 * time.Second

// Client implements WebSocketClientPort.
// Minimal wrapper to abstract library specifics.
type Client struct {
	conn   *websocket.Conn
	dialer *websocket.Dialer
	// writeMu serialises writes: gorilla/websocket allows one writer at a time.
	writeMu sync.Mutex
}

// NewClient returns a Client using the default WebSocket dialer.
func NewClient() *Client { return &Client{dialer: websocket.DefaultDialer} }

// NewClientWithDialer returns a Client using the provided dialer.
// Use this to customise TLS settings (e.g. skip certificate verification).
func NewClientWithDialer(dialer *websocket.Dialer) *Client {
	if dialer == nil {
		dialer = websocket.DefaultDialer
	}
	return &Client{dialer: dialer}
}

func (c *Client) Connect(url string, header http.Header) error {
	conn, _, err := c.dialer.Dial(url, header)
	if err != nil {
		return err
	}
	_ = conn.SetReadDeadline(time.Now().Add(readTimeout))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(readTimeout))
	})
	c.conn = conn
	return nil
}

func (c *Client) ReadMessage() ([]byte, error) {
	if c.conn == nil {
		return nil, websocket.ErrBadHandshake
	}
	_, msg, err := c.conn.ReadMessage()
	if err == nil {
		_ = c.conn.SetReadDeadline(time.Now().Add(readTimeout))
	}
	return msg, err
}

func (c *Client) WriteMessage(data []byte) error {
	if c.conn == nil {
		return websocket.ErrBadHandshake
	}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	_ = c.conn.SetWriteDeadline(time.Now().Add(writeTimeout))
	return c.conn.WriteMessage(websocket.TextMessage, data)
}

// Ping sends a WebSocket control-frame Ping with an empty payload.  Cheap
// (6 bytes on the wire) and processed transparently by the gorilla server
// (auto-responds with Pong via the default PingHandler), so it's purely a
// keepalive — it does not look like an application message to the server.
// The pong renews the read deadline: no pong for readTimeout means the
// connection is dead.
func (c *Client) Ping() error {
	if c.conn == nil {
		return websocket.ErrBadHandshake
	}
	return c.conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(writeTimeout))
}

func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}
