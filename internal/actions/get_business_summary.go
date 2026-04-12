package actions

import (
	"context"
	"fmt"
	"time"

	"github.com/Unmade-Lab/back-Alba/internal/convctx"
	"github.com/Unmade-Lab/back-Alba/internal/models"
	"github.com/jackc/pgx/v5/pgxpool"
)

// BusinessSummary holds the aggregated metric digest for a workspace.
type BusinessSummary struct {
	Period          string  `json:"period"`
	NewDeals        int     `json:"new_deals"`
	NewDealsValue   float64 `json:"new_deals_value"`
	DealsWon        int     `json:"deals_won"`
	DealsWonValue   float64 `json:"deals_won_value"`
	NewCompanies    int     `json:"new_companies"`
	TasksCreated    int     `json:"tasks_created"`
	TasksCompleted  int     `json:"tasks_completed"`
	TasksOverdue    int     `json:"tasks_overdue"`
	TotalDeals      int     `json:"total_deals_active"`
	TotalCompanies  int     `json:"total_companies"`
}

// GetBusinessSummaryAction generates a period-based business digest.
type GetBusinessSummaryAction struct {
	db *pgxpool.Pool
}

func NewGetBusinessSummaryAction(db *pgxpool.Pool) *GetBusinessSummaryAction {
	return &GetBusinessSummaryAction{db: db}
}

func (a *GetBusinessSummaryAction) Name() string        { return "get_business_summary" }
func (a *GetBusinessSummaryAction) Description() string {
	return "Generates a business digest/summary showing key metrics: new deals, won deals, new companies, task completion. Use when the user asks about 'how are we doing', 'weekly summary', 'business overview', or performance stats."
}

func (a *GetBusinessSummaryAction) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"period": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"today", "week", "month", "all"},
				"description": "Time period for the summary. Default is 'week'.",
			},
		},
	}
}

func (a *GetBusinessSummaryAction) Execute(ctx context.Context, convCtx *convctx.ConversationContext, params map[string]interface{}) (Result, error) {
	wid := ctx.Value(models.CtxWorkspaceID)
	if wid == nil || wid == "" {
		return Result{}, fmt.Errorf("workspace context missing")
	}

	period := "week"
	if p, ok := params["period"].(string); ok && p != "" {
		period = p
	}

	// Calculate time range
	now := time.Now()
	var since time.Time
	var periodLabel string
	switch period {
	case "today":
		since = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		periodLabel = "сегодня"
	case "month":
		since = now.AddDate(0, -1, 0)
		periodLabel = "за последний месяц"
	case "all":
		since = time.Time{} // zero value = no filter
		periodLabel = "за всё время"
	default: // week
		since = now.AddDate(0, 0, -7)
		periodLabel = "за последнюю неделю"
	}

	summary := BusinessSummary{Period: periodLabel}

	buildFilter := func(col string) (string, []interface{}) {
		if since.IsZero() {
			return fmt.Sprintf("WHERE workspace_id = $1"), []interface{}{wid}
		}
		return fmt.Sprintf("WHERE workspace_id = $1 AND %s >= $2", col), []interface{}{wid, since}
	}

	// 1. New Deals
	q, args := buildFilter("created_at")
	_ = a.db.QueryRow(ctx,
		fmt.Sprintf(`SELECT COUNT(*), COALESCE(SUM(amount), 0) FROM deals %s`, q), args...,
	).Scan(&summary.NewDeals, &summary.NewDealsValue)

	// 2. Won Deals (stage_name contains 'Won' or 'Выиграно')
	_ = a.db.QueryRow(ctx, `
		SELECT COUNT(d.id), COALESCE(SUM(d.amount), 0)
		FROM deals d
		JOIN stages s ON s.id = d.stage_id
		WHERE d.workspace_id = $1 AND (s.name ILIKE '%won%' OR s.name ILIKE '%выигр%' OR s.name ILIKE '%closed%')`,
		wid,
	).Scan(&summary.DealsWon, &summary.DealsWonValue)

	// 3. New Companies
	q, args = buildFilter("created_at")
	_ = a.db.QueryRow(ctx,
		fmt.Sprintf(`SELECT COUNT(*) FROM companies %s`, q), args...,
	).Scan(&summary.NewCompanies)

	// 4. Tasks stats
	q, args = buildFilter("created_at")
	_ = a.db.QueryRow(ctx,
		fmt.Sprintf(`SELECT COUNT(*) FROM tasks %s`, q), args...,
	).Scan(&summary.TasksCreated)

	_ = a.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM tasks WHERE workspace_id = $1 AND status = 'completed'`, wid,
	).Scan(&summary.TasksCompleted)

	_ = a.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM tasks WHERE workspace_id = $1 AND status = 'pending' AND due_at < NOW()`, wid,
	).Scan(&summary.TasksOverdue)

	// 5. Totals
	_ = a.db.QueryRow(ctx, `SELECT COUNT(*) FROM deals WHERE workspace_id = $1`, wid).Scan(&summary.TotalDeals)
	_ = a.db.QueryRow(ctx, `SELECT COUNT(*) FROM companies WHERE workspace_id = $1`, wid).Scan(&summary.TotalCompanies)

	msg := fmt.Sprintf(
		"Итоги %s: новых сделок — %d (на сумму %.0f), закрыто — %d, новых компаний — %d, задач создано — %d, просрочено — %d.",
		summary.Period, summary.NewDeals, summary.NewDealsValue,
		summary.DealsWon, summary.NewCompanies, summary.TasksCreated, summary.TasksOverdue,
	)

	return Result{
		Data:    summary,
		Message: msg,
	}, nil
}
