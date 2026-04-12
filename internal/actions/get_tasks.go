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

// GetTasksAction retrieves pending tasks for the workspace.
type GetTasksAction struct {
	db     *pgxpool.Pool
	logger *zap.Logger
}

func NewGetTasksAction(db *pgxpool.Pool, logger *zap.Logger) *GetTasksAction {
	return &GetTasksAction{db: db, logger: logger}
}

func (a *GetTasksAction) Name() string        { return "get_tasks" }
func (a *GetTasksAction) Description() string {
	return "Retrieves a list of tasks. Use when the user asks about pending tasks, reminders, or what needs to be done."
}

func (a *GetTasksAction) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"status": map[string]interface{}{
				"type":        "string",
				"enum":        []string{"pending", "in_progress", "completed", "all"},
				"description": "Filter tasks by status. Default is 'pending'.",
			},
		},
	}
}

func (a *GetTasksAction) Execute(ctx context.Context, convCtx *convctx.ConversationContext, args map[string]interface{}) (Result, error) {
	widStr := ctx.Value(models.CtxWorkspaceID)
	if widStr == nil {
		return Result{}, fmt.Errorf("workspace context missing")
	}
	wid, err := uuid.Parse(widStr.(string))
	if err != nil {
		return Result{}, fmt.Errorf("invalid workspace_id: %w", err)
	}

	status := "pending"
	showAll := false
	if s, ok := args["status"].(string); ok {
		if s == "all" {
			showAll = true
		} else if s != "" {
			status = s
		}
	}

	var rows interface {
		Next() bool
		Scan(...interface{}) error
		Close()
	}

	if showAll {
		rows, err = a.db.Query(ctx,
			`SELECT id, title, description, priority, status, due_at, entity_type, created_at
			 FROM tasks WHERE workspace_id = $1 ORDER BY due_at ASC NULLS LAST LIMIT 20`,
			wid,
		)
	} else {
		rows, err = a.db.Query(ctx,
			`SELECT id, title, description, priority, status, due_at, entity_type, created_at
			 FROM tasks WHERE workspace_id = $1 AND status = $2 ORDER BY due_at ASC NULLS LAST LIMIT 20`,
			wid, status,
		)
	}
	if err != nil {
		return Result{}, fmt.Errorf("query tasks: %w", err)
	}
	defer rows.Close()

	var tasks []*models.Task
	for rows.Next() {
		var t models.Task
		var dueAt *time.Time
		if err := rows.Scan(&t.ID, &t.Title, &t.Description, &t.Priority, &t.Status, &dueAt, &t.EntityType, &t.CreatedAt); err != nil {
			return Result{}, fmt.Errorf("scan task: %w", err)
		}
		t.DueAt = dueAt
		tasks = append(tasks, &t)
	}
	if tasks == nil {
		tasks = []*models.Task{}
	}

	msg := fmt.Sprintf("Найдено %d задач.", len(tasks))
	if len(tasks) == 0 {
		msg = "Задач нет."
	}

	return Result{
		Data:    tasks,
		Message: msg,
	}, nil
}
