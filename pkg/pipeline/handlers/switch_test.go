package handlers_test

import (
	"testing"

	"github.com/ravi-parthasarathy/attractor/pkg/pipeline"
	"github.com/ravi-parthasarathy/attractor/pkg/pipeline/handlers"
)

func TestSwitchHandlerValidKey(t *testing.T) {
	t.Parallel()
	pctx := pipeline.NewPipelineContext()
	pctx.Set("status", "ok")

	node := &pipeline.Node{
		ID:    "route",
		Type:  pipeline.NodeTypeSwitch,
		Attrs: map[string]string{"key": "status"},
	}
	h := &handlers.SwitchHandler{}
	if err := h.Handle(t.Context(), node, pctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSwitchHandlerMissingKey(t *testing.T) {
	t.Parallel()
	pctx := pipeline.NewPipelineContext()
	node := &pipeline.Node{
		ID:    "route",
		Type:  pipeline.NodeTypeSwitch,
		Attrs: map[string]string{},
	}
	h := &handlers.SwitchHandler{}
	if err := h.Handle(t.Context(), node, pctx); err == nil {
		t.Fatal("expected error for missing key attr, got nil")
	}
}

func TestSwitchHandlerKeyNotInContext(t *testing.T) {
	t.Parallel()
	// Key attr present but context value absent — handler should not error
	// (routing happens in engine; handler just warns).
	pctx := pipeline.NewPipelineContext()
	node := &pipeline.Node{
		ID:    "route",
		Type:  pipeline.NodeTypeSwitch,
		Attrs: map[string]string{"key": "missing"},
	}
	h := &handlers.SwitchHandler{}
	if err := h.Handle(t.Context(), node, pctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
