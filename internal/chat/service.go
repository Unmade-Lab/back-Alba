package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Unmade-Lab/back-Alba/internal/actions"
	"github.com/Unmade-Lab/back-Alba/internal/convctx"
	"github.com/Unmade-Lab/back-Alba/internal/models"
	"github.com/Unmade-Lab/back-Alba/internal/orchestrator"
	"github.com/Unmade-Lab/back-Alba/internal/widget"
	"github.com/Unmade-Lab/back-Alba/internal/ws"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// MessageRequest is the input payload for a user chat message.
type MessageRequest struct {
	SessionID string     `json:"session_id" binding:"required"`
	UserID    *uuid.UUID `json:"user_id,omitempty"`
	Message   string     `json:"message" binding:"required"`
}

// MessageResponse is the structured response returned to the client.
type MessageResponse struct {
	MessageID string         `json:"message_id"`
	SessionID string         `json:"session_id"`
	Text      string         `json:"text"`
	Widget    *widget.Widget `json:"widget,omitempty"`
	Intent    string         `json:"intent,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
}

// Service orchestrates the full chat request lifecycle.
type Service struct {
	repo          *Repository
	orchestrator  *orchestrator.Orchestrator
	registry      *actions.Registry
	ctxManager    *convctx.Manager
	widgetBuilder *widget.Builder
	hub           *ws.Hub
	logger        *zap.Logger
}

// NewService constructs the chat Service with all its dependencies.
func NewService(
	repo *Repository,
	orch *orchestrator.Orchestrator,
	registry *actions.Registry,
	ctxManager *convctx.Manager,
	wb *widget.Builder,
	hub *ws.Hub,
	logger *zap.Logger,
) *Service {
	return &Service{
		repo:          repo,
		orchestrator:  orch,
		registry:      registry,
		ctxManager:    ctxManager,
		widgetBuilder: wb,
		hub:           hub,
		logger:        logger,
	}
}

// Process is the main entry point: receives a user message and returns a structured response.
func (s *Service) Process(ctx context.Context, req MessageRequest) (*MessageResponse, error) {
	// 1. Persist user message.
	userMsg := &models.ChatMessage{
		SessionID: req.SessionID,
		UserID:    req.UserID,
		Role:      models.RoleUser,
		Content:   req.Message,
	}
	if err := s.repo.Save(ctx, userMsg); err != nil {
		return nil, fmt.Errorf("save user message: %w", err)
	}

	// 2. Load conversation context from Redis.
	convCtx, err := s.ctxManager.Load(ctx, req.SessionID)
	if err != nil {
		s.logger.Warn("load conv context", zap.Error(err))
		convCtx = &convctx.ConversationContext{SessionID: req.SessionID}
	}

	// 3. Fetch recent history for LLM context (last 10 messages).
	history, err := s.repo.GetHistory(ctx, req.SessionID, 10)
	if err != nil {
		s.logger.Warn("fetch history", zap.Error(err))
	}
	llmHistory := toOrchestratorHistory(history)

	// 4. Extract intent via AI Orchestrator.
	intent, plainText, err := s.orchestrator.ExtractIntent(ctx, req.Message, convCtx, llmHistory)
	if err != nil {
		return nil, fmt.Errorf("extract intent: %w", err)
	}

	var responseText string
	var responseWidget *widget.Widget
	var intentName string

	if intent != nil {
		// 5. Execute the resolved action.
		intentName = intent.Name
		action, ok := s.registry.Get(intent.Name)
		if !ok {
			s.logger.Warn("unknown intent", zap.String("intent", intent.Name))
			responseText = fmt.Sprintf("I understood you want to '%s', but I don't know how to do that yet.", intent.Name)
		} else {
			result, execErr := action.Execute(ctx, intent.Params)
			if execErr != nil {
				s.logger.Error("action execute", zap.String("action", intent.Name), zap.Error(execErr))
				responseText = fmt.Sprintf("I tried to %s but encountered an error: %s", intent.Name, execErr.Error())
				responseWidget = s.widgetBuilder.Error(execErr.Error())
			} else {
				// 6. Build UI widget from result.
				responseText = result.Message
				responseWidget = s.widgetBuilder.Build(intent.Name, result.Data)

				// 7. Update conversation context.
				convCtx.LastAction = intent.Name
				if result.Data != nil {
					convCtx.CurrentEntity = intent.Name
				}
				if saveErr := s.ctxManager.Save(ctx, convCtx); saveErr != nil {
					s.logger.Warn("save conv context", zap.Error(saveErr))
				}
			}
		}
	} else {
		responseText = plainText
	}

	// 8. Persist assistant response.
	assistantMsg := &models.ChatMessage{
		SessionID: req.SessionID,
		UserID:    req.UserID,
		Role:      models.RoleAssistant,
		Content:   responseText,
		Intent:    intentName,
	}
	if responseWidget != nil {
		wb, _ := json.Marshal(responseWidget)
		assistantMsg.Widget = wb
	}
	if err := s.repo.Save(ctx, assistantMsg); err != nil {
		s.logger.Error("save assistant message", zap.Error(err))
	}

	// 9. Build final response.
	response := &MessageResponse{
		MessageID: assistantMsg.ID.String(),
		SessionID: req.SessionID,
		Text:      responseText,
		Widget:    responseWidget,
		Intent:    intentName,
		CreatedAt: assistantMsg.CreatedAt,
	}

	// 10. Broadcast to any open WebSocket connections for this session.
	if payload, err := json.Marshal(response); err == nil {
		s.hub.Broadcast(req.SessionID, payload)
	}

	return response, nil
}

// GetHistory returns paginated chat history for a session.
func (s *Service) GetHistory(ctx context.Context, sessionID string, limit int) ([]*models.ChatMessage, error) {
	return s.repo.GetHistory(ctx, sessionID, limit)
}

// toOrchestratorHistory converts stored messages to the slim format the LLM expects.
func toOrchestratorHistory(msgs []*models.ChatMessage) []orchestrator.HistoryMessage {
	out := make([]orchestrator.HistoryMessage, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, orchestrator.HistoryMessage{
			Role:    m.Role,
			Content: m.Content,
		})
	}
	return out
}
