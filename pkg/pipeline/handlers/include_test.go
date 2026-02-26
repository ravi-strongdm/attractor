package handlers_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ravi-parthasarathy/attractor/pkg/pipeline"
	"github.com/ravi-parthasarathy/attractor/pkg/pipeline/handlers"
)

// minimalRegistry builds a registry with just the handlers needed for include tests.
func minimalRegistry(_, _ string) pipeline.HandlerRegistry {
	reg := handlers.NewRegistry()
	reg.Register("start", &handlers.StartHandler{})
	reg.Register("exit", &handlers.ExitHandler{})
	reg.Register("set", &handlers.SetHandler{})
	return reg
}

func writeSubPipeline(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write sub-pipeline: %v", err)
	}
	return path
}

const subPipelineDOT = `digraph sub {
    start [type=start]
    set_val [type=set key="sub_ran" value="yes"]
    done [type=exit]
    start -> set_val -> done
}`

func TestIncludeBasic(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	subPath := writeSubPipeline(t, dir, "sub.dot", subPipelineDOT)

	pctx := pipeline.NewPipelineContext()
	node := &pipeline.Node{
		ID:    "inc",
		Type:  pipeline.NodeTypeInclude,
		Attrs: map[string]string{"path": subPath},
	}
	h := &handlers.IncludeHandler{RegistryBuilder: minimalRegistry}
	if err := h.Handle(t.Context(), node, pctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestIncludeContextShared(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	subPath := writeSubPipeline(t, dir, "sub.dot", subPipelineDOT)

	pctx := pipeline.NewPipelineContext()
	node := &pipeline.Node{
		ID:    "inc",
		Type:  pipeline.NodeTypeInclude,
		Attrs: map[string]string{"path": subPath},
	}
	h := &handlers.IncludeHandler{RegistryBuilder: minimalRegistry}
	if err := h.Handle(t.Context(), node, pctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Sub-pipeline sets sub_ran="yes" — must be visible in parent context.
	if got := pctx.GetString("sub_ran"); got != "yes" {
		t.Errorf("sub_ran = %q, want %q", got, "yes")
	}
}

func TestIncludePathTemplate(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	subPath := writeSubPipeline(t, dir, "sub.dot", subPipelineDOT)

	pctx := pipeline.NewPipelineContext()
	pctx.Set("sub_dir", dir)
	node := &pipeline.Node{
		ID:    "inc",
		Type:  pipeline.NodeTypeInclude,
		Attrs: map[string]string{"path": "{{.sub_dir}}/sub.dot"},
	}
	_ = subPath
	h := &handlers.IncludeHandler{RegistryBuilder: minimalRegistry}
	if err := h.Handle(t.Context(), node, pctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := pctx.GetString("sub_ran"); got != "yes" {
		t.Errorf("sub_ran = %q, want %q", got, "yes")
	}
}

func TestIncludeMissingFile(t *testing.T) {
	t.Parallel()
	pctx := pipeline.NewPipelineContext()
	node := &pipeline.Node{
		ID:    "inc",
		Type:  pipeline.NodeTypeInclude,
		Attrs: map[string]string{"path": "/nonexistent/path/sub.dot"},
	}
	h := &handlers.IncludeHandler{RegistryBuilder: minimalRegistry}
	if err := h.Handle(t.Context(), node, pctx); err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestIncludeInvalidDOT(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	subPath := writeSubPipeline(t, dir, "bad.dot", `this is not valid dot syntax !!!`)

	pctx := pipeline.NewPipelineContext()
	node := &pipeline.Node{
		ID:    "inc",
		Type:  pipeline.NodeTypeInclude,
		Attrs: map[string]string{"path": subPath},
	}
	h := &handlers.IncludeHandler{RegistryBuilder: minimalRegistry}
	if err := h.Handle(t.Context(), node, pctx); err == nil {
		t.Fatal("expected error for invalid DOT")
	}
}

func TestIncludeMissingPathAttr(t *testing.T) {
	t.Parallel()
	pctx := pipeline.NewPipelineContext()
	node := &pipeline.Node{
		ID:    "inc",
		Type:  pipeline.NodeTypeInclude,
		Attrs: map[string]string{},
	}
	h := &handlers.IncludeHandler{RegistryBuilder: minimalRegistry}
	if err := h.Handle(t.Context(), node, pctx); err == nil {
		t.Fatal("expected error for missing path attr")
	}
}

func TestIncludeValidatorCatchesMissingPath(t *testing.T) {
	t.Parallel()
	node := &pipeline.Node{
		ID:    "inc",
		Type:  pipeline.NodeTypeInclude,
		Attrs: map[string]string{},
	}
	errs := pipeline.ValidateNode(node)
	if len(errs) == 0 {
		t.Fatal("expected validator error for missing path attr")
	}
}
