package widget

// Type constants for all supported UI widget types.
const (
	TypeDealCard    = "deal_card"
	TypeCompanyCard = "company_card"
	TypeDealsList   = "deals_list"
	TypeTaskCard    = "task_card"
	TypeTextOnly    = "text_only"
	TypeError       = "error"
)

// Widget is the JSON payload sent to the frontend to render a UI component.
type Widget struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}
