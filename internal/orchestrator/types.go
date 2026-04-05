package orchestrator

// Intent is the structured output parsed from an LLM function call.
type Intent struct {
	Name   string                 `json:"intent"`
	Params map[string]interface{} `json:"params"`
}

// HistoryMessage is a condensed chat turn passed to the LLM as conversation context.
type HistoryMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}
