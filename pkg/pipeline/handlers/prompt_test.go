package handlers_test

import (
	"testing"

	"github.com/ravi-parthasarathy/attractor/pkg/pipeline"
	"github.com/ravi-parthasarathy/attractor/pkg/pipeline/handlers"
)

func promptNode(id string, attrs map[string]string) *pipeline.Node {
	return &pipeline.Node{ID: id, Type: pipeline.NodeTypePrompt, Attrs: attrs}
}

func TestPromptMissingPromptAttr(t *testing.T) {
	t.Parallel()
	pctx := pipeline.NewPipelineContext()
	node := promptNode("p", map[string]string{"key": "out"})
	h := &handlers.PromptHandler{}
	if err := h.Handle(t.Context(), node, pctx); err == nil {
		t.Fatal("expected error for missing 'prompt' attribute")
	}
}

func TestPromptMissingKeyAttr(t *testing.T) {
	t.Parallel()
	pctx := pipeline.NewPipelineContext()
	node := promptNode("p", map[string]string{"prompt": "hello"})
	h := &handlers.PromptHandler{}
	if err := h.Handle(t.Context(), node, pctx); err == nil {
		t.Fatal("expected error for missing 'key' attribute")
	}
}

func TestPromptInvalidModel(t *testing.T) {
	t.Parallel()
	pctx := pipeline.NewPipelineContext()
	node := promptNode("p", map[string]string{
		"prompt": "hello",
		"key":    "out",
		"model":  "invalid-provider:no-such-model",
	})
	h := &handlers.PromptHandler{}
	if err := h.Handle(t.Context(), node, pctx); err == nil {
		t.Fatal("expected error for invalid model")
	}
}

func TestPromptTemplateError(t *testing.T) {
	t.Parallel()
	pctx := pipeline.NewPipelineContext()
	// Malformed template.
	node := promptNode("p", map[string]string{
		"prompt": "{{.unclosed",
		"key":    "out",
		"model":  "invalid-provider:x",
	})
	h := &handlers.PromptHandler{}
	if err := h.Handle(t.Context(), node, pctx); err == nil {
		t.Fatal("expected error for malformed template")
	}
}

func TestPromptValidatorCatchesMissingAttrs(t *testing.T) {
	t.Parallel()
	// Missing both prompt and key.
	node := &pipeline.Node{
		ID:    "p",
		Type:  pipeline.NodeTypePrompt,
		Attrs: map[string]string{},
	}
	errs := pipeline.ValidateNode(node)
	if len(errs) == 0 {
		t.Fatal("expected validator errors for missing prompt and key attrs")
	}
	found := map[string]bool{}
	for _, e := range errs {
		found[e.Error()] = true
	}
	// Both attrs should be flagged.
	if len(errs) < 2 {
		t.Errorf("expected at least 2 errors, got %d: %v", len(errs), errs)
	}
}
