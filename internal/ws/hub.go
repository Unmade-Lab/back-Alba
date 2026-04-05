package ws

import (
	"sync"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// Client wraps a single WebSocket connection.
type Client struct {
	conn      *websocket.Conn
	sessionID string
	send      chan []byte
}

// Hub manages all active WebSocket connections grouped by session ID.
type Hub struct {
	mu       sync.RWMutex
	sessions map[string]map[*Client]bool
	logger   *zap.Logger
}

// NewHub creates a new Hub.
func NewHub(logger *zap.Logger) *Hub {
	return &Hub{
		sessions: make(map[string]map[*Client]bool),
		logger:   logger,
	}
}

// Register adds a connection to the session pool.
func (h *Hub) Register(sessionID string, conn *websocket.Conn) *Client {
	client := &Client{
		conn:      conn,
		sessionID: sessionID,
		send:      make(chan []byte, 32),
	}

	h.mu.Lock()
	if h.sessions[sessionID] == nil {
		h.sessions[sessionID] = make(map[*Client]bool)
	}
	h.sessions[sessionID][client] = true
	h.mu.Unlock()

	// Start writer goroutine.
	go client.writePump(h)

	h.logger.Info("ws client registered", zap.String("session", sessionID))
	return client
}

// Unregister removes a connection from the session pool.
func (h *Hub) Unregister(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if clients, ok := h.sessions[client.sessionID]; ok {
		delete(clients, client)
		close(client.send)
		if len(clients) == 0 {
			delete(h.sessions, client.sessionID)
		}
	}
	h.logger.Info("ws client unregistered", zap.String("session", client.sessionID))
}

// Broadcast sends a message to all connections belonging to a session.
func (h *Hub) Broadcast(sessionID string, msg []byte) {
	h.mu.RLock()
	clients := h.sessions[sessionID]
	h.mu.RUnlock()

	for client := range clients {
		select {
		case client.send <- msg:
		default:
			// Slow client — drop message.
			h.logger.Warn("ws send buffer full, dropping message", zap.String("session", sessionID))
		}
	}
}

// writePump forwards messages from the send channel to the WebSocket connection.
func (c *Client) writePump(h *Hub) {
	defer func() {
		c.conn.Close()
		h.Unregister(c)
	}()
	for msg := range c.send {
		if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			return
		}
	}
}
