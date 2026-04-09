package actions

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Unmade-Lab/back-Alba/internal/events"
	"github.com/Unmade-Lab/back-Alba/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Committer handles saving finalized widgets (drafts) to the database.
type Committer struct {
	db     *pgxpool.Pool
	events *events.Dispatcher
}

func NewCommitter(db *pgxpool.Pool, dispatcher *events.Dispatcher) *Committer {
	return &Committer{db: db, events: dispatcher}
}

// Commit executes the physical database changes for a confirmed draft.
func (c *Committer) Commit(ctx context.Context, actionName string, payload map[string]interface{}) (map[string]interface{}, error) {
	switch actionName {
	case "create_company":
		return c.commitCompany(ctx, payload)
	case "create_deal":
		return c.commitDeal(ctx, payload)
	case "invite_user":
		return c.commitInviteUser(ctx, payload)
	default:
		return nil, fmt.Errorf("unknown draft action for commit: %s", actionName)
	}
}

func (c *Committer) commitCompany(ctx context.Context, payload map[string]interface{}) (map[string]interface{}, error) {
	// Parse company object
	companyData, ok := payload["company"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("missing 'company' data in payload")
	}

	companyBytes, _ := json.Marshal(companyData)
	var company models.Company
	if err := json.Unmarshal(companyBytes, &company); err != nil {
		return nil, fmt.Errorf("invalid company format: %w", err)
	}

	tx, err := c.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	wid := ctx.Value(models.CtxWorkspaceID)
	if wid == nil || wid == "" {
		return nil, fmt.Errorf("workspace context missing")
	}

	// Insert company
	_, err = tx.Exec(ctx,
		`INSERT INTO companies (id, name, industry, website, email, phone, workspace_id)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		company.ID, company.Name, company.Industry,
		company.Website, company.Email, company.Phone, wid,
	)
	if err != nil {
		return nil, fmt.Errorf("insert company: %w", err)
	}

	// Insert employees (if any)
	// We don't have an employees table in the base system yet, but this is how we'd do it:
	/*
		for _, emp := range company.Employees {
			_, err = tx.Exec(ctx, "INSERT INTO employees (id, company_id, name, department) VALUES ($1,$2,$3,$4)",
				emp.ID, company.ID, emp.Name, emp.Department)
		}
	*/

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	c.events.Publish(ctx, events.DomainEvent{
		Type: models.EventCompanyCreated,
		Payload: map[string]interface{}{
			"entity_type": "company",
			"entity_id":   company.ID.String(),
			"name":        company.Name,
		},
	})

	return map[string]interface{}{"id": company.ID.String(), "status": "success"}, nil
}

func (c *Committer) commitDeal(ctx context.Context, payload map[string]interface{}) (map[string]interface{}, error) {
	dealData, ok := payload["deal"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("missing 'deal' data in payload")
	}

	dealBytes, _ := json.Marshal(dealData)
	var deal models.Deal
	if err := json.Unmarshal(dealBytes, &deal); err != nil {
		return nil, fmt.Errorf("invalid deal format: %w", err)
	}

	wid := ctx.Value(models.CtxWorkspaceID)
	if wid == nil || wid == "" {
		return nil, fmt.Errorf("workspace context missing")
	}

	_, err := c.db.Exec(ctx,
		`INSERT INTO deals (id, name, amount, stage, description, workspace_id)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		deal.ID, deal.Name, deal.Amount, deal.Stage, deal.Description, wid,
	)
	if err != nil {
		return nil, fmt.Errorf("insert deal: %w", err)
	}

	c.events.Publish(ctx, events.DomainEvent{
		Type: models.EventDealCreated,
		Payload: map[string]interface{}{
			"entity_type": "deal",
			"entity_id":   deal.ID.String(),
			"name":        deal.Name,
			"amount":      deal.Amount,
		},
	})
	return map[string]interface{}{"id": deal.ID.String(), "status": "success"}, nil
}
func (c *Committer) commitInviteUser(ctx context.Context, payload map[string]interface{}) (map[string]interface{}, error) {
	email, ok := payload["email"].(string)
	if !ok || email == "" {
		return nil, fmt.Errorf("email is required")
	}
	name, _ := payload["name"].(string)
	if name == "" {
		name = "Новый сотрудник"
	}
	role, _ := payload["role"].(string)
	if role == "" {
		role = "user"
	}

	wid := ctx.Value(models.CtxWorkspaceID)
	if wid == nil || wid == "" {
		return nil, fmt.Errorf("workspace context missing")
	}

	token := uuid.New().String()
	
	// Create invitation in DB. Here we just set expires_at 7 days from now.
	_, err := c.db.Exec(ctx,
		`INSERT INTO invitations (email, name, role, token, workspace_id, expires_at)
		 VALUES ($1, $2, $3, $4, $5, NOW() + INTERVAL '7 days')`,
		email, name, role, token, wid,
	)
	if err != nil {
		return nil, fmt.Errorf("insert invitation: %w", err)
	}

	inviteURL := fmt.Sprintf("http://localhost:3000/invite?token=%s", token)

	return map[string]interface{}{
		"status": "success",
		"token": token,
		"invite_url": inviteURL,
		"message": fmt.Sprintf("Коллега добавлен. Отправьте ему ссылку: %s", inviteURL),
	}, nil
}
