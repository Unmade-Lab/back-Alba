package orchestrator

import (
	"context"
	"fmt"
	"strings"

	"github.com/Unmade-Lab/back-Alba/internal/actions"
	"github.com/Unmade-Lab/back-Alba/internal/convctx"
	"github.com/google/generative-ai-go/genai"
	"go.uber.org/zap"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

// Orchestrator converts a user message + context into a structured Intent using Gemini.
type Orchestrator struct {
	client   *genai.Client
	model    string
	registry *actions.Registry
	logger   *zap.Logger
	mockMode bool // true when no API key is configured
}

// New creates an Orchestrator. If apiKey is empty it runs in keyword-fallback mode.
func New(apiKey, model string, registry *actions.Registry, logger *zap.Logger) *Orchestrator {
	o := &Orchestrator{
		model:    model,
		registry: registry,
		logger:   logger,
		mockMode: apiKey == "",
	}
	if o.mockMode {
		logger.Warn("GEMINI_API_KEY not set — orchestrator running in keyword-fallback mode")
	}
	// Client creation is deferred to ExtractIntent to allow ctx propagation.
	if !o.mockMode {
		client, err := genai.NewClient(context.Background(), option.WithAPIKey(apiKey))
		if err != nil {
			logger.Error("failed to create Gemini client, falling back to keyword mode", zap.Error(err))
			o.mockMode = true
		} else {
			o.client = client
		}
	}
	return o
}

// ExtractIntent analyses the user message and returns a structured Intent.
// Returns (nil, plainText, nil) when no CRM action is triggered.
func (o *Orchestrator) ExtractIntent(
	ctx context.Context,
	message string,
	convCtx *convctx.ConversationContext,
	history []HistoryMessage,
	onboardingContext string,
	onChunk func(string),
) (*Intent, string, error) {
	if o.mockMode {
		return o.keywordFallback(message, onChunk)
	}
	return o.geminiExtract(ctx, message, convCtx, history, onboardingContext, onChunk)
}

// GenerateTitle generates a short 3-5 word title for the chat session based on the first message.
func (o *Orchestrator) GenerateTitle(ctx context.Context, firstMessage string) (string, error) {
	if o.mockMode {
		words := strings.Fields(firstMessage)
		if len(words) > 4 {
			return strings.Join(words[:4], " ") + "...", nil
		}
		return firstMessage, nil
	}

	model := o.client.GenerativeModel(o.model)
	model.SystemInstruction = &genai.Content{
		Parts: []genai.Part{genai.Text("You are an assistant that creates short, catchy 3-4 word titles for chat conversations based on the user's first message. Return ONLY the title, no quotes, no extra text.")},
	}

	resp, err := model.GenerateContent(ctx, genai.Text(firstMessage))
	if err != nil {
		return "New Chat", fmt.Errorf("generate title: %w", err)
	}

	if len(resp.Candidates) > 0 && len(resp.Candidates[0].Content.Parts) > 0 {
		if t, ok := resp.Candidates[0].Content.Parts[0].(genai.Text); ok {
			return strings.TrimSpace(string(t)), nil
		}
	}
	return "New Chat", nil
}

// GenerateWelcome generates a proactive greeting message for a new user.
func (o *Orchestrator) GenerateWelcome(ctx context.Context, onboardingContext string) (string, error) {
	if o.mockMode {
		return "Привет! Я Альба, твой AI-помощник. Давай настроим твой воркспейс?", nil
	}

	model := o.client.GenerativeModel(o.model)
	model.SystemInstruction = &genai.Content{
		Parts: []genai.Part{genai.Text(SystemPrompt("", onboardingContext))},
	}

	prompt := "Say hello to the new user and briefly explain that you are their AI CRM assistant. Ask them to start the onboarding process to configure their workspace."
	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return "", fmt.Errorf("generate welcome: %w", err)
	}

	if len(resp.Candidates) > 0 && len(resp.Candidates[0].Content.Parts) > 0 {
		if t, ok := resp.Candidates[0].Content.Parts[0].(genai.Text); ok {
			return strings.TrimSpace(string(t)), nil
		}
	}
	return "Привет! Я готов помочь вам в настройке CRM.", nil
}

// ─────────────────────────────────────────────
// Gemini path (function calling)
// ─────────────────────────────────────────────

func (o *Orchestrator) geminiExtract(
	ctx context.Context,
	message string,
	convCtx *convctx.ConversationContext,
	history []HistoryMessage,
	onboardingContext string,
	onChunk func(string),
) (*Intent, string, error) {
	model := o.client.GenerativeModel(o.model)
	model.SystemInstruction = &genai.Content{
		Parts: []genai.Part{genai.Text(SystemPrompt(buildContextSummary(convCtx), onboardingContext))},
	}

	model.Tools = o.buildTools()
	session := model.StartChat()
	session.History = toGeminiHistory(history)

	iter := session.SendMessageStream(ctx, genai.Text(message))

	var sb strings.Builder
	var intent *Intent

	for {
		resp, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, "", fmt.Errorf("gemini stream: %w", err)
		}

		if len(resp.Candidates) == 0 {
			continue
		}

		for _, part := range resp.Candidates[0].Content.Parts {
			if fc, ok := part.(genai.FunctionCall); ok {
				intent = &Intent{
					Name:   fc.Name,
					Params: fc.Args,
				}
				o.logger.Info("intent extracted via Gemini",
					zap.String("intent", intent.Name),
				)
			} else if t, ok := part.(genai.Text); ok {
				chunk := string(t)
				sb.WriteString(chunk)
				if onChunk != nil {
					onChunk(chunk)
				}
			}
		}
	}

	return intent, sb.String(), nil
}

