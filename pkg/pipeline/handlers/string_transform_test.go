package handlers_test

import (
	"testing"

	"github.com/ravi-parthasarathy/attractor/pkg/pipeline"
	"github.com/ravi-parthasarathy/attractor/pkg/pipeline/handlers"
)

func stNode(id string, attrs map[string]string) *pipeline.Node {
	return &pipeline.Node{ID: id, Type: pipeline.NodeTypeStringTransform, Attrs: attrs}
}

func TestStringTransformTrim(t *testing.T) {
	t.Parallel()
	pctx := pipeline.NewPipelineContext()
	pctx.Set("raw", "  hello world  ")
	node := stNode("st", map[string]string{"source": "raw", "ops": "trim", "key": "out"})
	h := &handlers.StringTransformHandler{}
	if err := h.Handle(t.Context(), node, pctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := pctx.GetString("out"); got != "hello world" {
		t.Errorf("out = %q, want %q", got, "hello world")
	}
}

func TestStringTransformUpper(t *testing.T) {
	t.Parallel()
	pctx := pipeline.NewPipelineContext()
	pctx.Set("raw", "hello")
	node := stNode("st", map[string]string{"source": "raw", "ops": "upper", "key": "out"})
	h := &handlers.StringTransformHandler{}
	if err := h.Handle(t.Context(), node, pctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := pctx.GetString("out"); got != "HELLO" {
		t.Errorf("out = %q, want %q", got, "HELLO")
	}
}

func TestStringTransformLower(t *testing.T) {
	t.Parallel()
	pctx := pipeline.NewPipelineContext()
	pctx.Set("raw", "HELLO")
	node := stNode("st", map[string]string{"source": "raw", "ops": "lower", "key": "out"})
	h := &handlers.StringTransformHandler{}
	if err := h.Handle(t.Context(), node, pctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := pctx.GetString("out"); got != "hello" {
		t.Errorf("out = %q, want %q", got, "hello")
	}
}

func TestStringTransformReplace(t *testing.T) {
	t.Parallel()
	pctx := pipeline.NewPipelineContext()
	pctx.Set("raw", "foo bar foo")
	node := stNode("st", map[string]string{
		"source": "raw",
		"ops":    "replace",
		"key":    "out",
		"old":    "foo",
		"new":    "baz",
	})
	h := &handlers.StringTransformHandler{}
	if err := h.Handle(t.Context(), node, pctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := pctx.GetString("out"); got != "baz bar baz" {
		t.Errorf("out = %q, want %q", got, "baz bar baz")
	}
}

func TestStringTransformChain(t *testing.T) {
	t.Parallel()
	pctx := pipeline.NewPipelineContext()
	pctx.Set("raw", "  Hello World  ")
	// trim then lower
	node := stNode("st", map[string]string{"source": "raw", "ops": "trim,lower", "key": "out"})
	h := &handlers.StringTransformHandler{}
	if err := h.Handle(t.Context(), node, pctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := pctx.GetString("out"); got != "hello world" {
		t.Errorf("out = %q, want %q", got, "hello world")
	}
}

func TestStringTransformUnknownOp(t *testing.T) {
	t.Parallel()
	pctx := pipeline.NewPipelineContext()
	pctx.Set("raw", "hello")
	node := stNode("st", map[string]string{"source": "raw", "ops": "base64encode", "key": "out"})
	h := &handlers.StringTransformHandler{}
	if err := h.Handle(t.Context(), node, pctx); err == nil {
		t.Fatal("expected error for unknown op")
	}
}

func TestStringTransformReplaceWithTemplate(t *testing.T) {
	t.Parallel()
	pctx := pipeline.NewPipelineContext()
	pctx.Set("raw", "Hello __NAME__")
	pctx.Set("username", "Alice")
	node := stNode("st", map[string]string{
		"source": "raw",
		"ops":    "replace",
		"key":    "out",
		"old":    "__NAME__",
		"new":    "{{.username}}",
	})
	h := &handlers.StringTransformHandler{}
	if err := h.Handle(t.Context(), node, pctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := pctx.GetString("out"); got != "Hello Alice" {
		t.Errorf("out = %q, want %q", got, "Hello Alice")
	}
}

func TestStringTransformValidatorCatchesMissingAttrs(t *testing.T) {
	t.Parallel()
	node := &pipeline.Node{
		ID:    "st",
		Type:  pipeline.NodeTypeStringTransform,
		Attrs: map[string]string{},
	}
	errs := pipeline.ValidateNode(node)
	if len(errs) < 3 {
		t.Fatalf("expected 3 errors, got %d: %v", len(errs), errs)
	}
}
