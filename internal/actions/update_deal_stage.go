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

// UpdateDealStageAction handles the "update_deal_stage" intent.
type UpdateDealStageAction struct {
	db     *pgxpool.Pool
	events *events.Dispatcher
}

func NewUpdateDealStageAction(db *pgxpool.Pool, dispatcher *events.Dispatcher) *UpdateDealStageAction {
	return &UpdateDealStageAction{db: db, events: dispatcher}
}

func (a *UpdateDealStageAction) Name() string        { return "update_deal_stage" }
func (a *UpdateDealStageAction) Description() string {
	return "Update the pipeline stage of an existing deal. Use when the user says a deal moved, progressed, or changed status."
}

func (a *UpdateDealStageAction) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"deal_id": map[string]interface{}{
				"type":        "string",
				"description": "UUID of the deal to update.",
			},
			"deal_name": map[string]interface{}{
				"type":        "string",
				"description": "Name of the deal (used to find it when deal_id is not provided).",
			},
			"stage": map[string]interface{}{
				"type":        "string",
				"description": "New stage for the deal.",
				"enum":        models.ValidStages(),
			},
		},
		"required": []string{"stage"},
	}
}

func (a *UpdateDealStageAction) Execute(ctx context.Context, convCtx *convctx.ConversationContext, params map[string]interface{}) (Result, error) {
	stage, ok := params["stage"].(string)
	if !ok || stage == "" {
		return Result{}, fmt.Errorf("'stage' is required")
	}

	wid := ctx.Value(models.CtxWorkspaceID)
	if wid == nil || wid == "" {
		return Result{}, fmt.Errorf("workspace context missing")
	}

	var deal models.Deal
	dealIDStr, _ := params["deal_id"].(string)
	dealName, _ := params["deal_name"].(string)

	var targetID uuid.UUID
	
	// 1. Find the deal first to get its pipeline_id
	var pID uuid.UUID
	if dealIDStr != "" {
		targetID, _ = uuid.Parse(dealIDStr)
		_ = a.db.QueryRow(ctx, "SELECT pipeline_id FROM deals WHERE id = $1 AND workspace_id = $2", targetID, wid).Scan(&pID)
	} else if dealName != "" {
		_ = a.db.QueryRow(ctx, "SELECT id, pipeline_id FROM deals WHERE name ILIKE $1 AND workspace_id = $2 LIMIT 1", "%"+dealName+"%", wid).Scan(&targetID, &pID)
	}

	if targetID == uuid.Nil {
		return Result{}, fmt.Errorf("could not find deal")
	}

	// 2. Resolve StageID by name within that pipeline
	var stageID uuid.UUID
	err := a.db.QueryRow(ctx, "SELECT id FROM stages WHERE pipeline_id = $1 AND LOWER(name) = LOWER($2) LIMIT 1", pID, stage).Scan(&stageID)
	if err != nil {
		// If stage NAME not found, maybe try finding it by ID if the AI passed an ID? 
		// Or just fallback to whatever was passed if it's a UUID
		sID, errP := uuid.Parse(stage)
		if errP == nil {
			stageID = sID
		} else {
			return Result{}, fmt.Errorf("stage '%s' not found for this deal's pipeline", stage)
		}
	}

	// 3. Update the deal
	err = a.db.QueryRow(ctx,
		`UPDATE deals SET stage=$1, stage_id=$2, updated_at=NOW()
		 WHERE id=$3 AND workspace_id=$4
		 RETURNING id, name, amount, stage, description, created_at, updated_at`,
		stage, stageID, targetID, wid,
	).Scan(&deal.ID, &deal.Name, &deal.Amount, &deal.Stage, &deal.Description, &deal.CreatedAt, &deal.UpdatedAt)
	
	if err != nil {
		return Result{}, fmt.Errorf("update deal: %w", err)
	}

	a.events.Publish(ctx, events.DomainEvent{
		Type: models.EventDealStageChanged,
		Payload: map[string]interface{}{
			"entity_type": "deal",
			"entity_id":   deal.ID.String(),
			"name":        deal.Name,
			"new_stage":   stage,
		},
	})

	return Result{
		Data:    deal,
		Message: fmt.Sprintf("Deal '%s' moved to stage '%s'.", deal.Name, stage),
	}, nil
}
