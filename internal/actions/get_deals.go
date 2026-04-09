package actions

import (
	"context"
	"fmt"

	"github.com/Unmade-Lab/back-Alba/internal/convctx"
	"github.com/Unmade-Lab/back-Alba/internal/models"
	"github.com/jackc/pgx/v5/pgxpool"
)

// GetDealsAction handles the "get_deals" intent.
type GetDealsAction struct {
	db *pgxpool.Pool
}

func NewGetDealsAction(db *pgxpool.Pool) *GetDealsAction {
	return &GetDealsAction{db: db}
}

func (a *GetDealsAction) Name() string        { return "get_deals" }
func (a *GetDealsAction) Description() string {
	return "Retrieve a list of deals from the CRM. Supports optional filtering by stage."
}

func (a *GetDealsAction) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"stage": map[string]interface{}{
				"type":        "string",
				"description": "Filter by deal stage.",
				"enum":        models.ValidStages(),
			},
			"limit": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum number of deals to return (default 20).",
			},
		},
	}
}

func (a *GetDealsAction) Execute(ctx context.Context, convCtx *convctx.ConversationContext, params map[string]interface{}) (Result, error) {
	stage, _ := params["stage"].(string)
	limit := 20
	if l, ok := params["limit"]; ok {
		if lf, ok := toFloat64(l); ok {
			limit = int(lf)
		}
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	wid := ctx.Value(models.CtxWorkspaceID)
	if wid == nil || wid == "" {
		return Result{}, fmt.Errorf("workspace context missing")
	}

	query := `SELECT id, name, amount, stage, description, created_at, updated_at
	          FROM deals WHERE workspace_id = $1 ORDER BY created_at DESC LIMIT $2`
	args := []interface{}{wid, limit}

	if stage != "" {
		query = `SELECT id, name, amount, stage, description, created_at, updated_at
		         FROM deals WHERE workspace_id = $1 AND stage = $2 ORDER BY created_at DESC LIMIT $3`
		args = []interface{}{wid, stage, limit}
	}

	rows, err := a.db.Query(ctx, query, args...)
	if err != nil {
		return Result{}, fmt.Errorf("query deals: %w", err)
	}
	defer rows.Close()

	var deals []models.Deal
	for rows.Next() {
		var d models.Deal
		if err := rows.Scan(&d.ID, &d.Name, &d.Amount, &d.Stage, &d.Description, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return Result{}, fmt.Errorf("scan deal: %w", err)
		}
		deals = append(deals, d)
	}
	if deals == nil {
		deals = []models.Deal{}
	}

	return Result{
		Data:    deals,
		Message: fmt.Sprintf("Found %d deal(s).", len(deals)),
	}, nil
}
