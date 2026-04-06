package api

import (
	"net/http"

	"github.com/Unmade-Lab/back-Alba/internal/api/handlers"
	"github.com/Unmade-Lab/back-Alba/internal/api/middleware"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// NewRouter builds and returns the Gin engine with all routes registered.
func NewRouter(
	chatHandler *handlers.ChatHandler,
	wsHandler *handlers.WSHandler,
	cmdHandler *handlers.CommandHandler,
	authHandler *handlers.AuthHandler,
	jwtSecret string,
	logger *zap.Logger,
) *gin.Engine {
	r := gin.New()

	// ── CORS Middleware ───────────────────────────────────────────────────────
	corsConfig := cors.DefaultConfig()
	corsConfig.AllowAllOrigins = true
	corsConfig.AllowHeaders = []string{"Origin", "Content-Length", "Content-Type", "Authorization"}
	corsConfig.AllowMethods = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}
	r.Use(cors.New(corsConfig))

	// Global middleware.
	r.Use(middleware.Recovery(logger))
	r.Use(middleware.Logger(logger))

	// Health check (unauthenticated).
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "alba-crm"})
	})

	// WebSocket (auth handled at upgrade level via session_id).
	r.GET("/ws/chat", wsHandler.Connect)

	// Auth routes (unauthenticated)
	auth := r.Group("/api/v1/auth")
	{
		auth.POST("/login", authHandler.Login)
		auth.POST("/activate", authHandler.ActivateInvite)
	}

	// Authenticated API routes.
	api := r.Group("/api/v1")
	api.Use(middleware.Auth(jwtSecret, logger))
	{
		chat := api.Group("/chat")
		{
			chat.POST("/message", chatHandler.SendMessage)
			chat.GET("/history", chatHandler.GetHistory)
			chat.GET("/sessions", chatHandler.GetSessions)
		}
		cmds := api.Group("/commands")
		{
			cmds.POST("/commit", cmdHandler.CommitDraft)
		}
	}

	return r
}
