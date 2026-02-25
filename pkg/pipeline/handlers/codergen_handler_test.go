package handlers_test

import (
	"context"
	"sync"
	"testing"

	"github.com/ravi-parthasarathy/attractor/pkg/llm"
	"github.com/ravi-parthasarathy/attractor/pkg/pipeline"
	"github.com/ravi-parthasarathy/attractor/pkg/pipeline/handlers"
)

// ─── mockClient ───────────────────────────────────────────────────────────────

// mockClient records the first GenerateRequest it receives and returns a
// simple non-tool text response so the agent loop completes in one turn.
type mockClient struct {
	mu       sync.Mutex
	lastReqs []llm.GenerateRequest
}

func (m *mockClient) Complete(_ context.Context, req llm.GenerateRequest) (llm.GenerateResponse, error) {
	m.mu.Lock()
	m.lastReqs = append(m.lastReqs, req)
	m.mu.Unlock()
	return llm.GenerateResponse{
		Content:    []llm.ContentBlock{{Type: llm.ContentTypeText, Text: "done"}},
		StopReason: llm.StopReasonEndTurn,
	}, nil
}

func (m *mockClient) Stream(_ context.Context, _ llm.GenerateRequest) (<-chan llm.StreamEvent, error) {
	ch := make(chan llm.StreamEvent)
	close(ch)
	return ch, nil
}

// registerMock registers a "mock" provider that returns mc for any model name.
func registerMock(t *testing.T, mc *mockClient) {
	t.Helper()
	llm.RegisterProvider("mock", func(_ string) (llm.Client, error) {
		return mc, nil
	})
	t.Cleanup(func() {
		// Re-register a nil factory so subsequent test runs start clean.
		llm.RegisterProvider("mock", nil)
	})
}

// ─── TestCodergenSystemPrompt ─────────────────────────────────────────────────

func TestCodergenSystemPrompt(t *testing.T) {
	mc := &mockClient{}
	registerMock(t, mc)

	dir := t.TempDir()
	h := &handlers.CodergenHandler{DefaultModel: "mock:test", Workdir: dir}

	node := &pipeline.Node{
		ID:   "gen",
		Type: pipeline.NodeType("codergen"),
		Attrs: map[string]string{
			"prompt":        "write hello world",
			"system_prompt": "You are a test assistant.",
		},
	}
	pctx := pipeline.NewPipelineContext()

	if err := h.Handle(context.Background(), node, pctx); err != nil {
		t.Fatalf("Handle: %v", err)
	}

	mc.mu.Lock()
	reqs := mc.lastReqs
	mc.mu.Unlock()

	if len(reqs) == 0 {
		t.Fatal("mock client received no requests")
	}
	got := reqs[0].System
	want := "You are a test assistant."
	if got != want {
		t.Errorf("GenerateRequest.System = %q, want %q", got, want)
	}
}

// ─── TestCodergenSystemPrompt_Default ────────────────────────────────────────

// When system_prompt is absent the agent uses its built-in default (empty or
// whatever the session initialises with). We only verify no error occurs.
func TestCodergenSystemPrompt_Default(t *testing.T) {
	mc := &mockClient{}
	registerMock(t, mc)

	dir := t.TempDir()
	h := &handlers.CodergenHandler{DefaultModel: "mock:test", Workdir: dir}

	node := &pipeline.Node{
		ID:    "gen",
		Type:  pipeline.NodeType("codergen"),
		Attrs: map[string]string{"prompt": "hello"},
	}
	pctx := pipeline.NewPipelineContext()

	if err := h.Handle(context.Background(), node, pctx); err != nil {
		t.Fatalf("Handle: %v", err)
	}
}
