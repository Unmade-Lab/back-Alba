package actions

import (
	"context"
	"fmt"

	"github.com/Unmade-Lab/back-Alba/internal/convctx"
	"github.com/Unmade-Lab/back-Alba/internal/models"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PipelineStageReport holds analytics for a single stage.
type PipelineStageReport struct {
	StageName  string  `json:"stage_name"`
	DealCount  int     `json:"deal_count"`
	TotalValue float64 `json:"total_value"`
	AvgValue   float64 `json:"avg_value"`
}

// PipelineReport holds the full funnel analytics for a workspace.
type PipelineReport struct {
	PipelineName    string                `json:"pipeline_name"`
	TotalDeals      int                   `json:"total_deals"`
	TotalValue      float64               `json:"total_value"`
	Stages          []PipelineStageReport `json:"stages"`
	ConversionRate  float64               `json:"conversion_rate_pct"` // (won / total) * 100
}

// GetPipelineReportAction generates a full funnel analytics report.
type GetPipelineReportAction struct {
	db *pgxpool.Pool
}

func NewGetPipelineReportAction(db *pgxpool.Pool) *GetPipelineReportAction {
	return &GetPipelineReportAction{db: db}
}

func (a *GetPipelineReportAction) Name() string        { return "get_pipeline_report" }
func (a *GetPipelineReportAction) Description() string {
	return "Generates a sales pipeline analytics report showing deal counts, values, and conversion rates per stage. Use when the user asks about pipeline health, conversion, funnel analysis, or sales performance."
}

func (a *GetPipelineReportAction) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"pipeline_name": map[string]interface{}{
				"type":        "string",
				"description": "Name of the pipeline to analyze. If omitted, uses the default pipeline.",
			},
		},
	}
}

func (a *GetPipelineReportAction) Execute(ctx context.Context, convCtx *convctx.ConversationContext, params map[string]interface{}) (Result, error) {
	wid := ctx.Value(models.CtxWorkspaceID)
	if wid == nil || wid == "" {
		return Result{}, fmt.Errorf("workspace context missing")
	}

	pipelineName, _ := params["pipeline_name"].(string)

	// 1. Find pipeline
	var pipelineID string
	var foundPipelineName string
	var queryErr error
	if pipelineName != "" {
		queryErr = a.db.QueryRow(ctx,
			`SELECT id, name FROM pipelines WHERE workspace_id = $1 AND name ILIKE $2 LIMIT 1`,
			wid, "%"+pipelineName+"%",
		).Scan(&pipelineID, &foundPipelineName)
	} else {
		queryErr = a.db.QueryRow(ctx,
			`SELECT id, name FROM pipelines WHERE workspace_id = $1 AND is_default = TRUE LIMIT 1`,
			wid,
		).Scan(&pipelineID, &foundPipelineName)
	}
	if queryErr != nil {
		// fallback: grab first pipeline
		queryErr = a.db.QueryRow(ctx,
			`SELECT id, name FROM pipelines WHERE workspace_id = $1 ORDER BY created_at LIMIT 1`,
			wid,
		).Scan(&pipelineID, &foundPipelineName)
		if queryErr != nil {
			return Result{Message: "Нет воронок продаж. Сначала завершите онбординг."}, nil
		}
	}

	// 2. Get stats per stage (join deals with stages)
	rows, err := a.db.Query(ctx, `
		SELECT 
			s.name AS stage_name,
			COUNT(d.id) AS deal_count,
			COALESCE(SUM(d.amount), 0) AS total_value,
			COALESCE(AVG(d.amount), 0) AS avg_value
		FROM stages s
		LEFT JOIN deals d ON d.stage_id = s.id AND d.workspace_id = $1
		WHERE s.pipeline_id = $2
		GROUP BY s.id, s.name, s.sort_order
		ORDER BY s.sort_order ASC
	`, wid, pipelineID)
	if err != nil {
		return Result{}, fmt.Errorf("query pipeline stats: %w", err)
	}
	defer rows.Close()

	var stageReports []PipelineStageReport
	totalDeals := 0
	totalValue := 0.0

	for rows.Next() {
		var sr PipelineStageReport
		if err := rows.Scan(&sr.StageName, &sr.DealCount, &sr.TotalValue, &sr.AvgValue); err != nil {
			return Result{}, fmt.Errorf("scan stage report: %w", err)
		}
		stageReports = append(stageReports, sr)
		totalDeals += sr.DealCount
		totalValue += sr.TotalValue
	}

	// 3. Calculate conversion (Won / Total)
	conversionRate := 0.0
	for _, sr := range stageReports {
		if sr.StageName == "Won" || sr.StageName == "Выиграно" || sr.StageName == "Closed" {
			if totalDeals > 0 {
				conversionRate = float64(sr.DealCount) / float64(totalDeals) * 100
			}
		}
	}

	report := PipelineReport{
		PipelineName:   foundPipelineName,
		TotalDeals:     totalDeals,
		TotalValue:     totalValue,
		Stages:         stageReports,
		ConversionRate: conversionRate,
	}

	msg := fmt.Sprintf("Отчет по воронке «%s»: %d сделок на сумму %.0f. Конверсия %.1f%%.",
		report.PipelineName, report.TotalDeals, report.TotalValue, report.ConversionRate)

	return Result{
		Data:    report,
		Message: msg,
	}, nil
}
