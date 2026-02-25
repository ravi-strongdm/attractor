package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ListDirTool lists files in a directory relative to the working directory.
type ListDirTool struct {
	workdir string
}

// NewListDirTool creates a ListDirTool sandboxed to workdir.
func NewListDirTool(workdir string) *ListDirTool {
	return &ListDirTool{workdir: workdir}
}

func (t *ListDirTool) Name() string        { return "list_dir" }
func (t *ListDirTool) Description() string { return "List files in a directory." }
func (t *ListDirTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"path":{"type":"string","description":"Directory path relative to working directory (default: '.')"}}}`)
}

func (t *ListDirTool) Execute(_ context.Context, input json.RawMessage) (string, error) {
	var params struct {
		Path string `json:"path"`
	}
	_ = json.Unmarshal(input, &params) // path is optional
	if params.Path == "" {
		params.Path = "."
	}
	safe, err := safePath(t.workdir, params.Path)
	if err != nil {
		return "", err
	}
	entries, err := os.ReadDir(safe)
	if err != nil {
		return "", fmt.Errorf("list_dir: %w", err)
	}
	var sb strings.Builder
	for _, e := range entries {
		if e.IsDir() {
			sb.WriteString(filepath.Join(params.Path, e.Name()) + "/\n")
		} else {
			sb.WriteString(filepath.Join(params.Path, e.Name()) + "\n")
		}
	}
	return sb.String(), nil
}
