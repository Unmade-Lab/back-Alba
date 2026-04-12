package actions

import (
	"context"
	"fmt"

	"github.com/Unmade-Lab/back-Alba/internal/convctx"
	"github.com/Unmade-Lab/back-Alba/internal/events"
	"github.com/Unmade-Lab/back-Alba/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CreateDealAction handles the "create_deal" intent.
type CreateDealAction struct {
	db     *pgxpool.Pool
	events *events.Dispatcher
}

// NewCreateDealAction constructs the action with its dependencies.
func NewCreateDealAction(db *pgxpool.Pool, dispatcher *events.Dispatcher) *CreateDealAction {
	return &CreateDealAction{db: db, events: dispatcher}
}

func (a *CreateDealAction) Name() string        { return "create_deal" }
func (a *CreateDealAction) Description() string {
	return "Create a new sales deal in the CRM. Use when the user wants to add a deal, opportunity, or sale."
}

func (a *CreateDealAction) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{
				"type":        "string",
				"description": "Name of the deal (e.g., 'Tesla Deal').",
			},
			"amount": map[string]interface{}{
				"type":        "number",
				"description": "Deal value in USD.",
			},
			"stage": map[string]interface{}{
				"type":        "string",
				"description": "Deal stage. One of: new, qualified, proposal, negotiation, closed_won, closed_lost.",
				"enum":        models.ValidStages(),
			},
			"description": map[string]interface{}{
				"type":        "string",
				"description": "Optional description or notes about the deal.",
			},
		},
		"required": []string{"name"},
	}
}

func (a *CreateDealAction) Execute(ctx context.Context, convCtx *convctx.ConversationContext, params map[string]interface{}) (Result, error) {
	// --- Validation ---
	name, ok := params["name"].(string)
	if !ok || name == "" {
		return Result{}, fmt.Errorf("'name' is required and must be a string")
	}

	amount, _ := toFloat64(params["amount"])
	stage, _ := params["stage"].(string)
	if stage == "" {
		stage = models.StageNew
	}
	description, _ := params["description"].(string)

	// --- Resolve Stage & Pipeline IDs ---
	wid := ctx.Value(models.CtxWorkspaceID)
	
	var stageID, pipelineID uuid.UUID
	
	// First, find the default pipeline for this workspace
	err := a.db.QueryRow(ctx, 
		"SELECT id FROM pipelines WHERE workspace_id = $1 AND is_default = TRUE LIMIT 1", 
		wid).Scan(&pipelineID)
	if err != nil {
		// Fallback: just find any pipeline for this workspace
		_ = a.db.QueryRow(ctx, "SELECT id FROM pipelines WHERE workspace_id = $1 LIMIT 1", wid).Scan(&pipelineID)
	}

	if pipelineID != uuid.Nil {
		// find the stage ID by name within that pipeline
		sq := "SELECT id FROM stages WHERE pipeline_id = $1 AND LOWER(name) = LOWER($2) LIMIT 1"
		if stage == "" || stage == models.StageNew {
			// Get first stage by sort order
			sq = "SELECT id FROM stages WHERE pipeline_id = $1 ORDER BY sort_order ASC LIMIT 1"
			_ = a.db.QueryRow(ctx, sq, pipelineID).Scan(&stageID)
		} else {
			err = a.db.QueryRow(ctx, sq, pipelineID, stage).Scan(&stageID)
			if err != nil {
				// Fallback to first stage if name not found
				_ = a.db.QueryRow(ctx, "SELECT id FROM stages WHERE pipeline_id = $1 ORDER BY sort_order ASC LIMIT 1", pipelineID).Scan(&stageID)
			}
		}
	}

	// --- Persist ---
	deal := &models.Deal{
		ID:          uuid.New(),
		WorkspaceID: uuid.MustParse(wid.(string)),
		Name:        name,
		Amount:      amount,
		Stage:       stage,
		PipelineID:  &pipelineID,
		StageID:     &stageID,
		Description: description,
	}

	convCtx.Draft = &convctx.DraftData{
		ActionName: a.Name(),
		Payload: map[string]interface{}{
			"deal": deal,
		},
	}

	return Result{
		Data:    deal,
		Message: fmt.Sprintf("I've drafted a new deal '%s' for $%.2f. Please review and confirm to save it.", deal.Name, deal.Amount),
	}, nil
}

// toFloat64 safely converts interface{} to float64.
func toFloat64(v interface{}) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case float32:
		return float64(val), true
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	}
	return 0, false
}
