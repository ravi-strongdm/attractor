package handlers_test

import (
	"strings"
	"testing"

	"github.com/ravi-parthasarathy/attractor/pkg/pipeline"
	"github.com/ravi-parthasarathy/attractor/pkg/pipeline/handlers"
)

func newAssertNode(id string, attrs map[string]string) *pipeline.Node {
	return &pipeline.Node{
		ID:    id,
		Type:  pipeline.NodeTypeAssert,
		Attrs: attrs,
	}
}

func TestAssertPass(t *testing.T) {
	t.Parallel()
	pctx := pipeline.NewPipelineContext()
	pctx.Set("status", "ok")

	node := newAssertNode("chk", map[string]string{
		"expr":    "status == 'ok'",
		"message": "status must be ok",
	})

	h := &handlers.AssertHandler{}
	if err := h.Handle(t.Context(), node, pctx); err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
}

func TestAssertFail(t *testing.T) {
	t.Parallel()
	pctx := pipeline.NewPipelineContext()
	pctx.Set("status", "error")

	node := newAssertNode("chk", map[string]string{
		"expr":    "status == 'ok'",
		"message": "status must be ok",
	})

	h := &handlers.AssertHandler{}
	err := h.Handle(t.Context(), node, pctx)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "status must be ok") {
		t.Errorf("error should contain message: %v", err)
	}
	if !strings.Contains(err.Error(), "status == 'ok'") {
		t.Errorf("error should contain expr: %v", err)
	}
}

func TestAssertDefaultMessage(t *testing.T) {
	t.Parallel()
	pctx := pipeline.NewPipelineContext()

	node := newAssertNode("chk", map[string]string{
		"expr": "missing_key",
	})

	h := &handlers.AssertHandler{}
	err := h.Handle(t.Context(), node, pctx)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "assertion failed") {
		t.Errorf("error should contain default message: %v", err)
	}
}

func TestAssertMissingExpr(t *testing.T) {
	t.Parallel()
	pctx := pipeline.NewPipelineContext()

	node := newAssertNode("chk", map[string]string{})

	h := &handlers.AssertHandler{}
	if err := h.Handle(t.Context(), node, pctx); err == nil {
		t.Fatal("expected error for missing expr, got nil")
	}
}

func TestAssertCompoundExpr(t *testing.T) {
	t.Parallel()
	pctx := pipeline.NewPipelineContext()
	pctx.Set("a", "1")
	pctx.Set("b", "2")

	node := newAssertNode("chk", map[string]string{
		"expr": "a == '1' && b == '2'",
	})

	h := &handlers.AssertHandler{}
	if err := h.Handle(t.Context(), node, pctx); err != nil {
		t.Fatalf("expected nil, got: %v", err)
	}
}
