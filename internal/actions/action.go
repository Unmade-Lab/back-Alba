package actions

import (
	"context"
	"fmt"

	"github.com/Unmade-Lab/back-Alba/internal/convctx"
)

// Result is the structured output returned by every Action.
type Result struct {
	Data    interface{} `json:"data"`
	Message string      `json:"message"`
}

// Action defines the contract all CRM action handlers must satisfy.
type Action interface {
	// Name returns the unique identifier used to look up this action.
	Name() string
	// Description is shown to the LLM so it understands when to call this action.
	Description() string
	// Schema returns a JSON Schema object describing the parameters.
	Schema() map[string]interface{}
	// Execute runs the business logic for the action.
	Execute(ctx context.Context, convCtx *convctx.ConversationContext, params map[string]interface{}) (Result, error)
}

// Registry holds all registered actions keyed by name.
type Registry struct {
	actions map[string]Action
}

// NewRegistry creates an empty Registry.
func NewRegistry() *Registry {
	return &Registry{actions: make(map[string]Action)}
}

// Register adds an action to the registry. Panics on duplicate names.
func (r *Registry) Register(a Action) {
	if _, exists := r.actions[a.Name()]; exists {
		panic(fmt.Sprintf("action %q already registered", a.Name()))
	}
	r.actions[a.Name()] = a
}

// Get retrieves an action by name; returns false if not found.
func (r *Registry) Get(name string) (Action, bool) {
	a, ok := r.actions[name]
	return a, ok
}

// All returns every registered action (used by the orchestrator to build tool lists).
func (r *Registry) All() []Action {
	out := make([]Action, 0, len(r.actions))
	for _, a := range r.actions {
		out = append(out, a)
	}
	return out
}
