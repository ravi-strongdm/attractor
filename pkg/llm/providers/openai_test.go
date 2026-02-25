package providers

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	openai "github.com/sashabaranov/go-openai"

	"github.com/ravi-parthasarathy/attractor/pkg/llm"
)

// ─── TestBuildMessages ────────────────────────────────────────────────────────

func TestBuildMessages_UserText(t *testing.T) {
	msgs := []llm.Message{
		llm.TextMessage(llm.RoleUser, "hello"),
	}
	out := buildMessages(msgs, "")
	if len(out) != 1 {
		t.Fatalf("want 1 message, got %d", len(out))
	}
	if out[0].Role != openai.ChatMessageRoleUser {
		t.Errorf("role: want %q, got %q", openai.ChatMessageRoleUser, out[0].Role)
	}
	if out[0].Content != "hello" {
		t.Errorf("content: want %q, got %q", "hello", out[0].Content)
	}
}

func TestBuildMessages_SystemPrepend(t *testing.T) {
	msgs := []llm.Message{
		llm.TextMessage(llm.RoleUser, "hi"),
	}
	out := buildMessages(msgs, "you are helpful")
	if len(out) != 2 {
		t.Fatalf("want 2 messages, got %d", len(out))
	}
	if out[0].Role != openai.ChatMessageRoleSystem {
		t.Errorf("first role: want system, got %q", out[0].Role)
	}
	if out[0].Content != "you are helpful" {
		t.Errorf("system content: want %q, got %q", "you are helpful", out[0].Content)
	}
	if out[1].Role != openai.ChatMessageRoleUser {
		t.Errorf("second role: want user, got %q", out[1].Role)
	}
}

func TestBuildMessages_ToolResults(t *testing.T) {
	msgs := []llm.Message{
		{
			Role: llm.RoleUser,
			Content: []llm.ContentBlock{
				{
					Type: llm.ContentTypeToolResult,
					ToolResult: &llm.ToolResult{
						ToolUseID: "call_abc",
						Content:   "file contents",
					},
				},
				{
					Type: llm.ContentTypeToolResult,
					ToolResult: &llm.ToolResult{
						ToolUseID: "call_def",
						Content:   "other result",
					},
				},
			},
		},
	}
	out := buildMessages(msgs, "")
	if len(out) != 2 {
		t.Fatalf("want 2 tool messages, got %d", len(out))
	}
	for i, tc := range []struct{ id, content string }{
		{"call_abc", "file contents"},
		{"call_def", "other result"},
	} {
		if out[i].Role != openai.ChatMessageRoleTool {
			t.Errorf("msg[%d] role: want tool, got %q", i, out[i].Role)
		}
		if out[i].ToolCallID != tc.id {
			t.Errorf("msg[%d] ToolCallID: want %q, got %q", i, tc.id, out[i].ToolCallID)
		}
		if out[i].Content != tc.content {
			t.Errorf("msg[%d] content: want %q, got %q", i, tc.content, out[i].Content)
		}
	}
}

func TestBuildMessages_AssistantToolUse(t *testing.T) {
	msgs := []llm.Message{
		{
			Role: llm.RoleAssistant,
			Content: []llm.ContentBlock{
				{
					Type: llm.ContentTypeToolUse,
					ToolUse: &llm.ToolUse{
						ID:    "call_1",
						Name:  "read_file",
						Input: []byte(`{"path":"foo.txt"}`),
					},
				},
			},
		},
	}
	out := buildMessages(msgs, "")
	if len(out) != 1 {
		t.Fatalf("want 1 message, got %d", len(out))
	}
	msg := out[0]
	if msg.Role != openai.ChatMessageRoleAssistant {
		t.Errorf("role: want assistant, got %q", msg.Role)
	}
	if msg.Content != "" {
		t.Errorf("content should be empty for tool-only message, got %q", msg.Content)
	}
	if len(msg.ToolCalls) != 1 {
		t.Fatalf("want 1 tool call, got %d", len(msg.ToolCalls))
	}
	tc := msg.ToolCalls[0]
	if tc.ID != "call_1" {
		t.Errorf("ToolCall.ID: want %q, got %q", "call_1", tc.ID)
	}
	if tc.Function.Name != "read_file" {
		t.Errorf("ToolCall.Function.Name: want %q, got %q", "read_file", tc.Function.Name)
	}
	if tc.Function.Arguments != `{"path":"foo.txt"}` {
		t.Errorf("ToolCall.Function.Arguments: want %q, got %q", `{"path":"foo.txt"}`, tc.Function.Arguments)
	}
}

