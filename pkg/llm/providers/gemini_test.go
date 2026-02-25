package providers

import (
	"encoding/json"
	"testing"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/googleapi"

	"github.com/ravi-parthasarathy/attractor/pkg/llm"
)

// ─── TestBuildContents ────────────────────────────────────────────────────────

func TestBuildContents_UserText(t *testing.T) {
	msgs := []llm.Message{
		llm.TextMessage(llm.RoleUser, "hello gemini"),
	}
	hist, last, err := buildContents(msgs)
	if err != nil {
		t.Fatalf("buildContents: %v", err)
	}
	if len(hist) != 0 {
		t.Errorf("history len = %d, want 0", len(hist))
	}
	if last == nil {
		t.Fatal("last content is nil")
	}
	if last.Role != "user" {
		t.Errorf("role = %q, want user", last.Role)
	}
	if len(last.Parts) != 1 {
		t.Fatalf("parts len = %d, want 1", len(last.Parts))
	}
	text, ok := last.Parts[0].(genai.Text)
	if !ok {
		t.Fatalf("part type = %T, want genai.Text", last.Parts[0])
	}
	if string(text) != "hello gemini" {
		t.Errorf("text = %q, want %q", string(text), "hello gemini")
	}
}

func TestBuildContents_SystemStripped(t *testing.T) {
	msgs := []llm.Message{
		llm.TextMessage(llm.RoleSystem, "you are helpful"),
		llm.TextMessage(llm.RoleUser, "hi"),
	}
	hist, last, err := buildContents(msgs)
	if err != nil {
		t.Fatalf("buildContents: %v", err)
	}
	// System message must not appear in history.
	if len(hist) != 0 {
		t.Errorf("history should be empty (system msg stripped), got %d entries", len(hist))
	}
	if last == nil || last.Role != "user" {
		t.Error("last message should be the user message")
	}
}

func TestBuildContents_AssistantText(t *testing.T) {
	msgs := []llm.Message{
		llm.TextMessage(llm.RoleUser, "say hello"),
		llm.TextMessage(llm.RoleAssistant, "hello"),
		llm.TextMessage(llm.RoleUser, "thanks"),
	}
	hist, last, err := buildContents(msgs)
	if err != nil {
		t.Fatalf("buildContents: %v", err)
	}
	if len(hist) != 2 {
		t.Fatalf("history len = %d, want 2", len(hist))
	}
	// First history entry: user
	if hist[0].Role != "user" {
		t.Errorf("hist[0].Role = %q, want user", hist[0].Role)
	}
	// Second history entry: assistant → "model" in Gemini
	if hist[1].Role != "model" {
		t.Errorf("hist[1].Role = %q, want model", hist[1].Role)
	}
	if last == nil || last.Role != "user" {
		t.Error("last should be the final user message")
	}
}

func TestBuildContents_ToolCall(t *testing.T) {
	msgs := []llm.Message{
		llm.TextMessage(llm.RoleUser, "search"),
		{
			Role: llm.RoleAssistant,
			Content: []llm.ContentBlock{
				{
					Type: llm.ContentTypeToolUse,
					ToolUse: &llm.ToolUse{
						ID:    "call-1",
						Name:  "search_file",
						Input: []byte(`{"pattern":"func main"}`),
					},
				},
			},
		},
		// dummy final user message
		llm.TextMessage(llm.RoleUser, "continue"),
	}
	hist, _, err := buildContents(msgs)
	if err != nil {
		t.Fatalf("buildContents: %v", err)
	}
	// hist[1] should be the assistant content with a FunctionCall part.
	if len(hist) < 2 {
		t.Fatalf("history too short: %d", len(hist))
	}
	assistantContent := hist[1]
	if assistantContent.Role != "model" {
		t.Errorf("role = %q, want model", assistantContent.Role)
	}
	if len(assistantContent.Parts) != 1 {
		t.Fatalf("parts len = %d, want 1", len(assistantContent.Parts))
	}
	fc, ok := assistantContent.Parts[0].(genai.FunctionCall)
	if !ok {
		t.Fatalf("part type = %T, want genai.FunctionCall", assistantContent.Parts[0])
	}
	if fc.Name != "search_file" {
		t.Errorf("function name = %q, want search_file", fc.Name)
	}
	if fc.Args["pattern"] != "func main" {
		t.Errorf("args.pattern = %v, want func main", fc.Args["pattern"])
	}
}

