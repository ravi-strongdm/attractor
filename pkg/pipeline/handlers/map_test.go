package handlers_test

import (
	"strings"
	"testing"

	"github.com/ravi-parthasarathy/attractor/pkg/pipeline"
	"github.com/ravi-parthasarathy/attractor/pkg/pipeline/handlers"
)

func mapNode(id string, attrs map[string]string) *pipeline.Node {
	return &pipeline.Node{ID: id, Type: pipeline.NodeTypeMap, Attrs: attrs}
}

func TestMapEmptyArray(t *testing.T) {
	t.Parallel()
	pctx := pipeline.NewPipelineContext()
	pctx.Set("files", "[]")

	node := mapNode("m", map[string]string{
		"items":    "files",
		"item_key": "f",
		"prompt":   "analyse {{.f}}",
	})
	h := &handlers.MapHandler{}
	if err := h.Handle(t.Context(), node, pctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := pctx.GetString("m_results"); got != "[]" {
		t.Errorf("got %q, want %q", got, "[]")
	}
}

func TestMapEmptyItemsKey(t *testing.T) {
	t.Parallel()
	// items key not set in context → empty string → treated as empty
	pctx := pipeline.NewPipelineContext()

	node := mapNode("m", map[string]string{
		"items":    "files",
		"item_key": "f",
		"prompt":   "analyse {{.f}}",
	})
	h := &handlers.MapHandler{}
	if err := h.Handle(t.Context(), node, pctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := pctx.GetString("m_results"); got != "[]" {
		t.Errorf("got %q, want %q", got, "[]")
	}
}

func TestMapInvalidJSON(t *testing.T) {
	t.Parallel()
	pctx := pipeline.NewPipelineContext()
	pctx.Set("files", "not-json")

	node := mapNode("m", map[string]string{
		"items":    "files",
		"item_key": "f",
		"prompt":   "analyse {{.f}}",
	})
	h := &handlers.MapHandler{}
	if err := h.Handle(t.Context(), node, pctx); err == nil {
		t.Fatal("expected error for invalid JSON in items key")
	}
}

func TestMapNonArrayJSON(t *testing.T) {
	t.Parallel()
	pctx := pipeline.NewPipelineContext()
	pctx.Set("files", `{"not":"array"}`)

	node := mapNode("m", map[string]string{
		"items":    "files",
		"item_key": "f",
		"prompt":   "analyse {{.f}}",
	})
	h := &handlers.MapHandler{}
	if err := h.Handle(t.Context(), node, pctx); err == nil {
		t.Fatal("expected error for JSON object (not array) in items key")
	}
}

func TestMapMissingItemsAttr(t *testing.T) {
	t.Parallel()
	pctx := pipeline.NewPipelineContext()
	node := mapNode("m", map[string]string{
		"item_key": "f",
		"prompt":   "analyse {{.f}}",
	})
	h := &handlers.MapHandler{}
	if err := h.Handle(t.Context(), node, pctx); err == nil {
		t.Fatal("expected error for missing items attr")
	}
}

func TestMapMissingItemKeyAttr(t *testing.T) {
	t.Parallel()
	pctx := pipeline.NewPipelineContext()
	node := mapNode("m", map[string]string{
		"items":  "files",
		"prompt": "analyse",
	})
	h := &handlers.MapHandler{}
	if err := h.Handle(t.Context(), node, pctx); err == nil {
		t.Fatal("expected error for missing item_key attr")
	}
}

func TestMapMissingPromptAttr(t *testing.T) {
	t.Parallel()
	pctx := pipeline.NewPipelineContext()
	node := mapNode("m", map[string]string{
		"items":    "files",
		"item_key": "f",
	})
	h := &handlers.MapHandler{}
	if err := h.Handle(t.Context(), node, pctx); err == nil {
		t.Fatal("expected error for missing prompt attr")
	}
}

func TestMapCustomResultsKey(t *testing.T) {
	t.Parallel()
	// Empty array with custom results_key — verifies key naming works.
	pctx := pipeline.NewPipelineContext()
	pctx.Set("items", "[]")

	node := mapNode("proc", map[string]string{
		"items":       "items",
		"item_key":    "it",
		"prompt":      "do {{.it}}",
		"results_key": "my_results",
	})
	h := &handlers.MapHandler{}
	if err := h.Handle(t.Context(), node, pctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := pctx.GetString("my_results"); got != "[]" {
		t.Errorf("got %q, want %q", got, "[]")
	}
}

func TestMapLLMErrorPropagated(t *testing.T) {
	t.Parallel()
	// Non-empty array with an invalid model → LLM client creation fails.
	// The error should propagate and mention the item index.
	pctx := pipeline.NewPipelineContext()
	pctx.Set("items", `["a","b"]`)

	node := mapNode("m", map[string]string{
		"items":    "items",
		"item_key": "x",
		"prompt":   "analyse {{.x}}",
		"model":    "invalid-provider:no-such-model",
	})
	h := &handlers.MapHandler{}
	err := h.Handle(t.Context(), node, pctx)
	if err == nil {
		t.Fatal("expected error for invalid model, got nil")
	}
	if !strings.Contains(err.Error(), "map node") {
		t.Errorf("error should mention 'map node': %v", err)
	}
}
