package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// WriteFileTool writes content to a file relative to the working directory.
type WriteFileTool struct {
	workdir string
}

// NewWriteFileTool creates a WriteFileTool sandboxed to workdir.
func NewWriteFileTool(workdir string) *WriteFileTool {
	return &WriteFileTool{workdir: workdir}
}

func (t *WriteFileTool) Name() string        { return "write_file" }
func (t *WriteFileTool) Description() string { return "Write content to a file." }
func (t *WriteFileTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"},"content":{"type":"string"}},"required":["path","content"]}`)
}

func (t *WriteFileTool) Execute(_ context.Context, input json.RawMessage) (string, error) {
	var params struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("write_file: invalid input: %w", err)
	}
	safe, err := safePath(t.workdir, params.Path)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(safe), 0o755); err != nil {
		return "", fmt.Errorf("write_file: mkdir: %w", err)
	}
	if err := os.WriteFile(safe, []byte(params.Content), 0o644); err != nil {
		return "", fmt.Errorf("write_file: %w", err)
	}
	return fmt.Sprintf("wrote %d bytes to %s", len(params.Content), params.Path), nil
}
