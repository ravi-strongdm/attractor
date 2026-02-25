package llm

import "fmt"

// Role represents the sender of a message.
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleSystem    Role = "system"
)

// ContentType identifies what kind of content a block holds.
type ContentType string

const (
	ContentTypeText       ContentType = "text"
	ContentTypeImage      ContentType = "image"
	ContentTypeToolUse    ContentType = "tool_use"
	ContentTypeToolResult ContentType = "tool_result"
)

// ContentBlock is one element in a message's content array.
type ContentBlock struct {
	Type       ContentType `json:"type"`
	Text       string      `json:"text,omitempty"`
	ToolUse    *ToolUse    `json:"tool_use,omitempty"`
	ToolResult *ToolResult `json:"tool_result,omitempty"`
}

// ToolUse represents a model's request to call a tool.
type ToolUse struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Input []byte `json:"input"` // raw JSON
}

// ToolResult is the response to a ToolUse call.
type ToolResult struct {
	ToolUseID string `json:"tool_use_id"`
	Content   string `json:"content"`
	IsError   bool   `json:"is_error"`
}

// Message is one turn in a conversation.
type Message struct {
	Role    Role           `json:"role"`
	Content []ContentBlock `json:"content"`
}

// TextMessage is a convenience constructor for a plain-text message.
func TextMessage(role Role, text string) Message {
	return Message{
		Role:    role,
		Content: []ContentBlock{{Type: ContentTypeText, Text: text}},
	}
}

// ToolDefinition describes a tool the model may call.
type ToolDefinition struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	InputSchema []byte `json:"input_schema"` // JSON Schema object bytes
}

// GenerateRequest is the unified input to the LLM client.
type GenerateRequest struct {
	Model     string           `json:"model"`
	Messages  []Message        `json:"messages"`
	Tools     []ToolDefinition `json:"tools,omitempty"`
	System    string           `json:"system,omitempty"`
	MaxTokens int              `json:"max_tokens,omitempty"`
}

// StopReason explains why generation stopped.
type StopReason string

const (
	StopReasonEndTurn   StopReason = "end_turn"
	StopReasonToolUse   StopReason = "tool_use"
	StopReasonMaxTokens StopReason = "max_tokens"
)

// Usage reports token counts.
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// GenerateResponse is the unified output from the LLM client.
type GenerateResponse struct {
	Content    []ContentBlock `json:"content"`
	StopReason StopReason     `json:"stop_reason"`
	Usage      Usage          `json:"usage"`
}

// StreamEventType identifies a streaming event.
type StreamEventType string

const (
	StreamEventDelta    StreamEventType = "delta"
	StreamEventToolUse  StreamEventType = "tool_use"
	StreamEventComplete StreamEventType = "complete"
)

// StreamEvent is one chunk emitted during streaming generation.
type StreamEvent struct {
	Type     StreamEventType   `json:"type"`
	Text     string            `json:"text,omitempty"`
	ToolUse  *ToolUse          `json:"tool_use,omitempty"`
	Response *GenerateResponse `json:"response,omitempty"`
}

// ParseModelID splits "provider:model-name" into (provider, modelName, nil).
// Both parts must be non-empty and the colon separator is required.
// Returns an error if the format is invalid.
func ParseModelID(id string) (provider, modelName string, err error) {
	for i, c := range id {
		if c == ':' {
			p := id[:i]
			m := id[i+1:]
			if p == "" {
				return "", "", fmt.Errorf("model ID %q: empty provider name", id)
			}
			if m == "" {
				return "", "", fmt.Errorf("model ID %q: empty model name", id)
			}
			return p, m, nil
		}
	}
	return "", "", fmt.Errorf("model ID %q: missing 'provider:model-name' format", id)
}
