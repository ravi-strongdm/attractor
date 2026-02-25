package handlers_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ravi-parthasarathy/attractor/pkg/pipeline"
	"github.com/ravi-parthasarathy/attractor/pkg/pipeline/handlers"
)

func TestWriteFileCreatesFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "out.txt")

	pctx := pipeline.NewPipelineContext()
	node := &pipeline.Node{
		ID:   "save",
		Type: pipeline.NodeTypeWriteFile,
		Attrs: map[string]string{
			"path":    path,
			"content": "hello from pipeline",
		},
	}
	h := &handlers.WriteFileHandler{}
	if err := h.Handle(t.Context(), node, pctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("file not created: %v", err)
	}
	if string(got) != "hello from pipeline" {
		t.Errorf("got %q, want %q", got, "hello from pipeline")
	}
}

func TestWriteFileTemplatePath(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	pctx := pipeline.NewPipelineContext()
	pctx.Set("dir", dir)
	pctx.Set("name", "result")
	node := &pipeline.Node{
		ID:   "save",
		Type: pipeline.NodeTypeWriteFile,
		Attrs: map[string]string{
			"path":    "{{.dir}}/{{.name}}.txt",
			"content": "dynamic content",
		},
	}
	h := &handlers.WriteFileHandler{}
	if err := h.Handle(t.Context(), node, pctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(dir, "result.txt"))
	if err != nil {
		t.Fatalf("file not created: %v", err)
	}
	if string(got) != "dynamic content" {
		t.Errorf("got %q, want %q", got, "dynamic content")
	}
}

func TestWriteFileCreatesParentDirs(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "a", "b", "c", "out.txt")

	pctx := pipeline.NewPipelineContext()
	node := &pipeline.Node{
		ID:   "save",
		Type: pipeline.NodeTypeWriteFile,
		Attrs: map[string]string{
			"path":    path,
			"content": "nested",
		},
	}
	h := &handlers.WriteFileHandler{}
	if err := h.Handle(t.Context(), node, pctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("file not created: %v", err)
	}
	if string(got) != "nested" {
		t.Errorf("got %q, want %q", got, "nested")
	}
}

func TestWriteFileAppend(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "log.txt")

	pctx := pipeline.NewPipelineContext()
	h := &handlers.WriteFileHandler{}

	for _, line := range []string{"line1\n", "line2\n", "line3\n"} {
		node := &pipeline.Node{
			ID:   "save",
			Type: pipeline.NodeTypeWriteFile,
			Attrs: map[string]string{
				"path":    path,
				"content": line,
				"append":  "true",
			},
		}
		if err := h.Handle(t.Context(), node, pctx); err != nil {
			t.Fatalf("append write error: %v", err)
		}
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("file not readable: %v", err)
	}
	want := "line1\nline2\nline3\n"
	if string(got) != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestWriteFileMode(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "secret.txt")

	pctx := pipeline.NewPipelineContext()
	node := &pipeline.Node{
		ID:   "save",
		Type: pipeline.NodeTypeWriteFile,
		Attrs: map[string]string{
			"path":    path,
			"content": "secret",
			"mode":    "0600",
		},
	}
	h := &handlers.WriteFileHandler{}
	if err := h.Handle(t.Context(), node, pctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat failed: %v", err)
	}
	// Check only the permission bits (mask out file type bits).
	if got := info.Mode().Perm(); got != 0o600 {
		t.Errorf("mode: got %o, want %o", got, 0o600)
	}
}

func TestReadWriteRoundtrip(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "round.txt")
	original := "roundtrip content"

	pctx := pipeline.NewPipelineContext()

	// Write
	wnode := &pipeline.Node{
		ID:   "save",
		Type: pipeline.NodeTypeWriteFile,
		Attrs: map[string]string{
			"path":    path,
			"content": original,
		},
	}
	wh := &handlers.WriteFileHandler{}
	if err := wh.Handle(t.Context(), wnode, pctx); err != nil {
		t.Fatalf("write error: %v", err)
	}

	// Read back
	rnode := &pipeline.Node{
		ID:   "load",
		Type: pipeline.NodeTypeReadFile,
		Attrs: map[string]string{
			"key":  "roundtrip",
			"path": path,
		},
	}
	rh := &handlers.ReadFileHandler{}
	if err := rh.Handle(t.Context(), rnode, pctx); err != nil {
		t.Fatalf("read error: %v", err)
	}
	if got := pctx.GetString("roundtrip"); got != original {
		t.Errorf("roundtrip: got %q, want %q", got, original)
	}
}

func TestWriteFileMissingPathAttr(t *testing.T) {
	t.Parallel()
	pctx := pipeline.NewPipelineContext()
	node := &pipeline.Node{
		ID:    "save",
		Type:  pipeline.NodeTypeWriteFile,
		Attrs: map[string]string{"content": "data"},
	}
	h := &handlers.WriteFileHandler{}
	if err := h.Handle(t.Context(), node, pctx); err == nil {
		t.Fatal("expected error for missing path attr")
	}
}

func TestWriteFileMissingContentAttr(t *testing.T) {
	t.Parallel()
	pctx := pipeline.NewPipelineContext()
	node := &pipeline.Node{
		ID:    "save",
		Type:  pipeline.NodeTypeWriteFile,
		Attrs: map[string]string{"path": "/tmp/x.txt"},
	}
	h := &handlers.WriteFileHandler{}
	if err := h.Handle(t.Context(), node, pctx); err == nil {
		t.Fatal("expected error for missing content attr")
	}
}
