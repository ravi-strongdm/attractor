package handlers_test

import (
	"strings"
	"testing"

	"github.com/ravi-parthasarathy/attractor/pkg/pipeline"
	"github.com/ravi-parthasarathy/attractor/pkg/pipeline/handlers"
)

func execNode(id string, attrs map[string]string) *pipeline.Node {
	return &pipeline.Node{ID: id, Type: pipeline.NodeTypeExec, Attrs: attrs}
}

func TestExecBasic(t *testing.T) {
	t.Parallel()
	pctx := pipeline.NewPipelineContext()
	node := execNode("run", map[string]string{
		"cmd": "echo hello",
	})
	h := &handlers.ExecHandler{}
	if err := h.Handle(t.Context(), node, pctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Default stdout_key is "<nodeID>_stdout".
	got := strings.TrimSpace(pctx.GetString("run_stdout"))
	if got != "hello" {
		t.Errorf("run_stdout = %q, want %q", got, "hello")
	}
}

func TestExecSetsLastOutput(t *testing.T) {
	t.Parallel()
	pctx := pipeline.NewPipelineContext()
	node := execNode("r", map[string]string{"cmd": "echo world"})
	h := &handlers.ExecHandler{}
	if err := h.Handle(t.Context(), node, pctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := strings.TrimSpace(pctx.GetString("last_output"))
	if got != "world" {
		t.Errorf("last_output = %q, want %q", got, "world")
	}
}

func TestExecCustomStdoutKey(t *testing.T) {
	t.Parallel()
	pctx := pipeline.NewPipelineContext()
	node := execNode("r", map[string]string{
		"cmd":        "echo custom",
		"stdout_key": "my_out",
	})
	h := &handlers.ExecHandler{}
	if err := h.Handle(t.Context(), node, pctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := strings.TrimSpace(pctx.GetString("my_out"))
	if got != "custom" {
		t.Errorf("my_out = %q, want %q", got, "custom")
	}
}

func TestExecStderr(t *testing.T) {
	t.Parallel()
	pctx := pipeline.NewPipelineContext()
	node := execNode("r", map[string]string{
		"cmd":            "echo err >&2; exit 0",
		"stderr_key":     "my_err",
		"fail_on_error":  "false",
	})
	h := &handlers.ExecHandler{}
	if err := h.Handle(t.Context(), node, pctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := strings.TrimSpace(pctx.GetString("my_err"))
	if got != "err" {
		t.Errorf("stderr = %q, want %q", got, "err")
	}
}

func TestExecExitCode(t *testing.T) {
	t.Parallel()
	pctx := pipeline.NewPipelineContext()
	node := execNode("r", map[string]string{
		"cmd":            "exit 42",
		"exit_code_key":  "code",
		"fail_on_error":  "false",
	})
	h := &handlers.ExecHandler{}
	if err := h.Handle(t.Context(), node, pctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := pctx.GetString("code"); got != "42" {
		t.Errorf("exit code = %q, want %q", got, "42")
	}
}

func TestExecFailOnError(t *testing.T) {
	t.Parallel()
	pctx := pipeline.NewPipelineContext()
	node := execNode("r", map[string]string{
		"cmd": "exit 1",
	})
	h := &handlers.ExecHandler{}
	if err := h.Handle(t.Context(), node, pctx); err == nil {
		t.Fatal("expected error for non-zero exit code")
	}
}

func TestExecNoFailOnError(t *testing.T) {
	t.Parallel()
	pctx := pipeline.NewPipelineContext()
	node := execNode("r", map[string]string{
		"cmd":           "exit 1",
		"fail_on_error": "false",
	})
	h := &handlers.ExecHandler{}
	if err := h.Handle(t.Context(), node, pctx); err != nil {
		t.Fatalf("expected no error with fail_on_error=false, got: %v", err)
	}
}

func TestExecTimeout(t *testing.T) {
	t.Parallel()
	pctx := pipeline.NewPipelineContext()
	node := execNode("r", map[string]string{
		"cmd":     "sleep 10",
		"timeout": "50ms",
	})
	h := &handlers.ExecHandler{}
	if err := h.Handle(t.Context(), node, pctx); err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestExecTemplateCmd(t *testing.T) {
	t.Parallel()
	pctx := pipeline.NewPipelineContext()
	pctx.Set("name", "world")
	node := execNode("r", map[string]string{
		"cmd":        "echo hello {{.name}}",
		"stdout_key": "out",
	})
	h := &handlers.ExecHandler{}
	if err := h.Handle(t.Context(), node, pctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := strings.TrimSpace(pctx.GetString("out"))
	if got != "hello world" {
		t.Errorf("out = %q, want %q", got, "hello world")
	}
}

func TestExecMissingCmdAttr(t *testing.T) {
	t.Parallel()
	pctx := pipeline.NewPipelineContext()
	node := execNode("r", map[string]string{})
	h := &handlers.ExecHandler{}
	if err := h.Handle(t.Context(), node, pctx); err == nil {
		t.Fatal("expected error for missing cmd attr")
	}
}

func TestExecValidatorCatchesMissingCmd(t *testing.T) {
	t.Parallel()
	node := &pipeline.Node{
		ID:    "r",
		Type:  pipeline.NodeTypeExec,
		Attrs: map[string]string{},
	}
	errs := pipeline.ValidateNode(node)
	if len(errs) == 0 {
		t.Fatal("expected validator error for missing cmd attr")
	}
}

func TestExecZeroExitCode(t *testing.T) {
	t.Parallel()
	pctx := pipeline.NewPipelineContext()
	node := execNode("r", map[string]string{
		"cmd":           "exit 0",
		"exit_code_key": "code",
	})
	h := &handlers.ExecHandler{}
	if err := h.Handle(t.Context(), node, pctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := pctx.GetString("code"); got != "0" {
		t.Errorf("exit code = %q, want %q", got, "0")
	}
}
