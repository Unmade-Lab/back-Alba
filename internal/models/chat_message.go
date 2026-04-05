package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Message role constants.
const (
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleSystem    = "system"
)

// ChatMessage represents a single message in a chat session.
type ChatMessage struct {
	ID        uuid.UUID       `json:"id"`
	SessionID string          `json:"session_id"`
	UserID    *uuid.UUID      `json:"user_id,omitempty"`
	Role      string          `json:"role"`
	Content   string          `json:"content"`
	Widget    json.RawMessage `json:"widget,omitempty"`
	Intent    string          `json:"intent,omitempty"`
	CreatedAt time.Time       `json:"created_at"`
}