func TestBuildContents_ToolResult(t *testing.T) {
	msgs := []llm.Message{
		llm.TextMessage(llm.RoleUser, "search"),
		{
			Role: llm.RoleAssistant,
			Content: []llm.ContentBlock{
				{
					Type: llm.ContentTypeToolUse,
					ToolUse: &llm.ToolUse{
						ID:   "call-1",
						Name: "search_file",
					},
				},
			},
		},
		{
			Role: llm.RoleUser,
			Content: []llm.ContentBlock{
				{
					Type: llm.ContentTypeToolResult,
					ToolResult: &llm.ToolResult{
						ToolUseID: "call-1",
						Content:   "main.go:3: func main() {}",
					},
				},
			},
		},
		llm.TextMessage(llm.RoleUser, "done"),
	}
	hist, _, err := buildContents(msgs)
	if err != nil {
		t.Fatalf("buildContents: %v", err)
	}
	// hist[2] should be the tool-result user message with a FunctionResponse.
	if len(hist) < 3 {
		t.Fatalf("history too short: %d", len(hist))
	}
	trContent := hist[2]
	if trContent.Role != "user" {
		t.Errorf("tool result role = %q, want user", trContent.Role)
	}
	if len(trContent.Parts) != 1 {
		t.Fatalf("parts len = %d, want 1", len(trContent.Parts))
	}
	fr, ok := trContent.Parts[0].(genai.FunctionResponse)
	if !ok {
		t.Fatalf("part type = %T, want genai.FunctionResponse", trContent.Parts[0])
	}
	// Name must be resolved from the ToolUseID → "search_file".
	if fr.Name != "search_file" {
		t.Errorf("function response name = %q, want search_file", fr.Name)
	}
	if fr.Response["result"] != "main.go:3: func main() {}" {
		t.Errorf("response result = %v", fr.Response["result"])
	}
}

// ─── TestJsonSchemaToGenai ────────────────────────────────────────────────────

func TestJsonSchemaToGenai_Object(t *testing.T) {
	raw := json.RawMessage(`{
		"type": "object",
		"description": "file operation",
		"properties": {
			"path": {"type": "string", "description": "file path"},
			"content": {"type": "string"}
		},
		"required": ["path"]
	}`)
	schema, err := jsonSchemaToGenai(raw)
	if err != nil {
		t.Fatalf("jsonSchemaToGenai: %v", err)
	}
	if schema.Type != genai.TypeObject {
		t.Errorf("type = %v, want TypeObject", schema.Type)
	}
	if schema.Description != "file operation" {
		t.Errorf("description = %q, want %q", schema.Description, "file operation")
	}
	if len(schema.Properties) != 2 {
		t.Errorf("properties len = %d, want 2", len(schema.Properties))
	}
	if schema.Properties["path"].Type != genai.TypeString {
		t.Errorf("path type = %v, want TypeString", schema.Properties["path"].Type)
	}
	if schema.Properties["path"].Description != "file path" {
		t.Errorf("path description = %q", schema.Properties["path"].Description)
	}
	if len(schema.Required) != 1 || schema.Required[0] != "path" {
		t.Errorf("required = %v, want [path]", schema.Required)
	}
}

func TestJsonSchemaToGenai_Primitives(t *testing.T) {
	cases := []struct {
		jsonType string
		want     genai.Type
	}{
		{"string", genai.TypeString},
		{"integer", genai.TypeInteger},
		{"number", genai.TypeNumber},
		{"boolean", genai.TypeBoolean},
		{"array", genai.TypeArray},
	}
	for _, tc := range cases {
		raw := json.RawMessage(`{"type":"` + tc.jsonType + `"}`)
		schema, err := jsonSchemaToGenai(raw)
		if err != nil {
			t.Errorf("%s: %v", tc.jsonType, err)
			continue
		}
		if schema.Type != tc.want {
			t.Errorf("%s: type = %v, want %v", tc.jsonType, schema.Type, tc.want)
		}
	}
}

func TestJsonSchemaToGenai_Array(t *testing.T) {
	raw := json.RawMessage(`{"type":"array","items":{"type":"string"}}`)
	schema, err := jsonSchemaToGenai(raw)
	if err != nil {
		t.Fatalf("jsonSchemaToGenai: %v", err)
	}
	if schema.Type != genai.TypeArray {
		t.Errorf("type = %v, want TypeArray", schema.Type)
	}
	if schema.Items == nil {
		t.Fatal("items is nil")
	}
	if schema.Items.Type != genai.TypeString {
		t.Errorf("items.type = %v, want TypeString", schema.Items.Type)
	}
}

