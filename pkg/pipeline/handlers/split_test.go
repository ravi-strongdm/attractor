package handlers_test

import (
	"encoding/json"
	"testing"

	"github.com/ravi-parthasarathy/attractor/pkg/pipeline"
	"github.com/ravi-parthasarathy/attractor/pkg/pipeline/handlers"
)

func splitNode(id string, attrs map[string]string) *pipeline.Node {
	return &pipeline.Node{ID: id, Type: pipeline.NodeTypeSplit, Attrs: attrs}
}

func unmarshalStrings(t *testing.T, s string) []string {
	t.Helper()
	var out []string
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		t.Fatalf("unmarshal %q: %v", s, err)
	}
	return out
}

func TestSplitNewline(t *testing.T) {
	t.Parallel()
	pctx := pipeline.NewPipelineContext()
	pctx.Set("raw", "alpha\nbeta\ngamma")

	h := &handlers.SplitHandler{}
	node := splitNode("s", map[string]string{"source": "raw", "key": "parts"})
	if err := h.Handle(t.Context(), node, pctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := unmarshalStrings(t, pctx.GetString("parts"))
	want := []string{"alpha", "beta", "gamma"}
	if len(got) != len(want) {
		t.Fatalf("len=%d want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("[%d] got %q, want %q", i, got[i], want[i])
		}
	}
}

func TestSplitCustomSep(t *testing.T) {
	t.Parallel()
	pctx := pipeline.NewPipelineContext()
	pctx.Set("raw", "a,b,c")

	h := &handlers.SplitHandler{}
	node := splitNode("s", map[string]string{"source": "raw", "key": "parts", "sep": ","})
	if err := h.Handle(t.Context(), node, pctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := unmarshalStrings(t, pctx.GetString("parts"))
	if len(got) != 3 || got[0] != "a" || got[1] != "b" || got[2] != "c" {
		t.Errorf("got %v", got)
	}
}

func TestSplitTrim(t *testing.T) {
	t.Parallel()
	pctx := pipeline.NewPipelineContext()
	pctx.Set("raw", "  alpha  \n\n  beta  \n\n")

	h := &handlers.SplitHandler{}
	node := splitNode("s", map[string]string{"source": "raw", "key": "parts", "trim": "true"})
	if err := h.Handle(t.Context(), node, pctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := unmarshalStrings(t, pctx.GetString("parts"))
	if len(got) != 2 || got[0] != "alpha" || got[1] != "beta" {
		t.Errorf("got %v, want [alpha beta]", got)
	}
}

func TestSplitEmptyNoTrim(t *testing.T) {
	t.Parallel()
	pctx := pipeline.NewPipelineContext()
	pctx.Set("raw", "")

	h := &handlers.SplitHandler{}
	node := splitNode("s", map[string]string{"source": "raw", "key": "parts"})
	if err := h.Handle(t.Context(), node, pctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// strings.Split("", "\n") → [""] — one empty element
	got := unmarshalStrings(t, pctx.GetString("parts"))
	if len(got) != 1 || got[0] != "" {
		t.Errorf("got %v, want [\"\"]", got)
	}
}

func TestSplitEmptyTrim(t *testing.T) {
	t.Parallel()
	pctx := pipeline.NewPipelineContext()
	pctx.Set("raw", "")

	h := &handlers.SplitHandler{}
	node := splitNode("s", map[string]string{"source": "raw", "key": "parts", "trim": "true"})
	if err := h.Handle(t.Context(), node, pctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// trim=true drops the empty element
	got := unmarshalStrings(t, pctx.GetString("parts"))
	if len(got) != 0 {
		t.Errorf("got %v, want []", got)
	}
}

func TestSplitMissingSourceAttr(t *testing.T) {
	t.Parallel()
	pctx := pipeline.NewPipelineContext()
	node := splitNode("s", map[string]string{"key": "parts"})
	h := &handlers.SplitHandler{}
	if err := h.Handle(t.Context(), node, pctx); err == nil {
		t.Fatal("expected error for missing source attr")
	}
}

func TestSplitMissingKeyAttr(t *testing.T) {
	t.Parallel()
	pctx := pipeline.NewPipelineContext()
	node := splitNode("s", map[string]string{"source": "raw"})
	h := &handlers.SplitHandler{}
	if err := h.Handle(t.Context(), node, pctx); err == nil {
		t.Fatal("expected error for missing key attr")
	}
}
