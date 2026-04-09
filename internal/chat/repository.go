package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Unmade-Lab/back-Alba/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository handles database persistence for chat messages.
type Repository struct {
	db *pgxpool.Pool
}

// NewRepository creates a chat Repository.
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// Save persists a ChatMessage to the database.
func (r *Repository) Save(ctx context.Context, msg *models.ChatMessage) error {
	if msg.ID == uuid.Nil {
		msg.ID = uuid.New()
	}
	if msg.CreatedAt.IsZero() {
		msg.CreatedAt = time.Now()
	}

	wid := ctx.Value(models.CtxWorkspaceID)
	if wid == nil || wid == "" {
		return fmt.Errorf("workspace context missing")
	}

	_, err := r.db.Exec(ctx,
		`INSERT INTO chat_messages (id, session_id, user_id, role, content, widget, intent, workspace_id)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		msg.ID, msg.SessionID, msg.UserID, msg.Role, msg.Content, msg.Widget, msg.Intent, wid,
	)
	if err != nil {
		return fmt.Errorf("save chat message: %w", err)
	}
	return nil
}

// GetHistory returns the most recent `limit` messages for a session, oldest first.
func (r *Repository) GetHistory(ctx context.Context, sessionID string, limit int) ([]*models.ChatMessage, error) {
	if limit <= 0 {
		limit = 50     
	}

	wid := ctx.Value(models.CtxWorkspaceID)
	if wid == nil || wid == "" {
		return nil, fmt.Errorf("workspace context missing")
	}

	rows, err := r.db.Query(ctx,
		`SELECT id, session_id, user_id, role, content, widget, intent, created_at
		 FROM chat_messages
		 WHERE session_id = $1 AND workspace_id = $2
		 ORDER BY created_at DESC
		 LIMIT $3`,
		sessionID, wid, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("query history: %w", err)
	}
	defer rows.Close()

	var messages []*models.ChatMessage
	for rows.Next() {
		var m models.ChatMessage
		var widgetRaw []byte
		if err := rows.Scan(
			&m.ID, &m.SessionID, &m.UserID, &m.Role,
			&m.Content, &widgetRaw, &m.Intent, &m.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan message: %w", err)
		}
		if widgetRaw != nil {
			m.Widget = json.RawMessage(widgetRaw)
		}
		messages = append(messages, &m)
	}

	// Reverse to chronological order.
	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	return messages, nil
}

// GetSessions returns all chat sessions for a user, ordered by newest first.
func (r *Repository) GetSessions(ctx context.Context, userID uuid.UUID) ([]*models.ChatSession, error) {
	wid := ctx.Value(models.CtxWorkspaceID)
	if wid == nil || wid == "" {
		return nil, fmt.Errorf("workspace context missing")
	}

	rows, err := r.db.Query(ctx,
		`SELECT id, user_id, title, created_at, updated_at
		 FROM chat_sessions
		 WHERE user_id = $1 AND workspace_id = $2
		 ORDER BY updated_at DESC`,
		userID, wid,
	)
	if err != nil {
		return nil, fmt.Errorf("query sessions: %w", err)
	}
	defer rows.Close()

	var sessions []*models.ChatSession
	for rows.Next() {
		var s models.ChatSession
		if err := rows.Scan(&s.ID, &s.UserID, &s.Title, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan session: %w", err)
		}
		sessions = append(sessions, &s)
	}
	if sessions == nil {
		sessions = []*models.ChatSession{}
	}

	return sessions, nil
}

// CreateSession creates a new chat session. Does nothing if ID exists.
func (r *Repository) CreateSession(ctx context.Context, sessionID string, userID uuid.UUID, title string) error {
	wid := ctx.Value(models.CtxWorkspaceID)
	if wid == nil || wid == "" {
		return fmt.Errorf("workspace context missing")
	}

	_, err := r.db.Exec(ctx,
		`INSERT INTO chat_sessions (id, user_id, title, workspace_id)
		 VALUES ($1, $2, $3, $4)
		 ON CONFLICT (id) DO NOTHING`,
		sessionID, userID, title, wid,
	)
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	return nil
}
