package actions

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Unmade-Lab/back-Alba/internal/events"
	"github.com/Unmade-Lab/back-Alba/internal/models"
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
func (c *Committer) Commit(ctx context.Context, actionName string, payload map[string]interface{}) error {
	switch actionName {
	case "create_company":
		return c.commitCompany(ctx, payload)
	case "create_deal":
		return c.commitDeal(ctx, payload)
	default:
		return fmt.Errorf("unknown draft action for commit: %s", actionName)
	}
}

func (c *Committer) commitCompany(ctx context.Context, payload map[string]interface{}) error {
	// Parse company object
	companyData, ok := payload["company"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("missing 'company' data in payload")
	}

	companyBytes, _ := json.Marshal(companyData)
	var company models.Company
	if err := json.Unmarshal(companyBytes, &company); err != nil {
		return fmt.Errorf("invalid company format: %w", err)
	}

	tx, err := c.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Insert company
	_, err = tx.Exec(ctx,
		`INSERT INTO companies (id, name, industry, website, email, phone)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		company.ID, company.Name, company.Industry,
		company.Website, company.Email, company.Phone,
	)
	if err != nil {
		return fmt.Errorf("insert company: %w", err)
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
		return err
	}

	c.events.Publish(ctx, events.DomainEvent{
		Type: models.EventCompanyCreated,
		Payload: map[string]interface{}{
			"entity_type": "company",
			"entity_id":   company.ID.String(),
			"name":        company.Name,
		},
	})

	return nil
}

func (c *Committer) commitDeal(ctx context.Context, payload map[string]interface{}) error {
	dealData, ok := payload["deal"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("missing 'deal' data in payload")
	}

	dealBytes, _ := json.Marshal(dealData)
	var deal models.Deal
	if err := json.Unmarshal(dealBytes, &deal); err != nil {
		return fmt.Errorf("invalid deal format: %w", err)
	}

	_, err := c.db.Exec(ctx,
		`INSERT INTO deals (id, name, amount, stage, description)
		 VALUES ($1, $2, $3, $4, $5)`,
		deal.ID, deal.Name, deal.Amount, deal.Stage, deal.Description,
	)
	if err != nil {
		return fmt.Errorf("insert deal: %w", err)
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
	return nil
}
