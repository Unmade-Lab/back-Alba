package events

import (
	"context"
	"encoding/json"

	"github.com/Unmade-Lab/back-Alba/internal/models"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// RegisterDefaultHandlers wires up all built-in event handlers.
func RegisterDefaultHandlers(d *Dispatcher, db *pgxpool.Pool, logger *zap.Logger) {
	d.Subscribe(models.EventDealCreated, persistEventHandler(db, logger))
	d.Subscribe(models.EventDealStageChanged, persistEventHandler(db, logger))
	d.Subscribe(models.EventCompanyCreated, persistEventHandler(db, logger))
	d.Subscribe(models.EventTaskCreated, persistEventHandler(db, logger))
}

// persistEventHandler returns a handler that writes events to the events table.
func persistEventHandler(db *pgxpool.Pool, logger *zap.Logger) Handler {
	return func(event DomainEvent) {
		ctx := context.Background()

		payload, err := json.Marshal(event.Payload)
		if err != nil {
			logger.Error("marshal event payload", zap.Error(err))
			return
		}

		entityType, _ := event.Payload["entity_type"].(string)
		entityIDStr, _ := event.Payload["entity_id"].(string)

		var entityID *uuid.UUID
		if entityIDStr != "" {
			if id, err := uuid.Parse(entityIDStr); err == nil {
				entityID = &id
			}
		}

		_, err = db.Exec(ctx,
			`INSERT INTO events (event_type, entity_type, entity_id, payload)
			 VALUES ($1, $2, $3, $4)`,
			event.Type, entityType, entityID, payload,
		)
		if err != nil {
			logger.Error("persist event to db", zap.String("event", event.Type), zap.Error(err))
			return
		}

		logger.Info("event persisted", zap.String("type", event.Type))
	}
}
