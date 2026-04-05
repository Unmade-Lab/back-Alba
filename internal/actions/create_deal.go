package actions

import (
	"context"
	"fmt"

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

func (a *CreateDealAction) Execute(ctx context.Context, params map[string]interface{}) (Result, error) {
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

	// --- Persist ---
	deal := &models.Deal{
		ID:          uuid.New(),
		Name:        name,
		Amount:      amount,
		Stage:       stage,
		Description: description,
	}

	_, err := a.db.Exec(ctx,
		`INSERT INTO deals (id, name, amount, stage, description)
		 VALUES ($1, $2, $3, $4, $5)`,
		deal.ID, deal.Name, deal.Amount, deal.Stage, deal.Description,
	)
	if err != nil {
		return Result{}, fmt.Errorf("insert deal: %w", err)
	}

	// --- Event ---
	a.events.Publish(ctx, events.DomainEvent{
		Type: models.EventDealCreated,
		Payload: map[string]interface{}{
			"entity_type": "deal",
			"entity_id":   deal.ID.String(),
			"name":        deal.Name,
			"amount":      deal.Amount,
		},
	})

	return Result{
		Data:    deal,
		Message: fmt.Sprintf("Deal '%s' created successfully with amount $%.2f.", deal.Name, deal.Amount),
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
