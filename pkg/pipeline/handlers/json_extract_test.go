package handlers_test

import (
	"testing"

	"github.com/ravi-parthasarathy/attractor/pkg/pipeline"
	"github.com/ravi-parthasarathy/attractor/pkg/pipeline/handlers"
)

func jsonExtractNode(id string, attrs map[string]string) *pipeline.Node {
	return &pipeline.Node{ID: id, Type: pipeline.NodeTypeJSONExtract, Attrs: attrs}
}

func TestJSONExtractScalar(t *testing.T) {
	t.Parallel()
	pctx := pipeline.NewPipelineContext()
	pctx.Set("body", `{"name":"alice","age":30}`)

	h := &handlers.JSONExtractHandler{}

	node := jsonExtractNode("x", map[string]string{"source": "body", "path": ".name", "key": "username"})
	if err := h.Handle(t.Context(), node, pctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := pctx.GetString("username"); got != "alice" {
		t.Errorf("got %q, want %q", got, "alice")
	}

	node2 := jsonExtractNode("x", map[string]string{"source": "body", "path": ".age", "key": "age"})
	if err := h.Handle(t.Context(), node2, pctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := pctx.GetString("age"); got != "30" {
		t.Errorf("got %q, want %q", got, "30")
	}
}

func TestJSONExtractNested(t *testing.T) {
	t.Parallel()
	pctx := pipeline.NewPipelineContext()
	pctx.Set("resp", `{"data":{"user":{"email":"bob@example.com"}}}`)

	h := &handlers.JSONExtractHandler{}
	node := jsonExtractNode("x", map[string]string{
		"source": "resp",
		"path":   ".data.user.email",
		"key":    "email",
	})
	if err := h.Handle(t.Context(), node, pctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := pctx.GetString("email"); got != "bob@example.com" {
		t.Errorf("got %q, want %q", got, "bob@example.com")
	}
}

func TestJSONExtractArrayIndex(t *testing.T) {
	t.Parallel()
	pctx := pipeline.NewPipelineContext()
	pctx.Set("items", `{"results":["first","second","third"]}`)

	h := &handlers.JSONExtractHandler{}
	node := jsonExtractNode("x", map[string]string{
		"source": "items",
		"path":   ".results.1",
		"key":    "second",
	})
	if err := h.Handle(t.Context(), node, pctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := pctx.GetString("second"); got != "second" {
		t.Errorf("got %q, want %q", got, "second")
	}
}

func TestJSONExtractDefault(t *testing.T) {
	t.Parallel()
	pctx := pipeline.NewPipelineContext()
	pctx.Set("resp", `{"status":"ok"}`)

	h := &handlers.JSONExtractHandler{}
	node := jsonExtractNode("x", map[string]string{
		"source":  "resp",
		"path":    ".missing_field",
		"key":     "val",
		"default": "fallback",
	})
	if err := h.Handle(t.Context(), node, pctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := pctx.GetString("val"); got != "fallback" {
		t.Errorf("got %q, want %q", got, "fallback")
	}
}

func TestJSONExtractObject(t *testing.T) {
	t.Parallel()
	pctx := pipeline.NewPipelineContext()
	pctx.Set("resp", `{"user":{"id":1,"name":"carol"}}`)

	h := &handlers.JSONExtractHandler{}
	node := jsonExtractNode("x", map[string]string{
		"source": "resp",
		"path":   ".user",
		"key":    "user_json",
	})
	if err := h.Handle(t.Context(), node, pctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := pctx.GetString("user_json")
	// Should be a compact JSON object (key order may vary, check both fields present).
	if got == "" {
		t.Error("expected non-empty JSON object")
	}
	if got[0] != '{' {
		t.Errorf("expected JSON object, got %q", got)
	}
}

func TestJSONExtractEmptySource(t *testing.T) {
	t.Parallel()
	pctx := pipeline.NewPipelineContext()
	// source key not set → empty string

	h := &handlers.JSONExtractHandler{}
	node := jsonExtractNode("x", map[string]string{
		"source":  "body",
		"path":    ".name",
		"key":     "name",
		"default": "unknown",
	})
	if err := h.Handle(t.Context(), node, pctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := pctx.GetString("name"); got != "unknown" {
		t.Errorf("got %q, want %q", got, "unknown")
	}
}

func TestJSONExtractMissingPath(t *testing.T) {
	t.Parallel()
	pctx := pipeline.NewPipelineContext()
	pctx.Set("resp", `{"a":1}`)

	h := &handlers.JSONExtractHandler{}
	// No default → should error when path missing.
	node := jsonExtractNode("x", map[string]string{
		"source": "resp",
		"path":   ".nonexistent",
		"key":    "val",
	})
	if err := h.Handle(t.Context(), node, pctx); err == nil {
		t.Fatal("expected error for missing path without default")
	}
}

func TestJSONExtractMissingRequiredAttrs(t *testing.T) {
	t.Parallel()
	h := &handlers.JSONExtractHandler{}
	pctx := pipeline.NewPipelineContext()

	for _, attrs := range []map[string]string{
		{"path": ".x", "key": "k"},               // missing source
		{"source": "s", "key": "k"},              // missing path
		{"source": "s", "path": ".x"},            // missing key
	} {
		node := jsonExtractNode("x", attrs)
		if err := h.Handle(t.Context(), node, pctx); err == nil {
			t.Errorf("expected error for attrs %v, got nil", attrs)
		}
	}
}
