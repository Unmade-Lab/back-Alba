package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// System event type constants.
const (
	EventDealCreated      = "deal_created"
	EventDealStageChanged = "deal_stage_changed"
	EventCompanyCreated   = "company_created"
	EventTaskCreated      = "task_created"
)

// Event is an immutable audit-log entry for system-level occurrences.
type Event struct {
	ID         uuid.UUID       `json:"id"`
	EventType  string          `json:"event_type"`
	EntityType string          `json:"entity_type"`
	EntityID   *uuid.UUID      `json:"entity_id,omitempty"`
	Payload    json.RawMessage `json:"payload,omitempty"`
	UserID     *uuid.UUID      `json:"user_id,omitempty"`
	CreatedAt  time.Time       `json:"created_at"`
}
