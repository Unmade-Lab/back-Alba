package handlers

import (
	"fmt"
	"net/http"

	"github.com/Unmade-Lab/back-Alba/internal/ws"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// Allow all origins in dev; restrict in production via config.
	CheckOrigin: func(r *http.Request) bool { return true },
}

// WSHandler manages WebSocket connections for real-time chat updates.
type WSHandler struct {
	hub    *ws.Hub
	logger *zap.Logger
}

// NewWSHandler creates a WSHandler.
func NewWSHandler(hub *ws.Hub, logger *zap.Logger) *WSHandler {
	return &WSHandler{hub: hub, logger: logger}
}

// Connect handles GET /ws/chat?session_id=...
// Upgrades the HTTP connection to WebSocket and registers it with the hub.
// The server uses this connection only to push outbound messages;
// clients should send messages via the REST endpoint.
func (h *WSHandler) Connect(c *gin.Context) {
	sessionID := c.Query("session_id")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "session_id is required"})
		return
	}
	fmt.Println("WebSocket connection established for session:", sessionID)
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		h.logger.Error("websocket upgrade", zap.Error(err))
		return
	}

	client := h.hub.Register(sessionID, conn)

	// Read loop — keeps the connection alive and detects disconnects.
	// We don't process inbound WS messages (chat goes through REST).
	go func() {
		defer h.hub.Unregister(client)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
					h.logger.Warn("ws unexpected close", zap.String("session", sessionID), zap.Error(err))
				}
				return
			}
		}
	}()
}
