package handlers_test

import (
	"encoding/json"
	"testing"

	"github.com/ravi-parthasarathy/attractor/pkg/pipeline"
	"github.com/ravi-parthasarathy/attractor/pkg/pipeline/handlers"
)

func jsonPackNode(id string, attrs map[string]string) *pipeline.Node {
	return &pipeline.Node{ID: id, Type: pipeline.NodeTypeJSONPack, Attrs: attrs}
}

func TestJSONPackBasic(t *testing.T) {
	t.Parallel()
	pctx := pipeline.NewPipelineContext()
	pctx.Set("name", "Alice")
	pctx.Set("age", "30")
	pctx.Set("city", "Boston")

	node := jsonPackNode("p", map[string]string{
		"keys":   "name,age,city",
		"output": "packed",
	})
	h := &handlers.JSONPackHandler{}
	if err := h.Handle(t.Context(), node, pctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	raw := pctx.GetString("packed")
	if raw == "" {
		t.Fatal("expected packed key to be set")
	}
	var got map[string]string
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("unmarshal packed: %v", err)
	}
	if got["name"] != "Alice" {
		t.Errorf("name = %q, want %q", got["name"], "Alice")
	}
	if got["age"] != "30" {
		t.Errorf("age = %q, want %q", got["age"], "30")
	}
	if got["city"] != "Boston" {
		t.Errorf("city = %q, want %q", got["city"], "Boston")
	}
}

func TestJSONPackEmptyKeys(t *testing.T) {
	t.Parallel()
	// keys attr contains only whitespace → empty object stored.
	pctx := pipeline.NewPipelineContext()
	node := jsonPackNode("p", map[string]string{
		"keys":   " ",
		"output": "packed",
	})
	h := &handlers.JSONPackHandler{}
	if err := h.Handle(t.Context(), node, pctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	raw := pctx.GetString("packed")
	var got map[string]string
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty object, got %v", got)
	}
}

func TestJSONPackMissingContextKey(t *testing.T) {
	t.Parallel()
	// Key listed but not set in context → stored as empty string.
	pctx := pipeline.NewPipelineContext()
	pctx.Set("present", "yes")

	node := jsonPackNode("p", map[string]string{
		"keys":   "present,absent",
		"output": "packed",
	})
	h := &handlers.JSONPackHandler{}
	if err := h.Handle(t.Context(), node, pctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	raw := pctx.GetString("packed")
	var got map[string]string
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got["present"] != "yes" {
		t.Errorf("present = %q, want %q", got["present"], "yes")
	}
	if got["absent"] != "" {
		t.Errorf("absent = %q, want empty string", got["absent"])
	}
}

func TestJSONPackMissingKeysAttr(t *testing.T) {
	t.Parallel()
	pctx := pipeline.NewPipelineContext()
	node := jsonPackNode("p", map[string]string{"output": "packed"})
	h := &handlers.JSONPackHandler{}
	if err := h.Handle(t.Context(), node, pctx); err == nil {
		t.Fatal("expected error for missing keys attr")
	}
}

func TestJSONPackMissingOutputAttr(t *testing.T) {
	t.Parallel()
	pctx := pipeline.NewPipelineContext()
	node := jsonPackNode("p", map[string]string{"keys": "x"})
	h := &handlers.JSONPackHandler{}
	if err := h.Handle(t.Context(), node, pctx); err == nil {
		t.Fatal("expected error for missing output attr")
	}
}

func TestJSONPackValidatorCatchesMissingAttrs(t *testing.T) {
	t.Parallel()
	node := &pipeline.Node{
		ID:    "p",
		Type:  pipeline.NodeTypeJSONPack,
		Attrs: map[string]string{},
	}
	errs := pipeline.ValidateNode(node)
	if len(errs) < 2 {
		t.Fatalf("expected at least 2 validator errors, got %d: %v", len(errs), errs)
	}
}

func TestJSONPackWhitespaceTrimmedKeys(t *testing.T) {
	t.Parallel()
	pctx := pipeline.NewPipelineContext()
	pctx.Set("foo", "bar")

	// Keys with surrounding spaces should be trimmed.
	node := jsonPackNode("p", map[string]string{
		"keys":   " foo , ",
		"output": "out",
	})
	h := &handlers.JSONPackHandler{}
	if err := h.Handle(t.Context(), node, pctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	raw := pctx.GetString("out")
	var got map[string]string
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got["foo"] != "bar" {
		t.Errorf("foo = %q, want %q", got["foo"], "bar")
	}
}
