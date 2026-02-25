package providers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	openai "github.com/sashabaranov/go-openai"

	"github.com/ravi-parthasarathy/attractor/pkg/llm"
)

func init() {
	llm.RegisterProvider("openai", func(modelName string) (llm.Client, error) {
		return newOpenAIClient(modelName)
	})
}

type openaiClient struct {
	sdk       *openai.Client
	modelName string
}

func newOpenAIClient(modelName string) (*openaiClient, error) {
	key := os.Getenv("OPENAI_API_KEY")
	if key == "" {
		return nil, fmt.Errorf("openai: OPENAI_API_KEY environment variable not set")
	}
	return &openaiClient{
		sdk:       openai.NewClient(key),
		modelName: modelName,
	}, nil
}

// Complete performs a blocking generation with automatic retry on transient errors.
func (c *openaiClient) Complete(ctx context.Context, req llm.GenerateRequest) (llm.GenerateResponse, error) {
	var resp llm.GenerateResponse
	err := llm.WithRetry(ctx, 4, func() error {
		var innerErr error
		resp, innerErr = c.doComplete(ctx, req)
		return innerErr
	})
	return resp, err
}

func (c *openaiClient) doComplete(ctx context.Context, req llm.GenerateRequest) (llm.GenerateResponse, error) {
	maxTokens := 4096
	if req.MaxTokens > 0 {
		maxTokens = req.MaxTokens
	}

	params := openai.ChatCompletionRequest{
		Model:     c.modelName,
		MaxTokens: maxTokens,
		Messages:  buildMessages(req.Messages, req.System),
	}
	if len(req.Tools) > 0 {
		params.Tools = buildTools(req.Tools)
	}

	resp, err := c.sdk.CreateChatCompletion(ctx, params)
	if err != nil {
		return llm.GenerateResponse{}, mapOpenAIError(err)
	}
	return convertOpenAIResponse(resp), nil
}

// Stream emits text deltas then a final complete event.
// Tool call deltas are not streamed; the final response is obtained via Complete.
func (c *openaiClient) Stream(ctx context.Context, req llm.GenerateRequest) (<-chan llm.StreamEvent, error) {
	ch := make(chan llm.StreamEvent, 64)
	go func() {
		defer close(ch)

		maxTokens := 4096
		if req.MaxTokens > 0 {
			maxTokens = req.MaxTokens
		}
		params := openai.ChatCompletionRequest{
			Model:     c.modelName,
			MaxTokens: maxTokens,
			Messages:  buildMessages(req.Messages, req.System),
		}
		if len(req.Tools) > 0 {
			params.Tools = buildTools(req.Tools)
		}

		stream, err := c.sdk.CreateChatCompletionStream(ctx, params)
		if err != nil {
			return
		}
		defer func() { _ = stream.Close() }()

		var toolCallsPresent bool
		for {
			chunk, err := stream.Recv()
			if err != nil {
				break
			}
			if len(chunk.Choices) == 0 {
				continue
			}
			delta := chunk.Choices[0].Delta
			if delta.Content != "" {
				ch <- llm.StreamEvent{Type: llm.StreamEventDelta, Text: delta.Content}
			}
			if len(delta.ToolCalls) > 0 {
				toolCallsPresent = true
			}
		}

		// If tool calls were present in the stream, re-run as blocking call to
		// get the structured tool call data in convertResponse format.
		if toolCallsPresent {
			resp, err := c.Complete(ctx, req)
			if err != nil {
				return
			}
			ch <- llm.StreamEvent{Type: llm.StreamEventComplete, Response: &resp}
			return
		}

		// Text-only: emit complete event from a non-streaming Complete call
		// to populate usage and stop reason.
		resp, err := c.Complete(ctx, req)
		if err != nil {
			return
		}
		ch <- llm.StreamEvent{Type: llm.StreamEventComplete, Response: &resp}
	}()
	return ch, nil
}

// ─── message conversion ───────────────────────────────────────────────────────

