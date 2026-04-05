package actions

import (
	"context"
	"fmt"

	"github.com/Unmade-Lab/back-Alba/internal/convctx"
)

// InviteUserDraftAction handles the AI intent "invite_user".
// It prepares an interactive draft (widget) where the admin can enter the employee's email.
type InviteUserDraftAction struct{}

func NewInviteUserDraftAction() *InviteUserDraftAction {
	return &InviteUserDraftAction{}
}

func (a *InviteUserDraftAction) Name() string {
	return "invite_user"
}

func (a *InviteUserDraftAction) Description() string {
	return "Initializes an interactive form to invite a new employee (user) to the CRM platform."
}

func (a *InviteUserDraftAction) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{
				"type":        "string",
				"description": "Full name of the employee to invite (e.g. 'Ivan Ivanov').",
			},
			"role": map[string]interface{}{
				"type":        "string",
				"description": "Job title or role of the employee (e.g. 'engineer', 'manager').",
			},
			"email": map[string]interface{}{
				"type":        "string",
				"description": "Email address of the employee, if provided by the user.",
			},
		},
		"required": []string{"name"},
	}
}

func (a *InviteUserDraftAction) Execute(ctx context.Context, cc *convctx.ConversationContext, params map[string]interface{}) (Result, error) {
	name, _ := params["name"].(string)
	role, _ := params["role"].(string)
	if role == "" {
		role = "user"
	}
	email, _ := params["email"].(string)

	draft := map[string]interface{}{
		"name":  name,
		"role":  role,
		"email": email, // Can be empty, to be filled by the admin in the frontend
	}

	cc.Draft = &convctx.DraftData{
		ActionName: "invite_user",
		Payload:    draft,
	}

	return Result{
		Message: fmt.Sprintf("Подготавливаю приглашение для сотрудника %s. Пожалуйста, укажите его email для создания ссылки.", name),
		Data:    draft,
	}, nil
}