// buildTools converts all registered actions into Gemini FunctionDeclarations.
func (o *Orchestrator) buildTools() []*genai.Tool {
	var decls []*genai.FunctionDeclaration
	for _, a := range o.registry.All() {
		decls = append(decls, &genai.FunctionDeclaration{
			Name:        a.Name(),
			Description: a.Description(),
			Parameters:  schemaToGenai(a.Schema()),
		})
	}
	return []*genai.Tool{{FunctionDeclarations: decls}}
}

// schemaToGenai converts the action's map[string]interface{} JSON Schema
// into the genai.Schema type that the Gemini SDK expects.
func schemaToGenai(schema map[string]interface{}) *genai.Schema {
	s := &genai.Schema{Type: genai.TypeObject}

	if props, ok := schema["properties"].(map[string]interface{}); ok {
		s.Properties = make(map[string]*genai.Schema)
		for k, v := range props {
			vm, ok := v.(map[string]interface{})
			if !ok {
				continue
			}
			ps := &genai.Schema{}
			if t, ok := vm["type"].(string); ok {
				ps.Type = strToGenaiType(t)
			}
			if desc, ok := vm["description"].(string); ok {
				ps.Description = desc
			}
			if enum, ok := vm["enum"].([]string); ok {
				ps.Enum = enum
			}
			s.Properties[k] = ps
		}
	}

	if req, ok := schema["required"].([]string); ok {
		s.Required = req
	}

	return s
}

func strToGenaiType(t string) genai.Type {
	switch t {
	case "string":
		return genai.TypeString
	case "number":
		return genai.TypeNumber
	case "integer":
		return genai.TypeInteger
	case "boolean":
		return genai.TypeBoolean
	case "array":
		return genai.TypeArray
	case "object":
		return genai.TypeObject
	default:
		return genai.TypeString
	}
}

// toGeminiHistory converts slim HistoryMessage slices into genai.Content history.
func toGeminiHistory(history []HistoryMessage) []*genai.Content {
	var out []*genai.Content
	for _, h := range history {
		role := h.Role
		// Gemini uses "model" instead of "assistant".
		if role == "assistant" {
			role = "model"
		}
		out = append(out, &genai.Content{
			Role:  role,
			Parts: []genai.Part{genai.Text(h.Content)},
		})
	}
	return out
}

// ─────────────────────────────────────────────
// Keyword fallback (no API key)
// ─────────────────────────────────────────────

func (o *Orchestrator) keywordFallback(message string, onChunk func(string)) (*Intent, string, error) {
	lower := strings.ToLower(message)

	switch {
	case containsAny(lower, "create deal", "add deal", "new deal", "make a deal"):
		name := extractQuoted(message)
		if name == "" {
			name = "New Deal"
		}
		return &Intent{
			Name:   "create_deal",
			Params: map[string]interface{}{"name": name, "amount": float64(0)},
		}, "", nil

	case containsAny(lower, "create company", "add company", "new company"):
		name := extractQuoted(message)
		if name == "" {
			name = "New Company"
		}
		return &Intent{
			Name:   "create_company",
			Params: map[string]interface{}{"name": name},
		}, "", nil

	case containsAny(lower, "show deals", "list deals", "get deals", "my deals"):
		return &Intent{Name: "get_deals", Params: map[string]interface{}{}}, "", nil

	case containsAny(lower, "update deal", "move deal", "change stage"):
		return &Intent{
			Name:   "update_deal_stage",
			Params: map[string]interface{}{"stage": "qualified"},
		}, "", nil
	}

	text := "I'm running in offline mode. Please set GEMINI_API_KEY to enable full AI capabilities."
	if onChunk != nil {
		// Simulate streaming feeling
		onChunk("I'm running in offline mode. ")
		onChunk("Please set GEMINI_API_KEY ")
		onChunk("to enable full AI capabilities.")
	}
	return nil, text, nil
}

func containsAny(s string, keywords ...string) bool {
	for _, kw := range keywords {
		if strings.Contains(s, kw) {
			return true
		}
	}
	return false
}

func extractQuoted(s string) string {
	start := strings.Index(s, `"`)
	if start == -1 {
		return ""
	}
	end := strings.Index(s[start+1:], `"`)
	if end == -1 {
		return ""
	}
	return s[start+1 : start+1+end]
}

func buildContextSummary(cc *convctx.ConversationContext) string {
	if cc == nil {
		return ""
	}
	var parts []string
	if cc.LastAction != "" {
		parts = append(parts, "last_action="+cc.LastAction)
	}
	if cc.CurrentEntity != "" {
		parts = append(parts, "current_entity="+cc.CurrentEntity)
	}
	return strings.Join(parts, ", ")
}
