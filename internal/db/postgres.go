package db

import (
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// NewPostgresPool creates a new PostgreSQL connection pool and verifies connectivity.
func NewPostgresPool(ctx context.Context, dsn string, logger *zap.Logger) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse postgres config: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create postgres pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	logger.Info("Connected to PostgreSQL")
	return pool, nil
}

// RunMigrations reads and executes migrations/001_init.sql.
func RunMigrations(ctx context.Context, pool *pgxpool.Pool, logger *zap.Logger) error {
	sql1, err := os.ReadFile("migrations/001_init.sql")
	if err != nil {
		return fmt.Errorf("read migration 001: %w", err)
	}
	if _, err := pool.Exec(ctx, string(sql1)); err != nil {
		return fmt.Errorf("execute migration 001: %w", err)
	}

	sql2, err := os.ReadFile("migrations/002_chat_sessions.sql")
	if err != nil {
		return fmt.Errorf("read migration 002: %w", err)
	}
	if _, err := pool.Exec(ctx, string(sql2)); err != nil {
		return fmt.Errorf("execute migration 002: %w", err)
	}

	logger.Info("Database migrations applied successfully")
	return nil
}
