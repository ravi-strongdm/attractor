package providers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"

	"github.com/ravi-parthasarathy/attractor/pkg/llm"
)

func init() {
	llm.RegisterProvider("gemini", func(modelName string) (llm.Client, error) {
		return newGeminiClient(modelName)
	})
}

type geminiClient struct {
	sdk       *genai.Client
	modelName string
}

func newGeminiClient(modelName string) (*geminiClient, error) {
	key := os.Getenv("GEMINI_API_KEY")
	if key == "" {
		return nil, fmt.Errorf("gemini: GEMINI_API_KEY environment variable not set")
	}
	// genai.NewClient requires a context; use Background for construction.
	sdk, err := genai.NewClient(context.Background(), option.WithAPIKey(key))
	if err != nil {
		return nil, fmt.Errorf("gemini: create client: %w", err)
	}
	return &geminiClient{sdk: sdk, modelName: modelName}, nil
}

// Complete performs a blocking generation with automatic retry on transient errors.
func (c *geminiClient) Complete(ctx context.Context, req llm.GenerateRequest) (llm.GenerateResponse, error) {
	var resp llm.GenerateResponse
	err := llm.WithRetry(ctx, 4, func() error {
		var innerErr error
		resp, innerErr = c.doComplete(ctx, req)
		return innerErr
	})
	return resp, err
}

func (c *geminiClient) doComplete(ctx context.Context, req llm.GenerateRequest) (llm.GenerateResponse, error) {
	model := c.sdk.GenerativeModel(c.modelName)

	if req.MaxTokens > 0 {
		n := int32(req.MaxTokens)
		model.MaxOutputTokens = &n
	}

	// System prompt goes to SystemInstruction, not the message history.
	if req.System != "" {
		model.SystemInstruction = &genai.Content{
			Parts: []genai.Part{genai.Text(req.System)},
		}
	}

	// Tools
	if len(req.Tools) > 0 {
		model.Tools = buildGeminiTools(req.Tools)
	}

	// Split history (all messages except last) from the final user message.
	history, lastContent, err := buildContents(req.Messages)
	if err != nil {
		return llm.GenerateResponse{}, fmt.Errorf("gemini: build contents: %w", err)
	}

	cs := model.StartChat()
	cs.History = history

	if lastContent == nil {
		return llm.GenerateResponse{}, fmt.Errorf("gemini: no user message to send")
	}

	apiResp, err := cs.SendMessage(ctx, lastContent.Parts...)
	if err != nil {
		return llm.GenerateResponse{}, mapGeminiError(err)
	}
	return convertGeminiResponse(apiResp), nil
}

// Stream emits text deltas then a final complete event.
func (c *geminiClient) Stream(ctx context.Context, req llm.GenerateRequest) (<-chan llm.StreamEvent, error) {
	ch := make(chan llm.StreamEvent, 64)
	go func() {
		defer close(ch)
		resp, err := c.doComplete(ctx, req)
		if err != nil {
			ch <- llm.StreamEvent{Type: llm.StreamEventComplete, Response: &llm.GenerateResponse{}}
			return
		}
		// Emit text deltas first.
		for _, b := range resp.Content {
			if b.Type == llm.ContentTypeText && b.Text != "" {
				ch <- llm.StreamEvent{Type: llm.StreamEventDelta, Text: b.Text}
			}
		}
		ch <- llm.StreamEvent{Type: llm.StreamEventComplete, Response: &resp}
	}()
	return ch, nil
}

// ─── message translation ─────────────────────────────────────────────────────

// buildContents translates unified messages into Gemini's format.
// Returns (history, lastUserContent, error).
// History contains all messages except the last one; the last user message
// is returned separately for use with cs.SendMessage().
func buildContents(msgs []llm.Message) ([]*genai.Content, *genai.Content, error) {
	var contents []*genai.Content
	for _, m := range msgs {
		if m.Role == llm.RoleSystem {
			continue // handled via model.SystemInstruction
		}
		c, err := messageToContent(m, msgs)
		if err != nil {
			return nil, nil, err
		}
		if c != nil {
			contents = append(contents, c)
		}
	}

	if len(contents) == 0 {
		return nil, nil, nil
	}

	// The last content is sent via SendMessage; everything before it is history.
	last := contents[len(contents)-1]
	history := contents[:len(contents)-1]
	return history, last, nil
}

