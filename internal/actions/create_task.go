package actions

import (
	"context"
	"fmt"
	"time"

	"github.com/Unmade-Lab/back-Alba/internal/convctx"
	"github.com/Unmade-Lab/back-Alba/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// CreateTaskAction handles the creation of a new task from an AI command.
type CreateTaskAction struct {
	db     *pgxpool.Pool
	logger *zap.Logger
}

func NewCreateTaskAction(db *pgxpool.Pool, logger *zap.Logger) *CreateTaskAction {
	return &CreateTaskAction{db: db, logger: logger}
}

func (a *CreateTaskAction) Name() string        { return "create_task" }
func (a *CreateTaskAction) Description() string {
	return "Creates a new task or reminder. Use this when the user wants to set a reminder, create a to-do, or assign a task to someone."
}

func (a *CreateTaskAction) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"title": map[string]interface{}{
				"type":        "string",
				"description": "Short title of the task (e.g. 'Call John at Tesla').",
			},
			"description": map[string]interface{}{
				"type":        "string",
				"description": "Optional longer description of the task.",
			},
			"due_at": map[string]interface{}{
				"type":        "string",
				"description": "Due date/time in ISO 8601 format (e.g. '2026-04-13T10:00:00Z'). Infer from relative phrases like 'tomorrow', 'next week'.",
			},
			"priority": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"low", "medium", "high", "urgent"},
				"description": "Priority level of the task. Default is 'medium'.",
			},
			"entity_type": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"deal", "company", "contact"},
				"description": "Optional entity type this task is linked to.",
			},
			"entity_id": map[string]interface{}{
				"type":        "string",
				"description": "Optional UUID of the deal, company, or contact this task is linked to.",
			},
		},
		"required": []string{"title"},
	}
}

func (a *CreateTaskAction) Execute(ctx context.Context, convCtx *convctx.ConversationContext, args map[string]interface{}) (Result, error) {
	widStr := ctx.Value(models.CtxWorkspaceID)
	if widStr == nil {
		return Result{}, fmt.Errorf("workspace context missing")
	}
	wid, err := uuid.Parse(widStr.(string))
	if err != nil {
		return Result{}, fmt.Errorf("invalid workspace_id: %w", err)
	}

	title, _ := args["title"].(string)
	if title == "" {
		return Result{}, fmt.Errorf("task title is required")
	}

	description, _ := args["description"].(string)
	priority := models.TaskPriorityMedium
	if p, ok := args["priority"].(string); ok && p != "" {
		priority = models.TaskPriority(p)
	}

	var dueAt *time.Time
	if dueStr, ok := args["due_at"].(string); ok && dueStr != "" {
		if t, err := time.Parse(time.RFC3339, dueStr); err == nil {
			dueAt = &t
		}
	}

	var entityID *uuid.UUID
	if eid, ok := args["entity_id"].(string); ok && eid != "" {
		if parsed, err := uuid.Parse(eid); err == nil {
			entityID = &parsed
		}
	}
	entityType, _ := args["entity_type"].(string)

	task := &models.Task{
		ID:          uuid.New(),
		WorkspaceID: wid,
		Title:       title,
		Description: description,
		Priority:    priority,
		Status:      models.TaskStatusPending,
		DueAt:       dueAt,
		EntityID:    entityID,
		EntityType:  entityType,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	_, err = a.db.Exec(ctx,
		`INSERT INTO tasks (id, workspace_id, title, description, priority, status, due_at, entity_id, entity_type, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		task.ID, task.WorkspaceID, task.Title, task.Description,
		task.Priority, task.Status, task.DueAt, task.EntityID, task.EntityType,
		task.CreatedAt, task.UpdatedAt,
	)
	if err != nil {
		return Result{}, fmt.Errorf("insert task: %w", err)
	}

	a.logger.Info("task created", zap.String("task_id", task.ID.String()), zap.String("title", task.Title))

	return Result{
		Data:    task,
		Message: fmt.Sprintf("Задача «%s» создана.", task.Title),
	}, nil
}