func TestBuildMessages_AssistantMixed(t *testing.T) {
	msgs := []llm.Message{
		{
			Role: llm.RoleAssistant,
			Content: []llm.ContentBlock{
				{Type: llm.ContentTypeText, Text: "Let me check."},
				{
					Type: llm.ContentTypeToolUse,
					ToolUse: &llm.ToolUse{
						ID:    "call_2",
						Name:  "write_file",
						Input: []byte(`{"path":"out.txt","content":"hi"}`),
					},
				},
			},
		},
	}
	out := buildMessages(msgs, "")
	if len(out) != 1 {
		t.Fatalf("want 1 message, got %d", len(out))
	}
	msg := out[0]
	if msg.Content != "Let me check." {
		t.Errorf("content: want %q, got %q", "Let me check.", msg.Content)
	}
	if len(msg.ToolCalls) != 1 {
		t.Fatalf("want 1 tool call, got %d", len(msg.ToolCalls))
	}
	if msg.ToolCalls[0].Function.Name != "write_file" {
		t.Errorf("tool name: want %q, got %q", "write_file", msg.ToolCalls[0].Function.Name)
	}
}

// ─── TestConvertOpenAIResponse ────────────────────────────────────────────────

func makeTextResponse(text string) openai.ChatCompletionResponse {
	return openai.ChatCompletionResponse{
		Choices: []openai.ChatCompletionChoice{
			{
				Message:      openai.ChatCompletionMessage{Content: text},
				FinishReason: openai.FinishReasonStop,
			},
		},
		Usage: openai.Usage{PromptTokens: 10, CompletionTokens: 5},
	}
}

func TestConvertOpenAIResponse_TextOnly(t *testing.T) {
	resp := makeTextResponse("hello world")
	got := convertOpenAIResponse(resp)
	if len(got.Content) != 1 {
		t.Fatalf("want 1 content block, got %d", len(got.Content))
	}
	if got.Content[0].Type != llm.ContentTypeText {
		t.Errorf("type: want text, got %q", got.Content[0].Type)
	}
	if got.Content[0].Text != "hello world" {
		t.Errorf("text: want %q, got %q", "hello world", got.Content[0].Text)
	}
	if got.StopReason != llm.StopReasonEndTurn {
		t.Errorf("stop reason: want end_turn, got %q", got.StopReason)
	}
	if got.Usage.InputTokens != 10 {
		t.Errorf("InputTokens: want 10, got %d", got.Usage.InputTokens)
	}
	if got.Usage.OutputTokens != 5 {
		t.Errorf("OutputTokens: want 5, got %d", got.Usage.OutputTokens)
	}
}

func TestConvertOpenAIResponse_MultipleToolCalls(t *testing.T) {
	resp := openai.ChatCompletionResponse{
		Choices: []openai.ChatCompletionChoice{
			{
				Message: openai.ChatCompletionMessage{
					ToolCalls: []openai.ToolCall{
						{
							ID:   "call_1",
							Type: openai.ToolTypeFunction,
							Function: openai.FunctionCall{
								Name:      "read_file",
								Arguments: `{"path":"a.txt"}`,
							},
						},
						{
							ID:   "call_2",
							Type: openai.ToolTypeFunction,
							Function: openai.FunctionCall{
								Name:      "write_file",
								Arguments: `{"path":"b.txt","content":"x"}`,
							},
						},
					},
				},
				FinishReason: openai.FinishReasonToolCalls,
			},
		},
	}
	got := convertOpenAIResponse(resp)
	if len(got.Content) != 2 {
		t.Fatalf("want 2 content blocks, got %d", len(got.Content))
	}
	for i, want := range []struct{ id, name string }{
		{"call_1", "read_file"},
		{"call_2", "write_file"},
	} {
		b := got.Content[i]
		if b.Type != llm.ContentTypeToolUse {
			t.Errorf("[%d] type: want tool_use, got %q", i, b.Type)
		}
		if b.ToolUse == nil {
			t.Fatalf("[%d] ToolUse is nil", i)
		}
		if b.ToolUse.ID != want.id {
			t.Errorf("[%d] ID: want %q, got %q", i, want.id, b.ToolUse.ID)
		}
		if b.ToolUse.Name != want.name {
			t.Errorf("[%d] Name: want %q, got %q", i, want.name, b.ToolUse.Name)
		}
	}
	if got.StopReason != llm.StopReasonToolUse {
		t.Errorf("stop reason: want tool_use, got %q", got.StopReason)
	}
}

