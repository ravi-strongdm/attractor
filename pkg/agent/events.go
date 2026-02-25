package agent

// EventType identifies the kind of agent event.
type EventType string

const (
	EventTypeToolCall   EventType = "tool_call"
	EventTypeToolResult EventType = "tool_result"
	EventTypeLLMTurn    EventType = "llm_turn"
	EventTypeComplete   EventType = "complete"
	EventTypeError      EventType = "error"
	EventTypeSteering   EventType = "steering"
)

// Event is emitted by the agent loop for real-time monitoring.
type Event struct {
	Type     EventType `json:"type"`
	Content  string    `json:"content,omitempty"`
	ToolName string    `json:"tool_name,omitempty"`
	IsError  bool      `json:"is_error,omitempty"`
}
