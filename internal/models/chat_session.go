package models

import (
	"time"

	"github.com/google/uuid"
)

// ChatSession represents an ongoing conversation thread with a specific topic.
type ChatSession struct {
	ID        string    `json:"id"`
	UserID    uuid.UUID `json:"user_id"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
