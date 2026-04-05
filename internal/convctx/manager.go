// Package convctx manages per-session conversation context stored in Redis.
package convctx

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	keyPrefix = "crm:ctx:"
	ttl       = 24 * time.Hour
)

// ConversationContext tracks the state of a single chat session.
type ConversationContext struct {
	SessionID      string                 `json:"session_id"`
	CurrentEntity  string                 `json:"current_entity,omitempty"`
	LastEntityType string                 `json:"last_entity_type,omitempty"`
	EntityID       string                 `json:"entity_id,omitempty"`
	LastAction     string                 `json:"last_action,omitempty"`
	ActiveFilters  map[string]interface{} `json:"active_filters,omitempty"`
}

// Manager stores and retrieves conversation context using Redis.
type Manager struct {
	redis  *redis.Client
	logger *zap.Logger
}

// NewManager returns a new context Manager.
func NewManager(redisClient *redis.Client, logger *zap.Logger) *Manager {
	return &Manager{redis: redisClient, logger: logger}
}

// Load retrieves the conversation context for a session.
// Returns an empty context (not an error) if none exists yet.
func (m *Manager) Load(ctx context.Context, sessionID string) (*ConversationContext, error) {
	key := keyPrefix + sessionID
	data, err := m.redis.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return &ConversationContext{SessionID: sessionID}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("redis get context: %w", err)
	}

	var cc ConversationContext
	if err := json.Unmarshal(data, &cc); err != nil {
		return nil, fmt.Errorf("unmarshal context: %w", err)
	}
	return &cc, nil
}

// Save persists the conversation context, refreshing TTL.
func (m *Manager) Save(ctx context.Context, cc *ConversationContext) error {
	key := keyPrefix + cc.SessionID
	data, err := json.Marshal(cc)
	if err != nil {
		return fmt.Errorf("marshal context: %w", err)
	}
	if err := m.redis.Set(ctx, key, data, ttl).Err(); err != nil {
		return fmt.Errorf("redis set context: %w", err)
	}
	return nil
}

// Delete removes a session's context (e.g., on logout).
func (m *Manager) Delete(ctx context.Context, sessionID string) error {
	return m.redis.Del(ctx, keyPrefix+sessionID).Err()
}
