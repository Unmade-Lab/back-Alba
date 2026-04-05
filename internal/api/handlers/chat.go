package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/Unmade-Lab/back-Alba/internal/chat"
	"github.com/Unmade-Lab/back-Alba/internal/ws"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// ChatHandler handles REST endpoints for the chat interface.
type ChatHandler struct {
	service *chat.Service
	logger  *zap.Logger
	hub     *ws.Hub
}

// NewChatHandler creates a ChatHandler.
func NewChatHandler(service *chat.Service, hub *ws.Hub, logger *zap.Logger) *ChatHandler {
	return &ChatHandler{
		service: service,
		hub:     hub,
		logger:  logger,
	}
}

// SendMessage handles POST /chat/message
//
//	@Summary		Send a chat message
//	@Description	Accepts a user message, processes it through the AI orchestrator, and returns a structured response with an optional UI widget.
//	@Tags			chat
//	@Accept			json
//	@Produce		json
//	@Param			body	body		chat.MessageRequest		true	"Message payload"
//	@Success		200		{object}	chat.MessageResponse
//	@Failure		400		{object}	map[string]string
//	@Failure		500		{object}	map[string]string
//	@Router			/chat/message [post]
func (h *ChatHandler) SendMessage(c *gin.Context) {
	var req chat.MessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	response, err := h.service.Process(c.Request.Context(), req)
	if err != nil {
		h.logger.Error("process message", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to process message"})
		return
	}
	data, _ := json.Marshal(response)
	h.hub.Broadcast(req.SessionID, data)
	c.JSON(http.StatusOK, response)
}

// GetHistory handles GET /chat/history?session_id=...&limit=...
//
//	@Summary		Get chat history
//	@Description	Returns paginated message history for a session, ordered oldest-first.
//	@Tags			chat
//	@Produce		json
//	@Param			session_id	query	string	true	"Session ID"
//	@Param			limit		query	int		false	"Max messages (default 50)"
//	@Success		200			{array}	models.ChatMessage
//	@Failure		400			{object}	map[string]string
//	@Failure		500			{object}	map[string]string
//	@Router			/chat/history [get]
func (h *ChatHandler) GetHistory(c *gin.Context) {
	sessionID := c.Query("session_id")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "session_id is required"})
		return
	}

	limit := 50
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	messages, err := h.service.GetHistory(c.Request.Context(), sessionID, limit)
	if err != nil {
		h.logger.Error("get history", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch history"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"session_id": sessionID,
		"messages":   messages,
		"count":      len(messages),
	})
}
