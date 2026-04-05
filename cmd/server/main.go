package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Unmade-Lab/back-Alba/config"
	"github.com/Unmade-Lab/back-Alba/internal/actions"
	"github.com/Unmade-Lab/back-Alba/internal/api"
	"github.com/Unmade-Lab/back-Alba/internal/api/handlers"
	"github.com/Unmade-Lab/back-Alba/internal/chat"
	"github.com/Unmade-Lab/back-Alba/internal/convctx"
	"github.com/Unmade-Lab/back-Alba/internal/db"
	"github.com/Unmade-Lab/back-Alba/internal/events"
	"github.com/Unmade-Lab/back-Alba/internal/orchestrator"
	"github.com/Unmade-Lab/back-Alba/internal/widget"
	"github.com/Unmade-Lab/back-Alba/internal/ws"
	"github.com/joho/godotenv"
	"go.uber.org/zap"
)

func main() {
	// ── Load .env (ignore error if file doesn't exist in production) ──────────
	_ = godotenv.Load()

	// ── Configuration ─────────────────────────────────────────────────────────
	cfg := config.Load()

	// ── Logger ────────────────────────────────────────────────────────────────
	var logger *zap.Logger
	var err error
	if cfg.IsDevelopment() {
		logger, err = zap.NewDevelopment()
	} else {
		logger, err = zap.NewProduction()
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync() //nolint:errcheck

	logger.Info("Starting Alba CRM backend", zap.String("env", cfg.Environment))

	ctx := context.Background()

	// ── Database ──────────────────────────────────────────────────────────────
	pgPool, err := db.NewPostgresPool(ctx, cfg.DatabaseURL, logger)
	if err != nil {
		logger.Fatal("connect to postgres", zap.Error(err))
	}
	defer pgPool.Close()

	if err := db.RunMigrations(ctx, pgPool, logger); err != nil {
		logger.Fatal("run migrations", zap.Error(err))
	}

	// ── Redis ─────────────────────────────────────────────────────────────────
	redisClient, err := db.NewRedisClient(ctx, cfg.RedisURL, logger)
	if err != nil {
		logger.Fatal("connect to redis", zap.Error(err))
	}
	defer redisClient.Close()

	// ── Event Dispatcher ──────────────────────────────────────────────────────
	dispatcher := events.NewDispatcher(redisClient, logger)
	events.RegisterDefaultHandlers(dispatcher, pgPool, logger)

	// ── Conversation Context Manager ──────────────────────────────────────────
	ctxManager := convctx.NewManager(redisClient, logger)

	// ── Action Registry ───────────────────────────────────────────────────────
	registry := actions.NewRegistry()
	registry.Register(actions.NewCreateDealAction(pgPool, dispatcher))
	registry.Register(actions.NewCreateCompanyAction(pgPool, dispatcher))
	registry.Register(actions.NewGetDealsAction(pgPool))
	registry.Register(actions.NewUpdateDealStageAction(pgPool, dispatcher))

	logger.Info("Action registry initialised", zap.Int("actions", len(registry.All())))

	// ── AI Orchestrator ───────────────────────────────────────────────────────
	orch := orchestrator.New(cfg.GeminiKey, cfg.GeminiModel, registry, logger)

	// ── Widget Builder ────────────────────────────────────────────────────────
	widgetBuilder := widget.NewBuilder()

	// ── WebSocket Hub ─────────────────────────────────────────────────────────
	hub := ws.NewHub(logger)

	// ── Chat Layer ────────────────────────────────────────────────────────────
	chatRepo := chat.NewRepository(pgPool)
	chatService := chat.NewService(chatRepo, orch, registry, ctxManager, widgetBuilder, hub, logger)

	// ── HTTP Handlers & Router ────────────────────────────────────────────────
	chatHandler := handlers.NewChatHandler(chatService, hub, logger)
	wsHandler := handlers.NewWSHandler(hub, logger)

	router := api.NewRouter(chatHandler, wsHandler, cfg.JWTSecret, logger)

	// ── HTTP Server ───────────────────────────────────────────────────────────
	srv := &http.Server{
		Addr:         ":" + cfg.ServerPort,
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in background.
	go func() {
		logger.Info("HTTP server listening", zap.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("server error", zap.Error(err))
		}
	}()

	// ── Graceful Shutdown ─────────────────────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("server shutdown error", zap.Error(err))
	}
	logger.Info("Server stopped gracefully")
}
