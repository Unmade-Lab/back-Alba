package models

import (
	"time"

	"github.com/google/uuid"
)

// TaskStatus represents the lifecycle state of a task.
type TaskStatus string

const (
	TaskStatusPending    TaskStatus = "pending"
	TaskStatusInProgress TaskStatus = "in_progress"
	TaskStatusCompleted  TaskStatus = "completed"
	TaskStatusCancelled  TaskStatus = "cancelled"
)

// TaskPriority represents the urgency of a task.
type TaskPriority string

const (
	TaskPriorityLow    TaskPriority = "low"
	TaskPriorityMedium TaskPriority = "medium"
	TaskPriorityHigh   TaskPriority = "high"
	TaskPriorityUrgent TaskPriority = "urgent"
)

// Task represents a to-do item or reminder in the CRM.
type Task struct {
	ID          uuid.UUID    `json:"id"`
	WorkspaceID uuid.UUID    `json:"workspace_id"`
	CreatedBy   *uuid.UUID   `json:"created_by,omitempty"`
	AssignedTo  *uuid.UUID   `json:"assigned_to,omitempty"`
	EntityID    *uuid.UUID   `json:"entity_id,omitempty"`   // Link to deal/company
	EntityType  string       `json:"entity_type,omitempty"` // "deal", "company", etc.
	Title       string       `json:"title"`
	Description string       `json:"description,omitempty"`
	DueAt       *time.Time   `json:"due_at,omitempty"`
	Status      TaskStatus   `json:"status"`
	Priority    TaskPriority `json:"priority"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
}