// buildMessages converts unified messages to OpenAI's chat completion format.
//
// Invariant from loop.go: a user message contains EITHER text blocks OR
// tool_result blocks, never both.  Assistant messages may contain text,
// tool_use blocks, or both (mixed).
func buildMessages(msgs []llm.Message, system string) []openai.ChatCompletionMessage {
	var out []openai.ChatCompletionMessage

	if system != "" {
		out = append(out, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleSystem,
			Content: system,
		})
	}

	for _, m := range msgs {
		switch m.Role {
		case llm.RoleSystem:
			// Handled above via req.System; skip any inline system messages.
			continue

		case llm.RoleUser:
			// Check if this is a tool-result message.
			if hasToolResults(m.Content) {
				// One OpenAI "tool" message per tool_result block.
				for _, b := range m.Content {
					if b.Type == llm.ContentTypeToolResult && b.ToolResult != nil {
						out = append(out, openai.ChatCompletionMessage{
							Role:       openai.ChatMessageRoleTool,
							Content:    b.ToolResult.Content,
							ToolCallID: b.ToolResult.ToolUseID,
						})
					}
				}
			} else {
				// Plain text user message.
				out = append(out, openai.ChatCompletionMessage{
					Role:    openai.ChatMessageRoleUser,
					Content: concatText(m.Content),
				})
			}

		case llm.RoleAssistant:
			msg := openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant}
			for _, b := range m.Content {
				switch b.Type {
				case llm.ContentTypeText:
					msg.Content += b.Text
				case llm.ContentTypeToolUse:
					if b.ToolUse != nil {
						msg.ToolCalls = append(msg.ToolCalls, openai.ToolCall{
							ID:   b.ToolUse.ID,
							Type: openai.ToolTypeFunction,
							Function: openai.FunctionCall{
								Name:      b.ToolUse.Name,
								Arguments: string(b.ToolUse.Input),
							},
						})
					}
				}
			}
			out = append(out, msg)
		}
	}
	return out
}

// buildTools converts unified tool definitions to OpenAI's tool format.
func buildTools(defs []llm.ToolDefinition) []openai.Tool {
	tools := make([]openai.Tool, 0, len(defs))
	for _, d := range defs {
		var params any
		if len(d.InputSchema) > 0 {
			// Pass the raw JSON schema bytes; go-openai accepts json.RawMessage.
			params = json.RawMessage(d.InputSchema)
		}
		tools = append(tools, openai.Tool{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        d.Name,
				Description: d.Description,
				Parameters:  params,
			},
		})
	}
	return tools
}

// convertOpenAIResponse maps an OpenAI response to the unified GenerateResponse.
func convertOpenAIResponse(resp openai.ChatCompletionResponse) llm.GenerateResponse {
	var blocks []llm.ContentBlock
	if len(resp.Choices) > 0 {
		msg := resp.Choices[0].Message

		if msg.Content != "" {
			blocks = append(blocks, llm.ContentBlock{
				Type: llm.ContentTypeText,
				Text: msg.Content,
			})
		}

		for _, tc := range msg.ToolCalls {
			blocks = append(blocks, llm.ContentBlock{
				Type: llm.ContentTypeToolUse,
				ToolUse: &llm.ToolUse{
					ID:    tc.ID,
					Name:  tc.Function.Name,
					Input: []byte(tc.Function.Arguments),
				},
			})
		}
	}

	stop := llm.StopReasonEndTurn
	if len(resp.Choices) > 0 {
		switch resp.Choices[0].FinishReason {
		case openai.FinishReasonToolCalls:
			stop = llm.StopReasonToolUse
		case openai.FinishReasonLength:
			stop = llm.StopReasonMaxTokens
		}
	}

	return llm.GenerateResponse{
		Content:    blocks,
		StopReason: stop,
		Usage: llm.Usage{
			InputTokens:  resp.Usage.PromptTokens,
			OutputTokens: resp.Usage.CompletionTokens,
		},
	}
}

// ─── error mapping ────────────────────────────────────────────────────────────

func mapOpenAIError(err error) error {
	if err == nil {
		return nil
	}
	var apiErr *openai.APIError
	if errors.As(err, &apiErr) {
		base := llm.LLMError{
			Code:    apiErr.HTTPStatusCode,
			Message: apiErr.Message,
			Cause:   err,
		}
		switch apiErr.HTTPStatusCode {
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
	return fmt.Errorf("openai: %w", err)
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func hasToolResults(blocks []llm.ContentBlock) bool {
	for _, b := range blocks {
		if b.Type == llm.ContentTypeToolResult {
			return true
		}
	}
	return false
}

func concatText(blocks []llm.ContentBlock) string {
	var s string
	for _, b := range blocks {
		if b.Type == llm.ContentTypeText {
			s += b.Text
		}
	}
	return s
}
