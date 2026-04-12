package models

import (
	"time"

	"github.com/google/uuid"
)

// Deal stage constants.
const (
	StageNew         = "new"
	StageQualified   = "qualified"
	StageProposal    = "proposal"
	StageNegotiation = "negotiation"
	StageClosedWon   = "closed_won"
	StageClosedLost  = "closed_lost"
)

// Deal represents a sales opportunity in the CRM.
type Deal struct {
	ID          uuid.UUID  `json:"id"`
	WorkspaceID uuid.UUID  `json:"workspace_id"`
	Name        string     `json:"name"`
	Amount      float64    `json:"amount"`
	Stage       string     `json:"stage"` // legacy, kept for backward compatibility
	PipelineID  *uuid.UUID `json:"pipeline_id,omitempty"`
	StageID     *uuid.UUID `json:"stage_id,omitempty"`
	CompanyID   *uuid.UUID `json:"company_id,omitempty"`
	OwnerID     *uuid.UUID `json:"owner_id,omitempty"`
	CloseDate   *time.Time `json:"close_date,omitempty"`
	Description string     `json:"description"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// ValidStages returns all valid deal stages.
func ValidStages() []string {
	return []string{
		StageNew, StageQualified, StageProposal,
		StageNegotiation, StageClosedWon, StageClosedLost,
	}
}
