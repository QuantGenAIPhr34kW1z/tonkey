package websocket

import (
	"encoding/json"
	"sync"
	"time"

	"tonkey/internal/logger"
)

// Hub maintains active WebSocket connections and broadcasts messages
type Hub struct {
	clients    map[*Client]bool
	broadcast  chan Message
	register   chan *Client
	unregister chan *Client
	mu         sync.RWMutex
}

// Message represents a WebSocket message
type Message struct {
	Type    string      `json:"type"`
	Data    interface{} `json:"data"`
	Address string      `json:"address,omitempty"`
}

// NewHub creates a new WebSocket hub
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan Message, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

// Run starts the hub's main loop
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			logger.Info.Printf("WebSocket client connected: %s", client.UserID)

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
				logger.Info.Printf("WebSocket client disconnected: %s", client.UserID)
			}
			h.mu.Unlock()

		case message := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				// Check if client is subscribed to this address
				if message.Address != "" && !client.IsSubscribed(message.Address) {
					continue
				}

				select {
				case client.send <- message:
				default:
					// Client's send buffer is full, close the connection
					h.mu.RUnlock()
					h.mu.Lock()
					close(client.send)
					delete(h.clients, client)
					h.mu.Unlock()
					h.mu.RLock()
				}
			}
			h.mu.RUnlock()
		}
	}
}

// Broadcast sends a message to all subscribed clients
func (h *Hub) Broadcast(msg Message) {
	h.broadcast <- msg
}

// BroadcastBalanceUpdate notifies clients of a balance change
func (h *Hub) BroadcastBalanceUpdate(address string, newBalance int64) {
	h.Broadcast(Message{
		Type:    "balance_update",
		Address: address,
		Data: map[string]interface{}{
			"address": address,
			"balance": newBalance,
			"time":    time.Now().Unix(),
		},
	})
}

// BroadcastTransaction notifies clients of a new transaction
func (h *Hub) BroadcastTransaction(txID, from, to string, amount int64, status string) {
	h.Broadcast(Message{
		Type:    "transaction",
		Address: from, // Send to sender
		Data: map[string]interface{}{
			"tx_id":  txID,
			"from":   from,
			"to":     to,
			"amount": amount,
			"status": status,
			"time":   time.Now().Unix(),
		},
	})

	// Also send to receiver
	h.Broadcast(Message{
		Type:    "transaction",
		Address: to,
		Data: map[string]interface{}{
			"tx_id":  txID,
			"from":   from,
			"to":     to,
			"amount": amount,
			"status": status,
			"time":   time.Now().Unix(),
		},
	})
}

// GetClientCount returns the number of connected clients
func (h *Hub) GetClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// Stats returns hub statistics
func (h *Hub) Stats() map[string]interface{} {
	h.mu.RLock()
	defer h.mu.RUnlock()

	totalSubscriptions := 0
	for client := range h.clients {
		totalSubscriptions += len(client.subscriptions)
	}

	return map[string]interface{}{
		"connected_clients":   len(h.clients),
		"total_subscriptions": totalSubscriptions,
	}
}
