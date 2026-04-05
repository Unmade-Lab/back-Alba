package actions

import (
	"context"
	"fmt"

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

func (a *UpdateDealStageAction) Execute(ctx context.Context, params map[string]interface{}) (Result, error) {
	stage, ok := params["stage"].(string)
	if !ok || stage == "" {
		return Result{}, fmt.Errorf("'stage' is required")
	}

	var deal models.Deal
	dealIDStr, _ := params["deal_id"].(string)
	dealName, _ := params["deal_name"].(string)

	if dealIDStr != "" {
		id, err := uuid.Parse(dealIDStr)
		if err != nil {
			return Result{}, fmt.Errorf("invalid deal_id: %w", err)
		}
		err = a.db.QueryRow(ctx,
			`UPDATE deals SET stage=$1, updated_at=NOW()
			 WHERE id=$2
			 RETURNING id, name, amount, stage, description, created_at, updated_at`,
			stage, id,
		).Scan(&deal.ID, &deal.Name, &deal.Amount, &deal.Stage, &deal.Description, &deal.CreatedAt, &deal.UpdatedAt)
		if err != nil {
			return Result{}, fmt.Errorf("update deal by id: %w", err)
		}
	} else if dealName != "" {
		err := a.db.QueryRow(ctx,
			`UPDATE deals SET stage=$1, updated_at=NOW()
			 WHERE name ILIKE $2
			 RETURNING id, name, amount, stage, description, created_at, updated_at`,
			stage, "%"+dealName+"%",
		).Scan(&deal.ID, &deal.Name, &deal.Amount, &deal.Stage, &deal.Description, &deal.CreatedAt, &deal.UpdatedAt)
		if err != nil {
			return Result{}, fmt.Errorf("update deal by name: %w", err)
		}
	} else {
		return Result{}, fmt.Errorf("provide either 'deal_id' or 'deal_name'")
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