// messageToContent converts a single unified Message to a *genai.Content.
// msgs is the full message slice (needed for tool_result name resolution).
func messageToContent(m llm.Message, allMsgs []llm.Message) (*genai.Content, error) {
	switch m.Role {
	case llm.RoleUser:
		return userContent(m, allMsgs)
	case llm.RoleAssistant:
		return assistantContent(m)
	default:
		return nil, nil
	}
}

func userContent(m llm.Message, allMsgs []llm.Message) (*genai.Content, error) {
	// Determine if this is a tool-result message or a plain text message.
	if hasToolResults(m.Content) {
		return toolResultContent(m, allMsgs)
	}
	// Plain text user message.
	text := concatText(m.Content)
	return &genai.Content{
		Role:  "user",
		Parts: []genai.Part{genai.Text(text)},
	}, nil
}

func toolResultContent(m llm.Message, allMsgs []llm.Message) (*genai.Content, error) {
	parts := make([]genai.Part, 0, len(m.Content))
	for _, b := range m.Content {
		if b.Type != llm.ContentTypeToolResult || b.ToolResult == nil {
			continue
		}
		// Gemini FunctionResponse requires the function name, not the call ID.
		name, ok := resolveToolName(b.ToolResult.ToolUseID, allMsgs)
		if !ok {
			// Fall back to the ID itself — better than failing.
			name = b.ToolResult.ToolUseID
		}
		parts = append(parts, genai.FunctionResponse{
			Name:     name,
			Response: map[string]any{"result": b.ToolResult.Content},
		})
	}
	if len(parts) == 0 {
		return nil, nil
	}
	return &genai.Content{Role: "user", Parts: parts}, nil
}

func assistantContent(m llm.Message) (*genai.Content, error) {
	var parts []genai.Part
	for _, b := range m.Content {
		switch b.Type {
		case llm.ContentTypeText:
			if b.Text != "" {
				parts = append(parts, genai.Text(b.Text))
			}
		case llm.ContentTypeToolUse:
			if b.ToolUse != nil {
				var args map[string]any
				if len(b.ToolUse.Input) > 0 {
					if err := json.Unmarshal(b.ToolUse.Input, &args); err != nil {
						return nil, fmt.Errorf("tool_use %q: unmarshal input: %w", b.ToolUse.Name, err)
					}
				}
				parts = append(parts, genai.FunctionCall{
					Name: b.ToolUse.Name,
					Args: args,
				})
			}
		}
	}
	if len(parts) == 0 {
		return nil, nil
	}
	return &genai.Content{Role: "model", Parts: parts}, nil
}

// resolveToolName scans all messages backward to find the ToolUse with
// the given ID and returns its Name.
func resolveToolName(toolUseID string, allMsgs []llm.Message) (string, bool) {
	for i := len(allMsgs) - 1; i >= 0; i-- {
		for _, b := range allMsgs[i].Content {
			if b.Type == llm.ContentTypeToolUse && b.ToolUse != nil && b.ToolUse.ID == toolUseID {
				return b.ToolUse.Name, true
			}
		}
	}
	return "", false
}

// ─── tool definition translation ─────────────────────────────────────────────

func buildGeminiTools(defs []llm.ToolDefinition) []*genai.Tool {
	decls := make([]*genai.FunctionDeclaration, 0, len(defs))
	for _, d := range defs {
		fd := &genai.FunctionDeclaration{
			Name:        d.Name,
			Description: d.Description,
		}
		if len(d.InputSchema) > 0 {
			schema, err := jsonSchemaToGenai(d.InputSchema)
			if err == nil && schema != nil {
				fd.Parameters = schema
			}
		}
		decls = append(decls, fd)
	}
	return []*genai.Tool{{FunctionDeclarations: decls}}
}

// jsonSchemaToGenai converts a JSON Schema (as raw bytes) to a *genai.Schema.
// It handles the common cases: object, string, integer, number, boolean, array.
func jsonSchemaToGenai(raw []byte) (*genai.Schema, error) {
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("jsonSchemaToGenai: %w", err)
	}
	return mapToGenaiSchema(m), nil
}

