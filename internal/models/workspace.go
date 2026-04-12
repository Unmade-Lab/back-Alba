package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Workspace represents a tenant in the system.
type Workspace struct {
	ID                  uuid.UUID       `json:"id"`
	Name                string          `json:"name"`
	OnboardingCompleted bool            `json:"onboarding_completed"`
	Industry            string          `json:"industry,omitempty"`
	OnboardingData      json.RawMessage `json:"onboarding_data,omitempty"`
	CreatedAt           time.Time       `json:"created_at"`
}

// Pipeline represents a sales funnel.
type Pipeline struct {
	ID          uuid.UUID `json:"id"`
	WorkspaceID uuid.UUID `json:"workspace_id"`
	Name        string    `json:"name"`
	IsDefault   bool      `json:"is_default"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Stage represents a step within a pipeline.
type Stage struct {
	ID         uuid.UUID `json:"id"`
	PipelineID uuid.UUID `json:"pipeline_id"`
	Name       string    `json:"name"`
	SortOrder  int       `json:"sort_order"`
	Probability int       `json:"probability"`
	CreatedAt  time.Time `json:"created_at"`
}

// Department represents a business unit.
type Department struct {
	ID          uuid.UUID  `json:"id"`
	WorkspaceID uuid.UUID  `json:"workspace_id"`
	ParentID    *uuid.UUID `json:"parent_id,omitempty"`
	Name        string     `json:"name"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}
