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

// RunMigrations ensures all migrations are applied exactly once.
func RunMigrations(ctx context.Context, pool *pgxpool.Pool, logger *zap.Logger) error {
	// 1. Create migrations table if not exists
	_, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			name VARCHAR(255) PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
	`)
	if err != nil {
		return fmt.Errorf("create migrations table: %w", err)
	}

	files := []string{
		"001_init.sql",
		"002_chat_sessions.sql",
		"003_invitations.sql",
		"004_workspaces.sql",
		"005_refresh_tokens.sql",
		"006_onboarding.sql",
		"007_fix_workspace_updated_at.sql",
		"008_crm_normalization.sql",
		"009_tasks.sql",
		"010_company_enrichment.sql",
	}

	for _, filename := range files {
		// Check if already applied
		var exists bool
		err := pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE name = $1)", filename).Scan(&exists)
		if err != nil {
			return fmt.Errorf("check migration %s: %w", filename, err)
		}

		if exists {
			logger.Debug("Skipping already applied migration", zap.String("file", filename))
			continue
		}

		// Apply migration
		logger.Info("Applying migration", zap.String("file", filename))
		content, err := os.ReadFile(fmt.Sprintf("migrations/%s", filename))
		if err != nil {
			return fmt.Errorf("read migration %s: %w", filename, err)
		}

		// Use a transaction for each migration file
		tx, err := pool.Begin(ctx)
		if err != nil {
			return fmt.Errorf("begin tx for %s: %w", filename, err)
		}
		defer tx.Rollback(ctx) //nolint:errcheck

		if _, err := tx.Exec(ctx, string(content)); err != nil {
			return fmt.Errorf("execute migration %s: %w", filename, err)
		}

		if _, err := tx.Exec(ctx, "INSERT INTO schema_migrations (name) VALUES ($1)", filename); err != nil {
			return fmt.Errorf("record migration %s: %w", filename, err)
		}

		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("commit migration %s: %w", filename, err)
		}
	}

	logger.Info("Database migrations applied successfully")
	return nil
}
