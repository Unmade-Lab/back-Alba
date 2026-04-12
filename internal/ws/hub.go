package ws

import (
	"sync"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// Client wraps a single WebSocket connection.
type Client struct {
	conn        *websocket.Conn
	sessionID   string
	workspaceID string
	send        chan []byte
}

// Hub manages all active WebSocket connections grouped by session ID and workspace ID.
type Hub struct {
	mu           sync.RWMutex
	sessions     map[string]map[*Client]bool  // sessionID -> clients
	workspaces   map[string]map[*Client]bool  // workspaceID -> clients
	logger       *zap.Logger
}

// NewHub creates a new Hub.
func NewHub(logger *zap.Logger) *Hub {
	return &Hub{
		sessions:   make(map[string]map[*Client]bool),
		workspaces: make(map[string]map[*Client]bool),
		logger:     logger,
	}
}

// Register adds a connection to the session and workspace pools.
func (h *Hub) Register(sessionID string, conn *websocket.Conn) *Client {
	return h.RegisterWithWorkspace(sessionID, "", conn)
}

// RegisterWithWorkspace adds a connection tracked by both session and workspace.
func (h *Hub) RegisterWithWorkspace(sessionID, workspaceID string, conn *websocket.Conn) *Client {
	client := &Client{
		conn:        conn,
		sessionID:   sessionID,
		workspaceID: workspaceID,
		send:        make(chan []byte, 64),
	}

	h.mu.Lock()
	// Session tracking
	if h.sessions[sessionID] == nil {
		h.sessions[sessionID] = make(map[*Client]bool)
	}
	h.sessions[sessionID][client] = true

	// Workspace tracking
	if workspaceID != "" {
		if h.workspaces[workspaceID] == nil {
			h.workspaces[workspaceID] = make(map[*Client]bool)
		}
		h.workspaces[workspaceID][client] = true
	}
	h.mu.Unlock()

	go client.writePump(h)

	h.logger.Info("ws client registered", zap.String("session", sessionID), zap.String("workspace", workspaceID))
	return client
}

// Unregister removes a connection from all pools.
func (h *Hub) Unregister(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Remove from session pool
	if clients, ok := h.sessions[client.sessionID]; ok {
		delete(clients, client)
		close(client.send)
		if len(clients) == 0 {
			delete(h.sessions, client.sessionID)
		}
	}

	// Remove from workspace pool
	if client.workspaceID != "" {
		if clients, ok := h.workspaces[client.workspaceID]; ok {
			delete(clients, client)
			if len(clients) == 0 {
				delete(h.workspaces, client.workspaceID)
			}
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
			h.logger.Warn("ws send buffer full, dropping message", zap.String("session", sessionID))
		}
	}
}

// BroadcastToWorkspace sends a notification to ALL connections in a workspace.
// Used for system-wide events like deal updates, task reminders, etc.
func (h *Hub) BroadcastToWorkspace(workspaceID string, msg []byte) {
	h.mu.RLock()
	clients := h.workspaces[workspaceID]
	h.mu.RUnlock()

	sent := 0
	for client := range clients {
		select {
		case client.send <- msg:
			sent++
		default:
			h.logger.Warn("ws send buffer full for workspace broadcast", zap.String("workspace", workspaceID))
		}
	}
	h.logger.Debug("workspace broadcast sent", zap.String("workspace", workspaceID), zap.Int("recipients", sent))
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
