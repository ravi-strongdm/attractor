package handlers_test

import (
	"context"
	"strings"
	"testing"

	"github.com/ravi-parthasarathy/attractor/pkg/pipeline"
	"github.com/ravi-parthasarathy/attractor/pkg/pipeline/handlers"
)

func newSleepNode(id string, attrs map[string]string) *pipeline.Node {
	return &pipeline.Node{
		ID:    id,
		Type:  pipeline.NodeTypeSleep,
		Attrs: attrs,
	}
}

func TestSleepNormal(t *testing.T) {
	t.Parallel()
	pctx := pipeline.NewPipelineContext()
	node := newSleepNode("pause", map[string]string{"duration": "10ms"})

	h := &handlers.SleepHandler{}
	if err := h.Handle(t.Context(), node, pctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSleepCancelled(t *testing.T) {
	t.Parallel()
	pctx := pipeline.NewPipelineContext()
	node := newSleepNode("pause", map[string]string{"duration": "10s"})

	ctx, cancel := context.WithCancel(t.Context())
	cancel() // cancel immediately

	h := &handlers.SleepHandler{}
	err := h.Handle(ctx, node, pctx)
	if err == nil {
		t.Fatal("expected cancellation error, got nil")
	}
	if !strings.Contains(err.Error(), "cancelled") {
		t.Errorf("error should mention 'cancelled': %v", err)
	}
}

func TestSleepMissingDuration(t *testing.T) {
	t.Parallel()
	pctx := pipeline.NewPipelineContext()
	node := newSleepNode("pause", map[string]string{})

	h := &handlers.SleepHandler{}
	if err := h.Handle(t.Context(), node, pctx); err == nil {
		t.Fatal("expected error for missing duration, got nil")
	}
}

func TestSleepInvalidDuration(t *testing.T) {
	t.Parallel()
	pctx := pipeline.NewPipelineContext()
	node := newSleepNode("pause", map[string]string{"duration": "not-a-duration"})

	h := &handlers.SleepHandler{}
	if err := h.Handle(t.Context(), node, pctx); err == nil {
		t.Fatal("expected error for invalid duration, got nil")
	}
}