func TestConvertOpenAIResponse_FinishReasonLength(t *testing.T) {
	resp := openai.ChatCompletionResponse{
		Choices: []openai.ChatCompletionChoice{
			{
				Message:      openai.ChatCompletionMessage{Content: "truncated"},
				FinishReason: openai.FinishReasonLength,
			},
		},
	}
	got := convertOpenAIResponse(resp)
	if got.StopReason != llm.StopReasonMaxTokens {
		t.Errorf("stop reason: want max_tokens, got %q", got.StopReason)
	}
}

// ─── TestMapOpenAIError ───────────────────────────────────────────────────────

func makeAPIError(code int) error {
	return &openai.APIError{
		HTTPStatusCode: code,
		Message:        "test error",
	}
}

func TestMapOpenAIError_RateLimit(t *testing.T) {
	err := mapOpenAIError(makeAPIError(429))
	var rl *llm.RateLimitError
	if !errors.As(err, &rl) {
		t.Errorf("want *llm.RateLimitError, got %T", err)
	}
	if !llm.Retryable(err) {
		t.Error("RateLimitError should be retryable")
	}
}

func TestMapOpenAIError_Auth(t *testing.T) {
	for _, code := range []int{401, 403} {
		err := mapOpenAIError(makeAPIError(code))
		var ae *llm.AuthError
		if !errors.As(err, &ae) {
			t.Errorf("code %d: want *llm.AuthError, got %T", code, err)
		}
		if llm.Retryable(err) {
			t.Errorf("code %d: AuthError should not be retryable", code)
		}
	}
}

func TestMapOpenAIError_Server(t *testing.T) {
	for _, code := range []int{500, 502, 503} {
		err := mapOpenAIError(makeAPIError(code))
		var se *llm.ServerError
		if !errors.As(err, &se) {
			t.Errorf("code %d: want *llm.ServerError, got %T", code, err)
		}
		if !llm.Retryable(err) {
			t.Errorf("code %d: ServerError should be retryable", code)
		}
	}
}

func TestMapOpenAIError_Nil(t *testing.T) {
	if err := mapOpenAIError(nil); err != nil {
		t.Errorf("want nil, got %v", err)
	}
}

// ─── TestBuildTools ───────────────────────────────────────────────────────────

func TestBuildTools(t *testing.T) {
	schema := json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"}}}`)
	defs := []llm.ToolDefinition{
		{
			Name:        "read_file",
			Description: "Read a file",
			InputSchema: schema,
		},
	}
	tools := buildTools(defs)
	if len(tools) != 1 {
		t.Fatalf("want 1 tool, got %d", len(tools))
	}
	tool := tools[0]
	if tool.Type != openai.ToolTypeFunction {
		t.Errorf("type: want function, got %q", tool.Type)
	}
	if tool.Function == nil {
		t.Fatal("Function is nil")
	}
	if tool.Function.Name != "read_file" {
		t.Errorf("name: want %q, got %q", "read_file", tool.Function.Name)
	}
	if tool.Function.Description != "Read a file" {
		t.Errorf("description: want %q, got %q", "Read a file", tool.Function.Description)
	}
}

// ─── Integration test (skipped without OPENAI_API_KEY) ───────────────────────

func TestOpenAIIntegration(t *testing.T) {
	t.Skipf("set OPENAI_API_KEY to run OpenAI integration test")

	client, err := newOpenAIClient("gpt-4o-mini")
	if err != nil {
		t.Skipf("skipping: %v", err)
	}
	ctx := context.Background()

	t.Run("simple_completion", func(t *testing.T) {
		req := llm.GenerateRequest{
			Messages: []llm.Message{
				llm.TextMessage(llm.RoleUser, "Say hello in exactly three words."),
			},
		}
		resp, err := client.Complete(ctx, req)
		if err != nil {
			t.Fatalf("Complete: %v", err)
		}
		if len(resp.Content) == 0 {
			t.Fatal("expected non-empty response content")
		}
		if resp.Content[0].Type != llm.ContentTypeText {
			t.Errorf("expected text block, got %q", resp.Content[0].Type)
		}
	})

	t.Run("tool_use", func(t *testing.T) {
		schema := json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"}},"required":["path"]}`)
		req := llm.GenerateRequest{
			System: "When asked to read a file, call the read_file tool.",
			Messages: []llm.Message{
				llm.TextMessage(llm.RoleUser, "Please read the file readme.txt"),
			},
			Tools: []llm.ToolDefinition{
				{Name: "read_file", Description: "Read a file by path", InputSchema: schema},
			},
		}
		resp, err := client.Complete(ctx, req)
		if err != nil {
			t.Fatalf("Complete with tools: %v", err)
		}
		var found bool
		for _, b := range resp.Content {
			if b.Type == llm.ContentTypeToolUse {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected at least one tool_use content block")
		}
	})
}
