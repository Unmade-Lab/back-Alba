package ws

import (
	"encoding/json"
	"time"

	"github.com/Unmade-Lab/back-Alba/internal/events"
)

// NotificationType defines the category of a push notification.
type NotificationType string

const (
	NotifyDealCreated      NotificationType = "deal.created"
	NotifyDealStageChanged NotificationType = "deal.stage_changed"
	NotifyTaskCreated      NotificationType = "task.created"
	NotifyTaskOverdue      NotificationType = "task.overdue"
	NotifyCompanyCreated   NotificationType = "company.created"
)

// Notification is the payload sent to the frontend via WebSocket.
type Notification struct {
	Type      NotificationType       `json:"type"`
	Title     string                 `json:"title"`
	Body      string                 `json:"body"`
	Data      map[string]interface{} `json:"data,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
}

// RegisterNotificationHandlers wires domain events to workspace-level WS push notifications.
func RegisterNotificationHandlers(dispatcher *events.Dispatcher, hub *Hub) {
	// Deal Created
	dispatcher.Subscribe("deal.created", func(event events.DomainEvent) {
		workspaceID, _ := event.Payload["workspace_id"].(string)
		dealName, _ := event.Payload["name"].(string)
		amount, _ := event.Payload["amount"].(float64)

		notify(hub, workspaceID, Notification{
			Type:      NotifyDealCreated,
			Title:     "Новая сделка",
			Body:      dealName + " добавлена в CRM",
			Data:      map[string]interface{}{"amount": amount},
			Timestamp: time.Now(),
		})
	})

	// Deal Stage Changed
	dispatcher.Subscribe("deal.stage_changed", func(event events.DomainEvent) {
		workspaceID, _ := event.Payload["workspace_id"].(string)
		dealName, _ := event.Payload["deal_name"].(string)
		newStage, _ := event.Payload["new_stage"].(string)

		notify(hub, workspaceID, Notification{
			Type:      NotifyDealStageChanged,
			Title:     "Сделка обновлена",
			Body:      dealName + " перешла в «" + newStage + "»",
			Data:      event.Payload,
			Timestamp: time.Now(),
		})
	})

	// Task Created
	dispatcher.Subscribe("task.created", func(event events.DomainEvent) {
		workspaceID, _ := event.Payload["workspace_id"].(string)
		taskTitle, _ := event.Payload["title"].(string)

		notify(hub, workspaceID, Notification{
			Type:      NotifyTaskCreated,
			Title:     "Задача создана",
			Body:      taskTitle,
			Data:      event.Payload,
			Timestamp: time.Now(),
		})
	})

	// Company Created
	dispatcher.Subscribe("company.created", func(event events.DomainEvent) {
		workspaceID, _ := event.Payload["workspace_id"].(string)
		companyName, _ := event.Payload["name"].(string)

		notify(hub, workspaceID, Notification{
			Type:      NotifyCompanyCreated,
			Title:     "Новая компания",
			Body:      companyName + " добавлена в базу",
			Data:      event.Payload,
			Timestamp: time.Now(),
		})
	})
}

// notify serializes a Notification and broadcasts it to the workspace.
func notify(hub *Hub, workspaceID string, n Notification) {
	if workspaceID == "" {
		return
	}
	payload, err := json.Marshal(map[string]interface{}{
		"type":         "notification",
		"notification": n,
	})
	if err != nil {
		return
	}
	hub.BroadcastToWorkspace(workspaceID, payload)
}
