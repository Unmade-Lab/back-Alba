package actions

import (
	"context"
	"fmt"

	"github.com/Unmade-Lab/back-Alba/internal/events"
	"github.com/Unmade-Lab/back-Alba/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CreateCompanyAction handles the "create_company" intent.
type CreateCompanyAction struct {
	db     *pgxpool.Pool
	events *events.Dispatcher
}

func NewCreateCompanyAction(db *pgxpool.Pool, dispatcher *events.Dispatcher) *CreateCompanyAction {
	return &CreateCompanyAction{db: db, events: dispatcher}
}

func (a *CreateCompanyAction) Name() string        { return "create_company" }
func (a *CreateCompanyAction) Description() string {
	return "Create a new company (account) in the CRM. Use when the user mentions a business, organization, or client they want to track."
}

func (a *CreateCompanyAction) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{
				"type":        "string",
				"description": "Company name.",
			},
			"industry": map[string]interface{}{
				"type":        "string",
				"description": "Industry the company operates in.",
			},
			"website": map[string]interface{}{
				"type":        "string",
				"description": "Company website URL.",
			},
			"email": map[string]interface{}{
				"type":        "string",
				"description": "Primary contact email.",
			},
			"phone": map[string]interface{}{
				"type":        "string",
				"description": "Primary phone number.",
			},
		},
		"required": []string{"name"},
	}
}

func (a *CreateCompanyAction) Execute(ctx context.Context, params map[string]interface{}) (Result, error) {
	name, ok := params["name"].(string)
	if !ok || name == "" {
		return Result{}, fmt.Errorf("'name' is required")
	}

	company := &models.Company{
		ID:       uuid.New(),
		Name:     name,
		Industry: strParam(params, "industry"),
		Website:  strParam(params, "website"),
		Email:    strParam(params, "email"),
		Phone:    strParam(params, "phone"),
	}

	_, err := a.db.Exec(ctx,
		`INSERT INTO companies (id, name, industry, website, email, phone)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		company.ID, company.Name, company.Industry,
		company.Website, company.Email, company.Phone,
	)
	if err != nil {
		return Result{}, fmt.Errorf("insert company: %w", err)
	}

	a.events.Publish(ctx, events.DomainEvent{
		Type: models.EventCompanyCreated,
		Payload: map[string]interface{}{
			"entity_type": "company",
			"entity_id":   company.ID.String(),
			"name":        company.Name,
		},
	})

	return Result{
		Data:    company,
		Message: fmt.Sprintf("Company '%s' created successfully.", company.Name),
	}, nil
}

func strParam(params map[string]interface{}, key string) string {
	v, _ := params[key].(string)
	return v
}
