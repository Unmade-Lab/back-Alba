package api

import (
	"net/http"

	"github.com/Unmade-Lab/back-Alba/internal/api/handlers"
	"github.com/Unmade-Lab/back-Alba/internal/api/middleware"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// NewRouter builds and returns the Gin engine with all routes registered.
func NewRouter(
	chatHandler *handlers.ChatHandler,
	wsHandler *handlers.WSHandler,
	jwtSecret string,
	logger *zap.Logger,
) *gin.Engine {
	r := gin.New()

	// Global middleware.
	r.Use(middleware.Recovery(logger))
	r.Use(middleware.Logger(logger))

	// Health check (unauthenticated).
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "alba-crm"})
	})

	// WebSocket (auth handled at upgrade level via session_id).
	r.GET("/ws/chat", wsHandler.Connect)

	// Authenticated API routes.
	api := r.Group("/api/v1")
	api.Use(middleware.Auth(jwtSecret, logger))
	{
		chat := api.Group("/chat")
		{
			chat.POST("/message", chatHandler.SendMessage)
			chat.GET("/history", chatHandler.GetHistory)
		}
	}

	return r
}
