package handlers_test

import (
	"testing"

	"github.com/ravi-parthasarathy/attractor/pkg/pipeline"
	"github.com/ravi-parthasarathy/attractor/pkg/pipeline/handlers"
)

func regexNode(id string, attrs map[string]string) *pipeline.Node {
	return &pipeline.Node{ID: id, Type: pipeline.NodeTypeRegex, Attrs: attrs}
}

func TestRegexWholeMatch(t *testing.T) {
	t.Parallel()
	pctx := pipeline.NewPipelineContext()
	pctx.Set("input", "version: v1.2.3")
	node := regexNode("r", map[string]string{
		"source":  "input",
		"pattern": `v\d+\.\d+\.\d+`,
		"key":     "ver",
	})
	h := &handlers.RegexHandler{}
	if err := h.Handle(t.Context(), node, pctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := pctx.GetString("ver"); got != "v1.2.3" {
		t.Errorf("ver = %q, want %q", got, "v1.2.3")
	}
}

func TestRegexCaptureGroup(t *testing.T) {
	t.Parallel()
	pctx := pipeline.NewPipelineContext()
	pctx.Set("input", "version: v1.2.3")
	node := regexNode("r", map[string]string{
		"source":  "input",
		"pattern": `v(\d+\.\d+\.\d+)`,
		"key":     "ver",
		"group":   "1",
	})
	h := &handlers.RegexHandler{}
	if err := h.Handle(t.Context(), node, pctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := pctx.GetString("ver"); got != "1.2.3" {
		t.Errorf("ver = %q, want %q", got, "1.2.3")
	}
}

func TestRegexNoMatch(t *testing.T) {
	t.Parallel()
	pctx := pipeline.NewPipelineContext()
	pctx.Set("input", "no version here")
	node := regexNode("r", map[string]string{
		"source":   "input",
		"pattern":  `v\d+\.\d+\.\d+`,
		"key":      "ver",
		"no_match": "unknown",
	})
	h := &handlers.RegexHandler{}
	if err := h.Handle(t.Context(), node, pctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := pctx.GetString("ver"); got != "unknown" {
		t.Errorf("ver = %q, want %q", got, "unknown")
	}
}

func TestRegexNoMatchDefaultEmpty(t *testing.T) {
	t.Parallel()
	pctx := pipeline.NewPipelineContext()
	pctx.Set("input", "nothing")
	node := regexNode("r", map[string]string{
		"source":  "input",
		"pattern": `v\d+`,
		"key":     "ver",
	})
	h := &handlers.RegexHandler{}
	if err := h.Handle(t.Context(), node, pctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := pctx.GetString("ver"); got != "" {
		t.Errorf("ver = %q, want empty string", got)
	}
}

func TestRegexInvalidPattern(t *testing.T) {
	t.Parallel()
	pctx := pipeline.NewPipelineContext()
	pctx.Set("input", "test")
	node := regexNode("r", map[string]string{
		"source":  "input",
		"pattern": `[invalid`,
		"key":     "out",
	})
	h := &handlers.RegexHandler{}
	if err := h.Handle(t.Context(), node, pctx); err == nil {
		t.Fatal("expected error for invalid pattern")
	}
}

func TestRegexGroupOutOfRange(t *testing.T) {
	t.Parallel()
	pctx := pipeline.NewPipelineContext()
	pctx.Set("input", "hello")
	node := regexNode("r", map[string]string{
		"source":  "input",
		"pattern": `hello`,
		"key":     "out",
		"group":   "5",
	})
	h := &handlers.RegexHandler{}
	if err := h.Handle(t.Context(), node, pctx); err == nil {
		t.Fatal("expected error for out-of-range group")
	}
}

func TestRegexValidatorCatchesMissingAttrs(t *testing.T) {
	t.Parallel()
	node := &pipeline.Node{
		ID:    "r",
		Type:  pipeline.NodeTypeRegex,
		Attrs: map[string]string{},
	}
	errs := pipeline.ValidateNode(node)
	if len(errs) < 3 {
		t.Fatalf("expected 3 errors for missing source/pattern/key, got %d: %v", len(errs), errs)
	}
}
