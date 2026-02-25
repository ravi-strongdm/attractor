package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ReadFileTool reads a file relative to the working directory.
type ReadFileTool struct {
	workdir string
}

// NewReadFileTool creates a ReadFileTool sandboxed to workdir.
func NewReadFileTool(workdir string) *ReadFileTool {
	return &ReadFileTool{workdir: workdir}
}

func (t *ReadFileTool) Name() string        { return "read_file" }
func (t *ReadFileTool) Description() string { return "Read the contents of a file." }
func (t *ReadFileTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"path":{"type":"string","description":"File path relative to the working directory"}},"required":["path"]}`)
}

func (t *ReadFileTool) Execute(_ context.Context, input json.RawMessage) (string, error) {
	var params struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("read_file: invalid input: %w", err)
	}
	safe, err := safePath(t.workdir, params.Path)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(safe)
	if err != nil {
		return "", fmt.Errorf("read_file: %w", err)
	}
	return string(data), nil
}

// safePath resolves a path under workdir and rejects path traversal attempts.
// Any path that resolves outside the workdir tree is rejected with an error.
func safePath(workdir, rel string) (string, error) {
	// Compute the absolute, cleaned path.
	abs := filepath.Clean(filepath.Join(workdir, rel))
	wdClean := filepath.Clean(workdir)
	// abs must be exactly wdClean or a descendant of it.
	if abs != wdClean && !strings.HasPrefix(abs, wdClean+string(filepath.Separator)) {
		return "", fmt.Errorf("path traversal detected: %q resolves outside workdir", rel)
	}
	return abs, nil
}
