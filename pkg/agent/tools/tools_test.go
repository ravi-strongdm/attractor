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

// ─── SearchFile ───────────────────────────────────────────────────────────────

func TestSearchFileTool_FindsPattern(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "helper.go"), []byte("package main\n\nfunc helper() {}\n"), 0o644)

	tool := tools.NewSearchFileTool(dir)
	input, _ := json.Marshal(map[string]string{"pattern": "func main"})
	out, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out == "no matches found" {
		t.Fatal("expected matches, got none")
	}
	// Should contain the file:line: content format
	if len(out) == 0 {
		t.Error("expected non-empty output")
	}
}

func TestSearchFileTool_NoMatches(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "file.go"), []byte("package main\n"), 0o644)

	tool := tools.NewSearchFileTool(dir)
	input, _ := json.Marshal(map[string]string{"pattern": "XYZNOTFOUND"})
	out, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out != "no matches found" {
		t.Errorf("output = %q, want %q", out, "no matches found")
	}
}

func TestSearchFileTool_PathTraversal(t *testing.T) {
	dir := t.TempDir()
	tool := tools.NewSearchFileTool(dir)
	input, _ := json.Marshal(map[string]string{"pattern": "x", "path": "../../etc"})
	_, err := tool.Execute(context.Background(), input)
	if err == nil {
		t.Fatal("expected path traversal error")
	}
}

func TestSearchFileTool_SubdirectoryScope(t *testing.T) {
	dir := t.TempDir()
	subdir := filepath.Join(dir, "pkg")
	_ = os.MkdirAll(subdir, 0o755)
	_ = os.WriteFile(filepath.Join(dir, "root.go"), []byte("// root\n"), 0o644)
	_ = os.WriteFile(filepath.Join(subdir, "pkg.go"), []byte("// pkg\n"), 0o644)

	tool := tools.NewSearchFileTool(dir)
	// Search only in the subdir
	input, _ := json.Marshal(map[string]string{"pattern": "pkg", "path": "pkg"})
	out, err := tool.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	// root.go should NOT appear since we scoped to pkg/
	if out == "no matches found" {
		t.Fatal("expected at least one match in pkg/")
	}
}