// ─── TestConvertGeminiResponse ────────────────────────────────────────────────

func TestConvertGeminiResponse_Text(t *testing.T) {
	resp := &genai.GenerateContentResponse{
		Candidates: []*genai.Candidate{
			{
				Content:      &genai.Content{Role: "model", Parts: []genai.Part{genai.Text("hello")}},
				FinishReason: genai.FinishReasonStop,
			},
		},
		UsageMetadata: &genai.UsageMetadata{
			PromptTokenCount:      10,
			CandidatesTokenCount:  5,
		},
	}
	got := convertGeminiResponse(resp)
	if len(got.Content) != 1 {
		t.Fatalf("content len = %d, want 1", len(got.Content))
	}
	if got.Content[0].Type != llm.ContentTypeText {
		t.Errorf("type = %v, want text", got.Content[0].Type)
	}
	if got.Content[0].Text != "hello" {
		t.Errorf("text = %q, want hello", got.Content[0].Text)
	}
	if got.StopReason != llm.StopReasonEndTurn {
		t.Errorf("stop_reason = %v, want end_turn", got.StopReason)
	}
	if got.Usage.InputTokens != 10 || got.Usage.OutputTokens != 5 {
		t.Errorf("usage = %+v, want {10, 5}", got.Usage)
	}
}

func TestConvertGeminiResponse_ToolCall(t *testing.T) {
	resp := &genai.GenerateContentResponse{
		Candidates: []*genai.Candidate{
			{
				Content: &genai.Content{
					Role: "model",
					Parts: []genai.Part{
						genai.FunctionCall{Name: "read_file", Args: map[string]any{"path": "main.go"}},
					},
				},
				FinishReason: genai.FinishReasonStop,
			},
		},
	}
	got := convertGeminiResponse(resp)
	if len(got.Content) != 1 {
		t.Fatalf("content len = %d, want 1", len(got.Content))
	}
	if got.Content[0].Type != llm.ContentTypeToolUse {
		t.Errorf("type = %v, want tool_use", got.Content[0].Type)
	}
	tu := got.Content[0].ToolUse
	if tu == nil || tu.Name != "read_file" {
		t.Errorf("tool_use.name = %v, want read_file", tu)
	}
	if got.StopReason != llm.StopReasonToolUse {
		t.Errorf("stop_reason = %v, want tool_use", got.StopReason)
	}
}

func TestConvertGeminiResponse_MaxTokens(t *testing.T) {
	resp := &genai.GenerateContentResponse{
		Candidates: []*genai.Candidate{
			{
				Content:      &genai.Content{Role: "model", Parts: []genai.Part{genai.Text("truncated")}},
				FinishReason: genai.FinishReasonMaxTokens,
			},
		},
	}
	got := convertGeminiResponse(resp)
	if got.StopReason != llm.StopReasonMaxTokens {
		t.Errorf("stop_reason = %v, want max_tokens", got.StopReason)
	}
}

// ─── TestMapGeminiError ───────────────────────────────────────────────────────

func TestMapGeminiError_RateLimit(t *testing.T) {
	err := mapGeminiError(&googleapi.Error{Code: 429, Message: "quota exceeded"})
	var rl *llm.RateLimitError
	if !isType(err, &rl) {
		t.Errorf("expected RateLimitError, got %T", err)
	}
}

func TestMapGeminiError_Auth(t *testing.T) {
	for _, code := range []int{401, 403} {
		err := mapGeminiError(&googleapi.Error{Code: code, Message: "unauthorized"})
		var ae *llm.AuthError
		if !isType(err, &ae) {
			t.Errorf("code %d: expected AuthError, got %T", code, err)
		}
	}
}

func TestMapGeminiError_Server(t *testing.T) {
	err := mapGeminiError(&googleapi.Error{Code: 503, Message: "unavailable"})
	var se *llm.ServerError
	if !isType(err, &se) {
		t.Errorf("expected ServerError, got %T", err)
	}
}

func TestMapGeminiError_Nil(t *testing.T) {
	if got := mapGeminiError(nil); got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

// isType is a helper to check error types without using errors.As generics.
func isType[T error](err error, target *T) bool {
	if err == nil {
		return false
	}
	_, ok := err.(T)
	return ok
}
