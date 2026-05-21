package ws

import (
	"net/http"

	"github.com/gorilla/websocket"
)

// Client implements WebSocketClientPort.
// Minimal wrapper to abstract library specifics.
type Client struct {
	conn   *websocket.Conn
	dialer *websocket.Dialer
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
	c.conn = conn
	return nil
}

func (c *Client) ReadMessage() ([]byte, error) {
	if c.conn == nil {
		return nil, websocket.ErrBadHandshake
	}
	_, msg, err := c.conn.ReadMessage()
	return msg, err
}

func (c *Client) WriteMessage(data []byte) error {
	if c.conn == nil {
		return websocket.ErrBadHandshake
	}
	return c.conn.WriteMessage(websocket.TextMessage, data)
}

// Ping sends a WebSocket control-frame Ping with an empty payload.  Cheap
// (6 bytes on the wire) and processed transparently by the gorilla server
// (auto-responds with Pong via the default PingHandler), so it's purely a
// keepalive — it does not look like an application message to the server.
func (c *Client) Ping() error {
	if c.conn == nil {
		return websocket.ErrBadHandshake
	}
	return c.conn.WriteMessage(websocket.PingMessage, nil)
}

func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}
