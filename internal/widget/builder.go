package widget

// Builder maps action names to their widget type and constructs Widget values.
type Builder struct{}

// NewBuilder creates a Builder.
func NewBuilder() *Builder { return &Builder{} }

// actionWidgetMap maps an action name to the widget type it should produce.
var actionWidgetMap = map[string]string{
	"create_deal":        TypeDealCard,
	"create_company":     TypeCompanyCard,
	"get_deals":          TypeDealsList,
	"update_deal_stage":  TypeDealCard,
}

// Build creates a Widget for the given action and result data.
// Falls back to TypeTextOnly when there is no specific widget for the action.
func (b *Builder) Build(actionName string, data interface{}) *Widget {
	widgetType, ok := actionWidgetMap[actionName]
	if !ok {
		widgetType = TypeTextOnly
	}
	return &Widget{
		Type: widgetType,
		Data: data,
	}
}

// Error creates an error widget for surfacing structured failures to the frontend.
func (b *Builder) Error(message string) *Widget {
	return &Widget{
		Type: TypeError,
		Data: map[string]string{"message": message},
	}
}
