package actions

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Unmade-Lab/back-Alba/internal/convctx"
	"github.com/Unmade-Lab/back-Alba/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CompleteOnboardingAction handles finalizing the onboarding process.
type CompleteOnboardingAction struct {
	db   *pgxpool.Pool
	repo OnboardingRepo // We'll need a slim interface or just use the Repo
}

type OnboardingRepo interface {
	UpdateWorkspaceOnboarding(ctx context.Context, id uuid.UUID, industry string, data json.RawMessage) error
	SeedOnboardingTemplates(ctx context.Context, wid uuid.UUID, pipelineName string, stages []string, departments []string) (uuid.UUID, uuid.UUID, uuid.UUID, error)
	LinkUserToDepartment(ctx context.Context, userID, departmentID uuid.UUID) error
}

func NewCompleteOnboardingAction(db *pgxpool.Pool, repo OnboardingRepo) *CompleteOnboardingAction {
	return &CompleteOnboardingAction{db: db, repo: repo}
}

func (a *CompleteOnboardingAction) Name() string { return "complete_onboarding" }
func (a *CompleteOnboardingAction) Description() string {
	return "Finalize workspace setup based on collected company info. Creates default pipelines and departments."
}

func (a *CompleteOnboardingAction) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"industry": map[string]interface{}{
				"type":        "string",
				"description": "Primary industry (e.g., 'saas', 'real_estate', 'services', 'other').",
			},
			"company_name": map[string]interface{}{
				"type":        "string",
				"description": "Official company name.",
			},
			"team_size": map[string]interface{}{
				"type":        "string",
				"description": "Approximate number of team members.",
			},
		},
		"required": []string{"industry"},
	}
}

func (a *CompleteOnboardingAction) Execute(ctx context.Context, convCtx *convctx.ConversationContext, params map[string]interface{}) (Result, error) {
	widStr := ctx.Value(models.CtxWorkspaceID)
	if widStr == nil {
		return Result{}, fmt.Errorf("workspace context missing")
	}
	wid, err := uuid.Parse(widStr.(string))
	if err != nil {
		return Result{}, fmt.Errorf("invalid workspace_id")
	}

	industry, _ := params["industry"].(string)
	
	// Define templates based on industry
	var stages []string
	var departments []string
	pipelineName := "Sales Funnel"

	switch industry {
	case "saas":
		stages = []string{"Lead", "Discovery", "Demo", "Trial", "Contract", "Won", "Lost"}
		departments = []string{"Sales", "Customer Success", "Product", "Engineering"}
	case "real_estate":
		stages = []string{"New Lead", "Viewing", "Offer", "Deposit", "Legal", "Closed"}
		departments = []string{"Sales / Agents", "Operations", "Legal"}
	case "services":
		stages = []string{"Inquiry", "Proposal", "Negotiation", "Agreement", "Active", "Fulfilled"}
		departments = []string{"Sales", "Delivery / Production", "Admin"}
	default:
		stages = []string{"Lead", "Qualified", "Proposal", "Negotiation", "Closed Won", "Closed Lost"}
		departments = []string{"Sales", "Management"}
	}

	// 1. Seed Templates
	_, _, firstDeptID, err := a.repo.SeedOnboardingTemplates(ctx, wid, pipelineName, stages, departments)
	if err != nil {
		return Result{}, fmt.Errorf("seed templates: %w", err)
	}

	// 2. Link current user (admin) to the first department
	uIDStr := ctx.Value(models.CtxUserID)
	if uIDStr != nil {
		uID, _ := uuid.Parse(uIDStr.(string))
		if firstDeptID != uuid.Nil {
			_ = a.repo.LinkUserToDepartment(ctx, uID, firstDeptID)
		}
	}

	// 3. Mark Workspace as Onboarded
	metadata, _ := json.Marshal(params)
	if err := a.repo.UpdateWorkspaceOnboarding(ctx, wid, industry, metadata); err != nil {
		return Result{}, fmt.Errorf("update workspace status: %w", err)
	}

	return Result{
		Data: map[string]interface{}{
			"industry":    industry,
			"stages":      stages,
			"departments": departments,
		},
		Message: fmt.Sprintf("Great! I've set up your workspace for the %s industry. I've created a custom sales funnel and initialized your department structure. You're all set to start using Alba!", industry),
	}, nil
}
