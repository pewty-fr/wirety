package ws

import (
	"github.com/gorilla/websocket"
)

// Client implements WebSocketClientPort.
// Minimal wrapper to abstract library specifics.
type Client struct {
	conn *websocket.Conn
}

func NewClient() *Client { return &Client{} }

func (c *Client) Connect(url string) error {
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
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

func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}
