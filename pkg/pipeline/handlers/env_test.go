package handlers_test

import (
	"strings"
	"testing"

	"github.com/ravi-parthasarathy/attractor/pkg/pipeline"
	"github.com/ravi-parthasarathy/attractor/pkg/pipeline/handlers"
)

func TestEnvPresent(t *testing.T) {
	t.Setenv("TEST_ENV_VAR_PRESENT", "hello")
	pctx := pipeline.NewPipelineContext()
	node := &pipeline.Node{
		ID:    "load",
		Type:  pipeline.NodeTypeEnv,
		Attrs: map[string]string{"key": "greeting", "from": "TEST_ENV_VAR_PRESENT"},
	}
	h := &handlers.EnvHandler{}
	if err := h.Handle(t.Context(), node, pctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := pctx.GetString("greeting"); got != "hello" {
		t.Errorf("got %q, want %q", got, "hello")
	}
}

func TestEnvDefault(t *testing.T) {
	t.Setenv("TEST_ENV_VAR_ABSENT", "") // ensure unset
	pctx := pipeline.NewPipelineContext()
	node := &pipeline.Node{
		ID:   "load",
		Type: pipeline.NodeTypeEnv,
		Attrs: map[string]string{
			"key":     "model",
			"from":    "TEST_ENV_VAR_ABSENT",
			"default": "anthropic:claude-sonnet-4-6",
		},
	}
	h := &handlers.EnvHandler{}
	if err := h.Handle(t.Context(), node, pctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := pctx.GetString("model"); got != "anthropic:claude-sonnet-4-6" {
		t.Errorf("got %q, want default", got)
	}
}

func TestEnvRequiredMissing(t *testing.T) {
	t.Setenv("TEST_ENV_VAR_REQUIRED", "") // ensure unset
	pctx := pipeline.NewPipelineContext()
	node := &pipeline.Node{
		ID:   "load",
		Type: pipeline.NodeTypeEnv,
		Attrs: map[string]string{
			"key":      "secret",
			"from":     "TEST_ENV_VAR_REQUIRED",
			"required": "true",
		},
	}
	h := &handlers.EnvHandler{}
	err := h.Handle(t.Context(), node, pctx)
	if err == nil {
		t.Fatal("expected error for missing required env var, got nil")
	}
	if !strings.Contains(err.Error(), "TEST_ENV_VAR_REQUIRED") {
		t.Errorf("error should name the missing var: %v", err)
	}
}

func TestEnvRequiredPresent(t *testing.T) {
	t.Setenv("TEST_ENV_VAR_REQ_OK", "mysecret")
	pctx := pipeline.NewPipelineContext()
	node := &pipeline.Node{
		ID:   "load",
		Type: pipeline.NodeTypeEnv,
		Attrs: map[string]string{
			"key":      "secret",
			"from":     "TEST_ENV_VAR_REQ_OK",
			"required": "true",
		},
	}
	h := &handlers.EnvHandler{}
	if err := h.Handle(t.Context(), node, pctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := pctx.GetString("secret"); got != "mysecret" {
		t.Errorf("got %q, want %q", got, "mysecret")
	}
}

func TestEnvMissingKeyAttr(t *testing.T) {
	t.Parallel()
	pctx := pipeline.NewPipelineContext()
	node := &pipeline.Node{
		ID:    "load",
		Type:  pipeline.NodeTypeEnv,
		Attrs: map[string]string{"from": "SOME_VAR"},
	}
	h := &handlers.EnvHandler{}
	if err := h.Handle(t.Context(), node, pctx); err == nil {
		t.Fatal("expected error for missing key attr, got nil")
	}
}

func TestEnvMissingFromAttr(t *testing.T) {
	t.Parallel()
	pctx := pipeline.NewPipelineContext()
	node := &pipeline.Node{
		ID:    "load",
		Type:  pipeline.NodeTypeEnv,
		Attrs: map[string]string{"key": "val"},
	}
	h := &handlers.EnvHandler{}
	if err := h.Handle(t.Context(), node, pctx); err == nil {
		t.Fatal("expected error for missing from attr, got nil")
	}
}
