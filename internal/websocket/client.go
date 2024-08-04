package websocket

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"tonkey/internal/logger"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 512
)

// Client represents a WebSocket client
type Client struct {
	hub           *Hub
	conn          *websocket.Conn
	send          chan Message
	UserID        string
	subscriptions map[string]bool
	subMu         sync.RWMutex
}

// ClientMessage represents incoming messages from clients
type ClientMessage struct {
	Action    string   `json:"action"` // subscribe, unsubscribe, ping
	Addresses []string `json:"addresses,omitempty"`
}

// NewClient creates a new WebSocket client
func NewClient(hub *Hub, conn *websocket.Conn, userID string) *Client {
	return &Client{
		hub:           hub,
		conn:          conn,
		send:          make(chan Message, 256),
		UserID:        userID,
		subscriptions: make(map[string]bool),
	}
}

// IsSubscribed checks if client is subscribed to an address
func (c *Client) IsSubscribed(address string) bool {
	c.subMu.RLock()
	defer c.subMu.RUnlock()
	return c.subscriptions[address]
}

// Subscribe adds an address to subscriptions
func (c *Client) Subscribe(address string) {
	c.subMu.Lock()
	defer c.subMu.Unlock()
	c.subscriptions[address] = true
	logger.Debug.Printf("Client %s subscribed to %s", c.UserID, address)
}

// Unsubscribe removes an address from subscriptions
func (c *Client) Unsubscribe(address string) {
	c.subMu.Lock()
	defer c.subMu.Unlock()
	delete(c.subscriptions, address)
	logger.Debug.Printf("Client %s unsubscribed from %s", c.UserID, address)
}

// readPump pumps messages from the WebSocket connection to the hub
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				logger.Error.Printf("WebSocket error: %v", err)
			}
			break
		}

		var msg ClientMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			logger.Warn.Printf("Invalid WebSocket message from %s: %v", c.UserID, err)
			continue
		}

		c.handleMessage(msg)
	}
}

// handleMessage processes incoming client messages
func (c *Client) handleMessage(msg ClientMessage) {
	switch msg.Action {
	case "subscribe":
		for _, addr := range msg.Addresses {
			c.Subscribe(addr)
		}
		c.sendAck("subscribed", msg.Addresses)

	case "unsubscribe":
		for _, addr := range msg.Addresses {
			c.Unsubscribe(addr)
		}
		c.sendAck("unsubscribed", msg.Addresses)

	case "ping":
		c.sendAck("pong", nil)

	default:
		logger.Warn.Printf("Unknown action from client %s: %s", c.UserID, msg.Action)
	}
}

// sendAck sends an acknowledgment message
func (c *Client) sendAck(action string, addresses []string) {
	c.send <- Message{
		Type: action,
		Data: map[string]interface{}{
			"addresses": addresses,
			"time":      time.Now().Unix(),
		},
	}
}

// writePump pumps messages from the hub to the WebSocket connection
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}

			if err := json.NewEncoder(w).Encode(message); err != nil {
				logger.Error.Printf("Failed to encode message: %v", err)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// Start begins the client's read and write pumps
func (c *Client) Start() {
	go c.writePump()
	go c.readPump()
}
