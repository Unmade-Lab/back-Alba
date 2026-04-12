package actions

import (
	"context"
	"fmt"

	"github.com/Unmade-Lab/back-Alba/internal/convctx"
	"github.com/Unmade-Lab/back-Alba/internal/models"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// EnrichedCompanyData holds additional details inferred or retrieved for a company.
type EnrichedCompanyData struct {
	CompanyName  string   `json:"company_name"`
	Industry     string   `json:"industry,omitempty"`
	Website      string   `json:"website,omitempty"`
	Size         string   `json:"size,omitempty"`        // "1-10", "11-50", "51-200", etc.
	Description  string   `json:"description,omitempty"`
	Tags         []string `json:"tags,omitempty"`
	LinkedInURL  string   `json:"linkedin_url,omitempty"`
}

// EnrichCompanyAction enriches a company record with additional context.
// The AI itself acts as the enrichment engine using its training knowledge.
type EnrichCompanyAction struct {
	db     *pgxpool.Pool
	logger *zap.Logger
}

func NewEnrichCompanyAction(db *pgxpool.Pool, logger *zap.Logger) *EnrichCompanyAction {
	return &EnrichCompanyAction{db: db, logger: logger}
}

func (a *EnrichCompanyAction) Name() string        { return "enrich_company" }
func (a *EnrichCompanyAction) Description() string {
	return "Enriches a company profile with additional data such as industry, size, website, and description. Use when the user adds a company and wants more details, or explicitly asks to enrich/research a company."
}

func (a *EnrichCompanyAction) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"company_name": map[string]interface{}{
				"type":        "string",
				"description": "Name of the company to enrich.",
			},
			"company_id": map[string]interface{}{
				"type":        "string",
				"description": "UUID of the company in the CRM to update with enriched data.",
			},
			"industry": map[string]interface{}{
				"type":        "string",
				"description": "Industry or sector the company operates in.",
			},
			"website": map[string]interface{}{
				"type":        "string",
				"description": "Company website URL.",
			},
			"size": map[string]interface{}{
				"type":        "string",
				"description": "Approximate company size: '1-10', '11-50', '51-200', '201-1000', '1000+'.",
			},
			"description": map[string]interface{}{
				"type":        "string",
				"description": "Short description of what the company does.",
			},
		},
		"required": []string{"company_name"},
	}
}

func (a *EnrichCompanyAction) Execute(ctx context.Context, convCtx *convctx.ConversationContext, params map[string]interface{}) (Result, error) {
	wid := ctx.Value(models.CtxWorkspaceID)
	if wid == nil || wid == "" {
		return Result{}, fmt.Errorf("workspace context missing")
	}

	companyName, _ := params["company_name"].(string)
	if companyName == "" {
		return Result{}, fmt.Errorf("company_name is required")
	}

	industry, _ := params["industry"].(string)
	website, _ := params["website"].(string)
	size, _ := params["size"].(string)
	description, _ := params["description"].(string)
	companyID, _ := params["company_id"].(string)

	enriched := EnrichedCompanyData{
		CompanyName: companyName,
		Industry:    industry,
		Website:     website,
		Size:        size,
		Description: description,
	}

	// If company_id is provided, update the company record in DB
	if companyID != "" {
		_, err := a.db.Exec(ctx, `
			UPDATE companies
			SET 
				industry     = COALESCE(NULLIF($2, ''), industry),
				website      = COALESCE(NULLIF($3, ''), website),
				description  = COALESCE(NULLIF($4, ''), description),
				updated_at   = NOW()
			WHERE id = $1 AND workspace_id = $5
		`, companyID, industry, website, description, wid)
		if err != nil {
			a.logger.Warn("failed to update company enrichment", zap.Error(err))
		} else {
			a.logger.Info("company enriched", zap.String("company_id", companyID))
		}
	}

	msg := fmt.Sprintf("Компания «%s» обогащена данными.", companyName)
	if industry != "" {
		msg += fmt.Sprintf(" Отрасль: %s.", industry)
	}
	if size != "" {
		msg += fmt.Sprintf(" Размер: %s сотрудников.", size)
	}

	return Result{
		Data:    enriched,
		Message: msg,
	}, nil
}