func mapToGenaiSchema(m map[string]any) *genai.Schema {
	if m == nil {
		return nil
	}
	s := &genai.Schema{}

	if t, ok := m["type"].(string); ok {
		switch t {
		case "object":
			s.Type = genai.TypeObject
		case "string":
			s.Type = genai.TypeString
		case "integer":
			s.Type = genai.TypeInteger
		case "number":
			s.Type = genai.TypeNumber
		case "boolean":
			s.Type = genai.TypeBoolean
		case "array":
			s.Type = genai.TypeArray
		default:
			s.Type = genai.TypeUnspecified
		}
	}

	if d, ok := m["description"].(string); ok {
		s.Description = d
	}

	if props, ok := m["properties"].(map[string]any); ok {
		s.Properties = make(map[string]*genai.Schema, len(props))
		for k, v := range props {
			if vm, ok := v.(map[string]any); ok {
				s.Properties[k] = mapToGenaiSchema(vm)
			}
		}
	}

	if req, ok := m["required"].([]any); ok {
		for _, r := range req {
			if rs, ok := r.(string); ok {
				s.Required = append(s.Required, rs)
			}
		}
	}

	if items, ok := m["items"].(map[string]any); ok {
		s.Items = mapToGenaiSchema(items)
	}

	if enum, ok := m["enum"].([]any); ok {
		for _, e := range enum {
			if es, ok := e.(string); ok {
				s.Enum = append(s.Enum, es)
			}
		}
	}

	return s
}

// ─── response conversion ─────────────────────────────────────────────────────

func convertGeminiResponse(resp *genai.GenerateContentResponse) llm.GenerateResponse {
	var blocks []llm.ContentBlock
	stopReason := llm.StopReasonEndTurn

	if len(resp.Candidates) > 0 {
		cand := resp.Candidates[0]

		if cand.Content != nil {
			for _, part := range cand.Content.Parts {
				switch v := part.(type) {
				case genai.Text:
					if string(v) != "" {
						blocks = append(blocks, llm.ContentBlock{
							Type: llm.ContentTypeText,
							Text: string(v),
						})
					}
				case genai.FunctionCall:
					inputJSON, _ := json.Marshal(v.Args)
					blocks = append(blocks, llm.ContentBlock{
						Type: llm.ContentTypeToolUse,
						ToolUse: &llm.ToolUse{
							ID:    v.Name, // Gemini doesn't give a unique call ID — use name
							Name:  v.Name,
							Input: inputJSON,
						},
					})
				}
			}
		}

		// Detect tool use from content first — Gemini returns FinishReasonStop
		// even when the response contains function calls.
		hasToolUse := false
		for _, b := range blocks {
			if b.Type == llm.ContentTypeToolUse {
				hasToolUse = true
				break
			}
		}
		switch {
		case hasToolUse:
			stopReason = llm.StopReasonToolUse
		case cand.FinishReason == genai.FinishReasonMaxTokens:
			stopReason = llm.StopReasonMaxTokens
		default:
			stopReason = llm.StopReasonEndTurn
		}
	}

	var usage llm.Usage
	if resp.UsageMetadata != nil {
		usage.InputTokens = int(resp.UsageMetadata.PromptTokenCount)
		usage.OutputTokens = int(resp.UsageMetadata.CandidatesTokenCount)
	}

	return llm.GenerateResponse{
		Content:    blocks,
		StopReason: stopReason,
		Usage:      usage,
	}
}

// ─── error mapping ────────────────────────────────────────────────────────────

func mapGeminiError(err error) error {
	if err == nil {
		return nil
	}
	var apiErr *googleapi.Error
	if errors.As(err, &apiErr) {
		base := llm.LLMError{
			Code:    apiErr.Code,
			Message: apiErr.Message,
			Cause:   err,
		}
		switch apiErr.Code {
		case 429:
			return &llm.RateLimitError{LLMError: base}
		case 401, 403:
			return &llm.AuthError{LLMError: base}
		case 400:
			return &llm.ContextLengthError{LLMError: base}
		case 500, 502, 503:
			return &llm.ServerError{LLMError: base}
		default:
			return &base
		}
	}
	return fmt.Errorf("gemini: %w", err)
}
