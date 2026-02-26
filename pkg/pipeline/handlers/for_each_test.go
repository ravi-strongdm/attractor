package handlers_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/ravi-parthasarathy/attractor/pkg/pipeline"
	"github.com/ravi-parthasarathy/attractor/pkg/pipeline/handlers"
)

func forEachNode(id string, attrs map[string]string) *pipeline.Node {
	return &pipeline.Node{ID: id, Type: pipeline.NodeTypeForEach, Attrs: attrs}
}

func TestForEachBasic(t *testing.T) {
	t.Parallel()
	pctx := pipeline.NewPipelineContext()
	pctx.Set("items", `["a","b","c"]`)

	node := forEachNode("fe", map[string]string{
		"items":    "items",
		"item_key": "it",
		"cmd":      "echo {{.it}}",
	})
	h := &handlers.ForEachHandler{}
	if err := h.Handle(t.Context(), node, pctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	raw := pctx.GetString("fe_results")
	var got []string
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("unmarshal results: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 results, got %d", len(got))
	}
	if strings.TrimSpace(got[0]) != "a" {
		t.Errorf("results[0] = %q, want %q", got[0], "a")
	}
}

func TestForEachEmptyArray(t *testing.T) {
	t.Parallel()
	pctx := pipeline.NewPipelineContext()
	pctx.Set("items", "[]")

	node := forEachNode("fe", map[string]string{
		"items":    "items",
		"item_key": "it",
		"cmd":      "echo {{.it}}",
	})
	h := &handlers.ForEachHandler{}
	if err := h.Handle(t.Context(), node, pctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := pctx.GetString("fe_results"); got != "[]" {
		t.Errorf("results = %q, want %q", got, "[]")
	}
}

func TestForEachMissingItemsKey(t *testing.T) {
	t.Parallel()
	// items key not present in context → empty → "[]"
	pctx := pipeline.NewPipelineContext()

	node := forEachNode("fe", map[string]string{
		"items":    "items",
		"item_key": "it",
		"cmd":      "echo hi",
	})
	h := &handlers.ForEachHandler{}
	if err := h.Handle(t.Context(), node, pctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := pctx.GetString("fe_results"); got != "[]" {
		t.Errorf("results = %q, want %q", got, "[]")
	}
}

func TestForEachItemKey(t *testing.T) {
	t.Parallel()
	pctx := pipeline.NewPipelineContext()
	pctx.Set("items", `["hello"]`)

	node := forEachNode("fe", map[string]string{
		"items":    "items",
		"item_key": "word",
		"cmd":      "echo upper:{{.word}}",
	})
	h := &handlers.ForEachHandler{}
	if err := h.Handle(t.Context(), node, pctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	raw := pctx.GetString("fe_results")
	var got []string
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !strings.Contains(got[0], "hello") {
		t.Errorf("expected item key substitution, got %q", got[0])
	}
}

func TestForEachFailOnError(t *testing.T) {
	t.Parallel()
	pctx := pipeline.NewPipelineContext()
	pctx.Set("items", `["x"]`)

	node := forEachNode("fe", map[string]string{
		"items":    "items",
		"item_key": "it",
		"cmd":      "exit 1",
	})
	h := &handlers.ForEachHandler{}
	if err := h.Handle(t.Context(), node, pctx); err == nil {
		t.Fatal("expected error on non-zero exit")
	}
}

func TestForEachNoFailOnError(t *testing.T) {
	t.Parallel()
	pctx := pipeline.NewPipelineContext()
	pctx.Set("items", `["x","y"]`)

	node := forEachNode("fe", map[string]string{
		"items":         "items",
		"item_key":      "it",
		"cmd":           "exit 1",
		"fail_on_error": "false",
	})
	h := &handlers.ForEachHandler{}
	if err := h.Handle(t.Context(), node, pctx); err != nil {
		t.Fatalf("expected no error with fail_on_error=false, got: %v", err)
	}
}

func TestForEachTimeout(t *testing.T) {
	t.Parallel()
	pctx := pipeline.NewPipelineContext()
	pctx.Set("items", `["x"]`)

	node := forEachNode("fe", map[string]string{
		"items":    "items",
		"item_key": "it",
		"cmd":      "sleep 10",
		"timeout":  "50ms",
	})
	h := &handlers.ForEachHandler{}
	if err := h.Handle(t.Context(), node, pctx); err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestForEachCustomResultsKey(t *testing.T) {
	t.Parallel()
	pctx := pipeline.NewPipelineContext()
	pctx.Set("items", `["a"]`)

	node := forEachNode("fe", map[string]string{
		"items":       "items",
		"item_key":    "it",
		"cmd":         "echo {{.it}}",
		"results_key": "my_results",
	})
	h := &handlers.ForEachHandler{}
	if err := h.Handle(t.Context(), node, pctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := pctx.GetString("my_results"); got == "" {
		t.Error("expected my_results to be set")
	}
}

func TestForEachValidatorCatchesMissingAttrs(t *testing.T) {
	t.Parallel()
	node := &pipeline.Node{
		ID:    "fe",
		Type:  pipeline.NodeTypeForEach,
		Attrs: map[string]string{},
	}
	errs := pipeline.ValidateNode(node)
	if len(errs) < 3 {
		t.Fatalf("expected 3 validator errors, got %d: %v", len(errs), errs)
	}
}
