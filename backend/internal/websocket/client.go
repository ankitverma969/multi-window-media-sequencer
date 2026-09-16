package websocket

import (
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer.
	maxMessageSize = 512 * 1024 // 512 KB

	// Size of outbound message buffer channel.
	sendBufferSize = 256
)

// Client represents a single active WebSocket connection.
type Client struct {
	hub          *Hub
	conn         *websocket.Conn
	send         chan []byte
	windowNumber int
	mu           sync.RWMutex
	closed       bool
}

// NewClient constructs a new WebSocket client instance.
func NewClient(hub *Hub, conn *websocket.Conn, windowNumber int) *Client {
	return &Client{
		hub:          hub,
		conn:         conn,
		send:         make(chan []byte, sendBufferSize),
		windowNumber: windowNumber,
	}
}

// WindowNumber returns the currently registered window number for this client.
func (c *Client) WindowNumber() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.windowNumber
}

// SetWindowNumber updates the client's associated window number.
func (c *Client) SetWindowNumber(windowNumber int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.windowNumber = windowNumber
}

// SendEnvelope marshals the envelope to JSON and queues it for the client's writePump.
func (c *Client) SendEnvelope(env Envelope) {
	data, err := json.Marshal(env)
	if err != nil {
		slog.Error("failed to marshal websocket envelope", "error", err, "type", env.Type)
		return
	}

	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.closed {
		return
	}

	select {
	case c.send <- data:
	default:
		slog.Warn("client send buffer full, dropping message and flagging disconnect",
			"window", c.windowNumber)
		go c.Close()
	}
}

// Close gracefully closes the client connection and buffered channel.
func (c *Client) Close() {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	c.closed = true
	close(c.send)
	c.mu.Unlock()

	if c.conn != nil {
		_ = c.conn.Close()
	}
}

// ReadPump pumps messages from the websocket connection to the hub.
// Application runs ReadPump in a per-connection goroutine.
func (c *Client) ReadPump() {
	defer func() {
		c.hub.Unregister(c)
		c.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				slog.Debug("websocket read error", "error", err, "window", c.WindowNumber())
			}
			break
		}

		// Handle client-originated messages
		var inbound ClientInboundMessage
		if err := json.Unmarshal(message, &inbound); err != nil {
			slog.Debug("unrecognized client message", "error", err)
			continue
		}

		switch inbound.Type {
		case EventRegisterWindow:
			if inbound.Payload.WindowNumber > 0 {
				c.hub.RegisterWindow(c, inbound.Payload.WindowNumber)
			}
		case EventPing:
			c.SendEnvelope(NewEnvelope(EventPong, map[string]string{"status": "ok"}))
		default:
			// Client messages other than registration/ping are ignored
		}
	}
}

// WritePump pumps messages from the client's send channel to the websocket connection.
// A goroutine running WritePump is started for each connection.
// This guarantees that AT MOST ONE goroutine writes to the connection at any time.
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// Hub or client closed the channel
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			if _, err := w.Write(message); err != nil {
				return
			}

			// Drain any additional queued messages into the same write buffer for efficiency
			n := len(c.send)
			for i := 0; i < n; i++ {
				_, _ = w.Write([]byte{'\n'})
				_, _ = w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
