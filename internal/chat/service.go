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
	Type      string         `json:"type,omitempty"` // e.g. "message"
	MessageID string         `json:"message_id"`
	SessionID string         `json:"session_id"`
	Text      string         `json:"text"`
	Widget    *widget.Widget `json:"widget,omitempty"`
	Intent    string         `json:"intent,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
}

// StreamEvent represents a real-time streaming update for the current message.
type StreamEvent struct {
	Type      string `json:"type"`       // "stream_start" or "stream_chunk"
	SessionID string `json:"session_id"`
	MessageID string `json:"message_id"`
	Text      string `json:"text,omitempty"`
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

	// --- Onboarding Check ---
	var onboardingContext string
	widStr := ctx.Value(models.CtxWorkspaceID)
	if widStr != nil {
		wid, _ := uuid.Parse(widStr.(string))
		workspace, _ := s.repo.GetWorkspace(ctx, wid)
		if workspace != nil && !workspace.OnboardingCompleted {
			onboardingContext = "THE WORKSPACE IS NEW. You are in ONBOARDING MODE. Ask about business type, goals, and team. Once you have enough info, call 'complete_onboarding'."
		}
	}

	// If this is the very first message in the session, automatically create a Session record with an AI-generated title.
	if len(history) == 1 && req.UserID != nil {
		go func(msgText string, sessionID string, uID uuid.UUID) {
			title, err := s.orchestrator.GenerateTitle(context.Background(), msgText)
			if err != nil {
				title = "New Chat"
				s.logger.Warn("failed to generate title", zap.Error(err))
			}
			if err := s.repo.CreateSession(context.Background(), sessionID, uID, title); err != nil {
				s.logger.Error("failed to create session", zap.Error(err))
			}
		}(req.Message, req.SessionID, *req.UserID)
	}

	// 4. Prepare stream start and callback.
	assistantMsgID := uuid.New().String()
	
	streamStart := StreamEvent{
		Type:      "stream_start",
		SessionID: req.SessionID,
		MessageID: assistantMsgID,
	}
	if b, err := json.Marshal(streamStart); err == nil {
		s.hub.Broadcast(req.SessionID, b)
	}

	onChunk := func(textChunk string) {
		chunkEvt := StreamEvent{
			Type:      "stream_chunk",
			SessionID: req.SessionID,
			MessageID: assistantMsgID,
			Text:      textChunk,
		}
		if b, err := json.Marshal(chunkEvt); err == nil {
			s.hub.Broadcast(req.SessionID, b)
		}
	}

	// 5. Extract intent via AI Orchestrator (with streaming callback).
	intent, plainText, err := s.orchestrator.ExtractIntent(ctx, req.Message, convCtx, llmHistory, onboardingContext, onChunk)
	if err != nil {
		return nil, fmt.Errorf("extract intent: %w", err)
	}

	var responseText string
	var responseWidget *widget.Widget
	var intentName string

	if intent != nil {
		// 6. Execute the resolved action.
		intentName = intent.Name
		action, ok := s.registry.Get(intent.Name)
		if !ok {
			s.logger.Warn("unknown intent", zap.String("intent", intent.Name))
			responseText = fmt.Sprintf("I understood you want to '%s', but I don't know how to do that yet.", intent.Name)
		} else {
			result, execErr := action.Execute(ctx, convCtx, intent.Params)
			if execErr != nil {
				s.logger.Error("action execute", zap.String("action", intent.Name), zap.Error(execErr))
				responseText = fmt.Sprintf("I tried to %s but encountered an error: %s", intent.Name, execErr.Error())
				responseWidget = s.widgetBuilder.Error(execErr.Error())
			} else {
				// 7. Build UI widget from result.
				responseText = result.Message
				responseWidget = s.widgetBuilder.Build(intent.Name, result.Data)

				// Simulate streaming for the action response text
				onChunk(responseText)

				// 8. Update conversation context.
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

	// 9. Persist assistant response.
	assistantMsg := &models.ChatMessage{
		ID:        uuid.MustParse(assistantMsgID),
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
		Type:      "message",
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

// InitializeOnboarding creates the first chat session and a proactive AI greeting for new workspaces.
func (s *Service) InitializeOnboarding(ctx context.Context, userID uuid.UUID, workspaceID uuid.UUID) error {
	// 1. Generate a session ID
	sessionID := uuid.New().String()

	// 2. Create the session in DB
	// We need to inject workspace_id into ctx for repo methods to work
	ctx = context.WithValue(ctx, models.CtxWorkspaceID, workspaceID.String())
	
	if err := s.repo.CreateSession(ctx, sessionID, userID, "Welcome to Alba"); err != nil {
		return fmt.Errorf("create initial session: %w", err)
	}

	// 3. Prepare onboarding context for the AI
	onboardingContext := "THE WORKSPACE IS NEW. You are in ONBOARDING MODE. This is your FIRST interaction with this user. Introduce yourself as Alba, your AI CRM assistant, and ask to start the onboarding to set up the workspace."

	// 4. Generate AI greeting
	greeting, err := s.orchestrator.GenerateWelcome(ctx, onboardingContext)
	if err != nil {
		s.logger.Error("failed to generate welcome message", zap.Error(err))
		greeting = "Привет! Я Альба, твой AI-помощник в управлении CRM. Давай настроим твой воркспейс, чтобы тебе было удобно работать?"
	}

	// 5. Save AI message
	assistantMsg := &models.ChatMessage{
		SessionID: sessionID,
		UserID:    &userID,
		Role:      models.RoleAssistant,
		Content:   greeting,
	}

	if err := s.repo.Save(ctx, assistantMsg); err != nil {
		return fmt.Errorf("save welcome message: %w", err)
	}

	return nil
}

// GetHistory returns paginated chat history for a session.
func (s *Service) GetHistory(ctx context.Context, sessionID string, limit int) ([]*models.ChatMessage, error) {
	return s.repo.GetHistory(ctx, sessionID, limit)
}

// GetSessions returns all chat sessions for the user.
func (s *Service) GetSessions(ctx context.Context, userID uuid.UUID) ([]*models.ChatSession, error) {
	return s.repo.GetSessions(ctx, userID)
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
