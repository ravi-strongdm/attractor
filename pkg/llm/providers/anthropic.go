// Package providers registers LLM provider adapters.
// Import this package with a blank identifier to activate all providers:
//
//	import _ "github.com/ravi-parthasarathy/attractor/pkg/llm/providers"
package providers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	anthropicsdk "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/anthropics/anthropic-sdk-go/packages/param"
	"github.com/ravi-parthasarathy/attractor/pkg/llm"
)

func init() {
	llm.RegisterProvider("anthropic", func(modelName string) (llm.Client, error) {
		return newAnthropicClient(modelName)
	})
}

type anthropicClient struct {
	sdk       anthropicsdk.Client
	modelName string
}

func newAnthropicClient(modelName string) (*anthropicClient, error) {
	sdk := anthropicsdk.NewClient(option.WithAPIKey("")) // reads ANTHROPIC_API_KEY automatically
	return &anthropicClient{sdk: sdk, modelName: modelName}, nil
}

// Complete performs a blocking generation with automatic retry on transient errors.
func (a *anthropicClient) Complete(ctx context.Context, req llm.GenerateRequest) (llm.GenerateResponse, error) {
	var resp llm.GenerateResponse
	err := llm.WithRetry(ctx, 4, func() error {
		var innerErr error
		resp, innerErr = a.doComplete(ctx, req)
		return innerErr
	})
	return resp, err
}

func (a *anthropicClient) doComplete(ctx context.Context, req llm.GenerateRequest) (llm.GenerateResponse, error) {
	// Convert messages (skip system role — handled via System param)
	msgs := make([]anthropicsdk.MessageParam, 0, len(req.Messages))
	for _, m := range req.Messages {
		if m.Role == llm.RoleSystem {
			continue
		}
		blocks := make([]anthropicsdk.ContentBlockParamUnion, 0, len(m.Content))
		for _, b := range m.Content {
			switch b.Type {
			case llm.ContentTypeText:
				blocks = append(blocks, anthropicsdk.NewTextBlock(b.Text))
			case llm.ContentTypeToolResult:
				if b.ToolResult != nil {
					blocks = append(blocks, anthropicsdk.NewToolResultBlock(
						b.ToolResult.ToolUseID,
						b.ToolResult.Content,
						b.ToolResult.IsError,
					))
				}
			case llm.ContentTypeToolUse:
				if b.ToolUse != nil {
					var input any
					_ = json.Unmarshal(b.ToolUse.Input, &input)
					blocks = append(blocks, anthropicsdk.NewToolUseBlock(b.ToolUse.ID, input, b.ToolUse.Name))
				}
			}
		}
		switch m.Role {
		case llm.RoleUser:
			msgs = append(msgs, anthropicsdk.NewUserMessage(blocks...))
		case llm.RoleAssistant:
			msgs = append(msgs, anthropicsdk.NewAssistantMessage(blocks...))
		}
	}

	// Convert tool definitions
	tools := make([]anthropicsdk.ToolUnionParam, 0, len(req.Tools))
	for _, t := range req.Tools {
		schema := buildInputSchema(t.InputSchema)
		tp := anthropicsdk.ToolParam{
			Name:        t.Name,
			InputSchema: schema,
			Description: param.NewOpt(t.Description),
		}
		tools = append(tools, anthropicsdk.ToolUnionParam{OfTool: &tp})
	}

	maxTokens := int64(4096)
	if req.MaxTokens > 0 {
		maxTokens = int64(req.MaxTokens)
	}

	params := anthropicsdk.MessageNewParams{
		Model:     anthropicsdk.Model(a.modelName),
		MaxTokens: maxTokens,
		Messages:  msgs,
	}
	if req.System != "" {
		params.System = []anthropicsdk.TextBlockParam{{Text: req.System}}
	}
	if len(tools) > 0 {
		params.Tools = tools
	}

	msg, err := a.sdk.Messages.New(ctx, params)
	if err != nil {
		return llm.GenerateResponse{}, mapError(err)
	}
	return convertResponse(msg), nil
}

// Stream sends events over a channel. The channel is closed when done.
// For simplicity, this implementation calls Complete and emits the result as a stream.
func (a *anthropicClient) Stream(ctx context.Context, req llm.GenerateRequest) (<-chan llm.StreamEvent, error) {
	ch := make(chan llm.StreamEvent, 64)
	go func() {
		defer close(ch)
		resp, err := a.Complete(ctx, req)
		if err != nil {
			return
		}
		for _, b := range resp.Content {
			if b.Type == llm.ContentTypeText && b.Text != "" {
				ch <- llm.StreamEvent{Type: llm.StreamEventDelta, Text: b.Text}
			}
		}
		ch <- llm.StreamEvent{Type: llm.StreamEventComplete, Response: &resp}
	}()
	return ch, nil
}

// buildInputSchema converts raw JSON Schema bytes into a ToolInputSchemaParam.
func buildInputSchema(raw []byte) anthropicsdk.ToolInputSchemaParam {
	schema := anthropicsdk.ToolInputSchemaParam{}
	if len(raw) == 0 {
		return schema
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return schema
	}
	if props, ok := m["properties"]; ok {
		schema.Properties = props
	}
	if req, ok := m["required"]; ok {
		if reqSlice, ok2 := req.([]any); ok2 {
			strs := make([]string, 0, len(reqSlice))
			for _, r := range reqSlice {
				if s, ok := r.(string); ok {
					strs = append(strs, s)
				}
			}
			schema.Required = strs
		}
	}
	return schema
}

func convertResponse(msg *anthropicsdk.Message) llm.GenerateResponse {
	blocks := make([]llm.ContentBlock, 0, len(msg.Content))
	for _, b := range msg.Content {
		switch b.Type {
		case "text":
			blocks = append(blocks, llm.ContentBlock{
				Type: llm.ContentTypeText,
				Text: b.Text,
			})
		case "tool_use":
			raw, _ := json.Marshal(b.Input)
			blocks = append(blocks, llm.ContentBlock{
				Type: llm.ContentTypeToolUse,
				ToolUse: &llm.ToolUse{
					ID:    b.ID,
					Name:  b.Name,
					Input: raw,
				},
			})
		}
	}

	stop := llm.StopReasonEndTurn
	switch msg.StopReason {
	case anthropicsdk.StopReasonToolUse:
		stop = llm.StopReasonToolUse
	case anthropicsdk.StopReasonMaxTokens:
		stop = llm.StopReasonMaxTokens
	}

	return llm.GenerateResponse{
		Content:    blocks,
		StopReason: stop,
		Usage: llm.Usage{
			InputTokens:  int(msg.Usage.InputTokens),
			OutputTokens: int(msg.Usage.OutputTokens),
		},
	}
}

func mapError(err error) error {
	if err == nil {
		return nil
	}
	var apiErr *anthropicsdk.Error
	if errors.As(err, &apiErr) {
		base := llm.LLMError{Code: apiErr.StatusCode, Message: apiErr.Error(), Cause: err}
		switch apiErr.StatusCode {
		case 429:
			return &llm.RateLimitError{LLMError: base}
		case 401, 403:
			return &llm.AuthError{LLMError: base}
		case 400:
			return &llm.ContextLengthError{LLMError: base}
		case 500, 502, 503, 529:
			return &llm.ServerError{LLMError: base}
		}
	}
	return fmt.Errorf("anthropic: %w", err)
}
