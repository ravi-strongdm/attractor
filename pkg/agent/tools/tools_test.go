package tools_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/ravi-parthasarathy/attractor/pkg/agent/tools"
)

func TestRegistry_GetMissing(t *testing.T) {
	reg := tools.NewRegistry()
	_, err := reg.Get("nonexistent")
	if err == nil {
		t.Fatal("expected error for missing tool")
	}
}

func TestRegistry_List(t *testing.T) {
	reg := tools.NewRegistry()
	dir := t.TempDir()
	reg.Register(tools.NewReadFileTool(dir))
	reg.Register(tools.NewWriteFileTool(dir))
	all := reg.All()
	if len(all) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(all))
	}
}

// ─── ReadFile ─────────────────────────────────────────────────────────────────

func TestReadFileTool(t *testing.T) {
	dir := t.TempDir()
	content := "hello world\n"
	if err := os.WriteFile(filepath.Join(dir, "test.txt"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	tool := tools.NewReadFileTool(dir)
	input, _ := json.Marshal(map[string]string{"path": "test.txt"})
	out, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out != content {
		t.Errorf("output = %q, want %q", out, content)
	}
}

func TestReadFileTool_PathTraversal(t *testing.T) {
	dir := t.TempDir()
	tool := tools.NewReadFileTool(dir)
	input, _ := json.Marshal(map[string]string{"path": "../../etc/passwd"})
	_, err := tool.Execute(context.Background(), input)
	if err == nil {
		t.Fatal("expected path traversal error")
	}
}

func TestReadFileTool_Missing(t *testing.T) {
	dir := t.TempDir()
	tool := tools.NewReadFileTool(dir)
	input, _ := json.Marshal(map[string]string{"path": "nofile.txt"})
	_, err := tool.Execute(context.Background(), input)
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

// ─── WriteFile ────────────────────────────────────────────────────────────────

func TestWriteFileTool(t *testing.T) {
	dir := t.TempDir()
	tool := tools.NewWriteFileTool(dir)
	input, _ := json.Marshal(map[string]string{"path": "out.txt", "content": "data"})
	_, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	got, _ := os.ReadFile(filepath.Join(dir, "out.txt"))
	if string(got) != "data" {
		t.Errorf("file content = %q, want %q", string(got), "data")
	}
}

func TestWriteFileTool_PathTraversal(t *testing.T) {
	dir := t.TempDir()
	tool := tools.NewWriteFileTool(dir)
	input, _ := json.Marshal(map[string]string{"path": "../evil.txt", "content": "x"})
	_, err := tool.Execute(context.Background(), input)
	if err == nil {
		t.Fatal("expected path traversal error")
	}
}

// ─── ListDir ─────────────────────────────────────────────────────────────────

func TestListDirTool(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "a.go"), []byte(""), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "b.go"), []byte(""), 0o644)

	tool := tools.NewListDirTool(dir)
	input, _ := json.Marshal(map[string]string{"path": "."})
	out, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out == "" {
		t.Error("expected non-empty directory listing")
	}
}

// ─── RunCommand ───────────────────────────────────────────────────────────────

func TestRunCommandTool(t *testing.T) {
	dir := t.TempDir()
	tool := tools.NewRunCommandTool(dir)
	input, _ := json.Marshal(map[string]string{"command": "echo hello"})
	out, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out != "hello\n" {
		t.Errorf("output = %q, want %q", out, "hello\n")
	}
}
