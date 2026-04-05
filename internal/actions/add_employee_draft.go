package actions

import (
	"context"
	"fmt"

	"github.com/Unmade-Lab/back-Alba/internal/convctx"
	"github.com/Unmade-Lab/back-Alba/internal/models"
	"github.com/google/uuid"
)

// AddEmployeeDraftAction handles appending an employee to a drafted company widget.
type AddEmployeeDraftAction struct{}

func NewAddEmployeeDraftAction() *AddEmployeeDraftAction {
	return &AddEmployeeDraftAction{}
}

func (a *AddEmployeeDraftAction) Name() string { return "add_employee_to_draft" }
func (a *AddEmployeeDraftAction) Description() string {
	return "Add a new employee and optionally their department to the currently drafted company. Use when the user wants to append an employee to the company form before submitting."
}

func (a *AddEmployeeDraftAction) Schema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"name": map[string]interface{}{
				"type":        "string",
				"description": "Employee's full name.",
			},
			"department": map[string]interface{}{
				"type":        "string",
				"description": "Department name (e.g., 'Sales', 'IT').",
			},
		},
		"required": []string{"name"},
	}
}

func (a *AddEmployeeDraftAction) Execute(ctx context.Context, convCtx *convctx.ConversationContext, params map[string]interface{}) (Result, error) {
	if convCtx.Draft == nil || convCtx.Draft.ActionName != "create_company" {
		return Result{}, fmt.Errorf("there is no active company draft to add an employee to")
	}

	name, ok := params["name"].(string)
	if !ok || name == "" {
		return Result{}, fmt.Errorf("'name' is required")
	}

	department, _ := params["department"].(string)

	employee := models.Employee{
		ID:         uuid.New(),
		Name:       name,
		Department: department,
	}

	// Retrieve company from draft payload
	companyAny := convCtx.Draft.Payload["company"]
	var company *models.Company
	company, ok = companyAny.(*models.Company)
	if !ok {
		return Result{}, fmt.Errorf("draft payload does not contain a valid company object")
	}

	// Append employee
	company.Employees = append(company.Employees, employee)
	convCtx.Draft.Payload["company"] = company // re-assign

	msg := fmt.Sprintf("Added employee '%s' ", name)
	if department != "" {
		msg += fmt.Sprintf("to the '%s' department ", department)
	}
	msg += "in the company draft."

	return Result{
		Data:    company,
		Message: msg,
	}, nil
}
