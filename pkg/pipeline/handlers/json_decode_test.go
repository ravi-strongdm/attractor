package handlers_test

import (
	"testing"

	"github.com/ravi-parthasarathy/attractor/pkg/pipeline"
	"github.com/ravi-parthasarathy/attractor/pkg/pipeline/handlers"
)

func jsonDecodeNode(id string, attrs map[string]string) *pipeline.Node {
	return &pipeline.Node{ID: id, Type: pipeline.NodeTypeJSONDecode, Attrs: attrs}
}

func TestJSONDecodeBasic(t *testing.T) {
	t.Parallel()
	pctx := pipeline.NewPipelineContext()
	pctx.Set("data", `{"name":"Alice","age":"30"}`)

	node := jsonDecodeNode("d", map[string]string{"source": "data"})
	h := &handlers.JSONDecodeHandler{}
	if err := h.Handle(t.Context(), node, pctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := pctx.GetString("name"); got != "Alice" {
		t.Errorf("name = %q, want %q", got, "Alice")
	}
	if got := pctx.GetString("age"); got != "30" {
		t.Errorf("age = %q, want %q", got, "30")
	}
}

func TestJSONDecodePrefix(t *testing.T) {
	t.Parallel()
	pctx := pipeline.NewPipelineContext()
	pctx.Set("resp", `{"model":"gpt-4","tokens":"100"}`)

	node := jsonDecodeNode("d", map[string]string{"source": "resp", "prefix": "llm_"})
	h := &handlers.JSONDecodeHandler{}
	if err := h.Handle(t.Context(), node, pctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := pctx.GetString("llm_model"); got != "gpt-4" {
		t.Errorf("llm_model = %q, want %q", got, "gpt-4")
	}
	if got := pctx.GetString("llm_tokens"); got != "100" {
		t.Errorf("llm_tokens = %q, want %q", got, "100")
	}
	// Unprefixed keys must NOT be set.
	if got := pctx.GetString("model"); got != "" {
		t.Errorf("unprefixed key 'model' should not be set, got %q", got)
	}
}

func TestJSONDecodeEmptySource(t *testing.T) {
	t.Parallel()
	// Empty string in context → no-op, no error.
	pctx := pipeline.NewPipelineContext()
	// "data" key not set → GetString returns "".

	node := jsonDecodeNode("d", map[string]string{"source": "data"})
	h := &handlers.JSONDecodeHandler{}
	if err := h.Handle(t.Context(), node, pctx); err != nil {
		t.Fatalf("expected no-op for empty source, got: %v", err)
	}
}

func TestJSONDecodeNestedObject(t *testing.T) {
	t.Parallel()
	pctx := pipeline.NewPipelineContext()
	pctx.Set("data", `{"meta":{"author":"Bob","year":2024},"title":"Test"}`)

	node := jsonDecodeNode("d", map[string]string{"source": "data"})
	h := &handlers.JSONDecodeHandler{}
	if err := h.Handle(t.Context(), node, pctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Nested object should be re-marshalled to a JSON string.
	meta := pctx.GetString("meta")
	if meta == "" {
		t.Error("expected 'meta' key to be set")
	}
	// Plain string field should still work.
	if got := pctx.GetString("title"); got != "Test" {
		t.Errorf("title = %q, want %q", got, "Test")
	}
}

func TestJSONDecodeNonObject(t *testing.T) {
	t.Parallel()
	pctx := pipeline.NewPipelineContext()
	pctx.Set("data", `["a","b","c"]`)

	node := jsonDecodeNode("d", map[string]string{"source": "data"})
	h := &handlers.JSONDecodeHandler{}
	if err := h.Handle(t.Context(), node, pctx); err == nil {
		t.Fatal("expected error for JSON array at top level")
	}
}

func TestJSONDecodeInvalidJSON(t *testing.T) {
	t.Parallel()
	pctx := pipeline.NewPipelineContext()
	pctx.Set("data", `not-json`)

	node := jsonDecodeNode("d", map[string]string{"source": "data"})
	h := &handlers.JSONDecodeHandler{}
	if err := h.Handle(t.Context(), node, pctx); err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestJSONDecodeMissingSourceAttr(t *testing.T) {
	t.Parallel()
	pctx := pipeline.NewPipelineContext()
	node := jsonDecodeNode("d", map[string]string{})
	h := &handlers.JSONDecodeHandler{}
	if err := h.Handle(t.Context(), node, pctx); err == nil {
		t.Fatal("expected error for missing 'source' attribute")
	}
}

func TestJSONDecodeValidatorCatchesMissingSource(t *testing.T) {
	t.Parallel()
	node := &pipeline.Node{
		ID:    "d",
		Type:  pipeline.NodeTypeJSONDecode,
		Attrs: map[string]string{},
	}
	errs := pipeline.ValidateNode(node)
	if len(errs) == 0 {
		t.Fatal("expected validator error for missing 'source' attr")
	}
}

func TestJSONDecodeNumericValues(t *testing.T) {
	t.Parallel()
	pctx := pipeline.NewPipelineContext()
	// JSON numbers are float64 after json.Unmarshal into any.
	pctx.Set("data", `{"count":42,"ratio":0.5,"active":true}`)

	node := jsonDecodeNode("d", map[string]string{"source": "data"})
	h := &handlers.JSONDecodeHandler{}
	if err := h.Handle(t.Context(), node, pctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// count stored as "42" (float64 → string).
	if got := pctx.GetString("count"); got == "" {
		t.Error("expected 'count' key to be set")
	}
	if got := pctx.GetString("active"); got != "true" {
		t.Errorf("active = %q, want %q", got, "true")
	}
}
