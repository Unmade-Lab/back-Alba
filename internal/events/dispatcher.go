package events

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// DomainEvent carries the event type and an arbitrary payload.
type DomainEvent struct {
	Type    string                 `json:"type"`
	Payload map[string]interface{} `json:"payload"`
}

// Handler is a function that processes a DomainEvent.
type Handler func(event DomainEvent)

// Dispatcher is an in-process event bus with optional Redis Pub/Sub publishing.
type Dispatcher struct {
	mu       sync.RWMutex
	handlers map[string][]Handler
	redis    *redis.Client
	logger   *zap.Logger
}

// NewDispatcher creates a Dispatcher. redisClient may be nil to skip pub/sub.
func NewDispatcher(redisClient *redis.Client, logger *zap.Logger) *Dispatcher {
	return &Dispatcher{
		handlers: make(map[string][]Handler),
		redis:    redisClient,
		logger:   logger,
	}
}

// Subscribe registers a handler for the given event type.
func (d *Dispatcher) Subscribe(eventType string, handler Handler) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.handlers[eventType] = append(d.handlers[eventType], handler)
}

// Publish dispatches an event to all registered handlers and publishes to Redis.
func (d *Dispatcher) Publish(ctx context.Context, event DomainEvent) {
	d.mu.RLock()
	handlers := d.handlers[event.Type]
	d.mu.RUnlock()

	// Call in-process handlers in background goroutines.
	for _, h := range handlers {
		go func(fn Handler) {
			defer func() {
				if r := recover(); r != nil {
					d.logger.Error("event handler panic", zap.Any("recovered", r))
				}
			}()
			fn(event)
		}(h)
	}

	// Publish to Redis Pub/Sub for potential out-of-process consumers.
	if d.redis != nil {
		payload, _ := json.Marshal(event)
		if err := d.redis.Publish(ctx, "crm:events:"+event.Type, payload).Err(); err != nil {
			d.logger.Warn("redis publish failed", zap.String("event", event.Type), zap.Error(err))
		}
	}
}
