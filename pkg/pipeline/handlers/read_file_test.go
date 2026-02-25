package handlers_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ravi-parthasarathy/attractor/pkg/pipeline"
	"github.com/ravi-parthasarathy/attractor/pkg/pipeline/handlers"
)

func TestReadFileOK(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "hello.txt")
	if err := os.WriteFile(path, []byte("hello world"), 0o644); err != nil {
		t.Fatal(err)
	}

	pctx := pipeline.NewPipelineContext()
	node := &pipeline.Node{
		ID:    "load",
		Type:  pipeline.NodeTypeReadFile,
		Attrs: map[string]string{"key": "content", "path": path},
	}
	h := &handlers.ReadFileHandler{}
	if err := h.Handle(t.Context(), node, pctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := pctx.GetString("content"); got != "hello world" {
		t.Errorf("got %q, want %q", got, "hello world")
	}
}

func TestReadFileTemplatePath(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "spec.md")
	if err := os.WriteFile(path, []byte("# spec"), 0o644); err != nil {
		t.Fatal(err)
	}

	pctx := pipeline.NewPipelineContext()
	pctx.Set("dir", dir)
	node := &pipeline.Node{
		ID:    "load",
		Type:  pipeline.NodeTypeReadFile,
		Attrs: map[string]string{"key": "spec", "path": "{{.dir}}/spec.md"},
	}
	h := &handlers.ReadFileHandler{}
	if err := h.Handle(t.Context(), node, pctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := pctx.GetString("spec"); got != "# spec" {
		t.Errorf("got %q, want %q", got, "# spec")
	}
}

func TestReadFileMissingRequired(t *testing.T) {
	t.Parallel()
	pctx := pipeline.NewPipelineContext()
	node := &pipeline.Node{
		ID:    "load",
		Type:  pipeline.NodeTypeReadFile,
		Attrs: map[string]string{"key": "x", "path": "/nonexistent/file.txt"},
	}
	h := &handlers.ReadFileHandler{}
	if err := h.Handle(t.Context(), node, pctx); err == nil {
		t.Fatal("expected error for missing required file, got nil")
	}
}

func TestReadFileMissingOptional(t *testing.T) {
	t.Parallel()
	pctx := pipeline.NewPipelineContext()
	node := &pipeline.Node{
		ID:   "load",
		Type: pipeline.NodeTypeReadFile,
		Attrs: map[string]string{
			"key":      "x",
			"path":     "/nonexistent/file.txt",
			"required": "false",
		},
	}
	h := &handlers.ReadFileHandler{}
	if err := h.Handle(t.Context(), node, pctx); err != nil {
		t.Fatalf("unexpected error for optional missing file: %v", err)
	}
	if got := pctx.GetString("x"); got != "" {
		t.Errorf("expected empty string for missing optional file, got %q", got)
	}
}

func TestReadFileMissingKeyAttr(t *testing.T) {
	t.Parallel()
	pctx := pipeline.NewPipelineContext()
	node := &pipeline.Node{
		ID:    "load",
		Type:  pipeline.NodeTypeReadFile,
		Attrs: map[string]string{"path": "/some/file.txt"},
	}
	h := &handlers.ReadFileHandler{}
	if err := h.Handle(t.Context(), node, pctx); err == nil {
		t.Fatal("expected error for missing key attr")
	}
}

func TestReadFileMissingPathAttr(t *testing.T) {
	t.Parallel()
	pctx := pipeline.NewPipelineContext()
	node := &pipeline.Node{
		ID:    "load",
		Type:  pipeline.NodeTypeReadFile,
		Attrs: map[string]string{"key": "x"},
	}
	h := &handlers.ReadFileHandler{}
	if err := h.Handle(t.Context(), node, pctx); err == nil {
		t.Fatal("expected error for missing path attr")
	}
}
