package handlers_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/ravi-parthasarathy/attractor/pkg/pipeline"
	"github.com/ravi-parthasarathy/attractor/pkg/pipeline/handlers"
)

func humanNode(id string, attrs map[string]string) *pipeline.Node {
	return &pipeline.Node{ID: id, Type: pipeline.NodeTypeHuman, Attrs: attrs}
}

func TestHumanBasicResponse(t *testing.T) {
	t.Parallel()
	pctx := pipeline.NewPipelineContext()
	node := humanNode("ask", map[string]string{"prompt": "What is your name?"})

	var out bytes.Buffer
	h := &handlers.HumanHandler{In: strings.NewReader("Alice\n"), Out: &out}
	if err := h.Handle(t.Context(), node, pctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Default key is <nodeID>_response.
	if got := pctx.GetString("ask_response"); got != "Alice" {
		t.Errorf("ask_response = %q, want %q", got, "Alice")
	}
}

func TestHumanCustomKey(t *testing.T) {
	t.Parallel()
	pctx := pipeline.NewPipelineContext()
	node := humanNode("ask", map[string]string{
		"prompt": "Continue?",
		"key":    "user_choice",
	})

	var out bytes.Buffer
	h := &handlers.HumanHandler{In: strings.NewReader("yes\n"), Out: &out}
	if err := h.Handle(t.Context(), node, pctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := pctx.GetString("user_choice"); got != "yes" {
		t.Errorf("user_choice = %q, want %q", got, "yes")
	}
}

func TestHumanOptionsNumericSelection(t *testing.T) {
	t.Parallel()
	pctx := pipeline.NewPipelineContext()
	node := humanNode("ask", map[string]string{
		"prompt":  "Deploy?",
		"key":     "confirm",
		"options": "yes,no,cancel",
	})

	var out bytes.Buffer
	h := &handlers.HumanHandler{In: strings.NewReader("2\n"), Out: &out}
	if err := h.Handle(t.Context(), node, pctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// "2" → "no" (canonical option text)
	if got := pctx.GetString("confirm"); got != "no" {
		t.Errorf("confirm = %q, want %q", got, "no")
	}
}

func TestHumanOptionsTextSelection(t *testing.T) {
	t.Parallel()
	pctx := pipeline.NewPipelineContext()
	node := humanNode("ask", map[string]string{
		"prompt":  "Deploy?",
		"key":     "confirm",
		"options": "yes,no",
	})

	var out bytes.Buffer
	h := &handlers.HumanHandler{In: strings.NewReader("YES\n"), Out: &out}
	if err := h.Handle(t.Context(), node, pctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Case-insensitive match → canonical "yes"
	if got := pctx.GetString("confirm"); got != "yes" {
		t.Errorf("confirm = %q, want %q", got, "yes")
	}
}

func TestHumanOptionsInvalidThenValid(t *testing.T) {
	t.Parallel()
	pctx := pipeline.NewPipelineContext()
	node := humanNode("ask", map[string]string{
		"prompt":  "Choose:",
		"key":     "choice",
		"options": "a,b,c",
	})

	// First input invalid, second valid.
	var out bytes.Buffer
	h := &handlers.HumanHandler{In: strings.NewReader("x\nb\n"), Out: &out}
	if err := h.Handle(t.Context(), node, pctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := pctx.GetString("choice"); got != "b" {
		t.Errorf("choice = %q, want %q", got, "b")
	}
}

func TestHumanOptionsMenuDisplayed(t *testing.T) {
	t.Parallel()
	pctx := pipeline.NewPipelineContext()
	node := humanNode("ask", map[string]string{
		"prompt":  "Pick one:",
		"key":     "pick",
		"options": "alpha,beta",
	})

	var out bytes.Buffer
	h := &handlers.HumanHandler{In: strings.NewReader("1\n"), Out: &out}
	if err := h.Handle(t.Context(), node, pctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	display := out.String()
	if !strings.Contains(display, "1) alpha") {
		t.Errorf("expected menu item '1) alpha' in output: %s", display)
	}
	if !strings.Contains(display, "2) beta") {
		t.Errorf("expected menu item '2) beta' in output: %s", display)
	}
}
