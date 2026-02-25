package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// PatchFileTool replaces the first occurrence of old_string with new_string
// in a file.  It returns an error if old_string is not found, ensuring the
// agent gets explicit feedback on a failed patch rather than a silent no-op.
type PatchFileTool struct {
	workdir string
}

// NewPatchFileTool creates a PatchFileTool sandboxed to workdir.
func NewPatchFileTool(workdir string) *PatchFileTool {
	return &PatchFileTool{workdir: workdir}
}

func (t *PatchFileTool) Name() string { return "patch_file" }
func (t *PatchFileTool) Description() string {
	return "Replace the first occurrence of old_string with new_string in a file. " +
		"Returns an error if old_string is not found. " +
		"Use read_file first to confirm the exact text you want to replace."
}
func (t *PatchFileTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"path":       {"type": "string",  "description": "File path relative to working directory"},
			"old_string": {"type": "string",  "description": "Exact text to search for (must appear in the file)"},
			"new_string": {"type": "string",  "description": "Replacement text"}
		},
		"required": ["path", "old_string", "new_string"]
	}`)
}

func (t *PatchFileTool) Execute(_ context.Context, input json.RawMessage) (string, error) {
	var params struct {
		Path      string `json:"path"`
		OldString string `json:"old_string"`
		NewString string `json:"new_string"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("patch_file: invalid input: %w", err)
	}
	if params.OldString == "" {
		return "", fmt.Errorf("patch_file: old_string must not be empty")
	}

	safe, err := safePath(t.workdir, params.Path)
	if err != nil {
		return "", err
	}

	data, err := os.ReadFile(safe)
	if err != nil {
		return "", fmt.Errorf("patch_file: read %s: %w", params.Path, err)
	}

	content := string(data)
	if !strings.Contains(content, params.OldString) {
		return "", fmt.Errorf("patch_file: old_string not found in %s", params.Path)
	}

	patched := strings.Replace(content, params.OldString, params.NewString, 1)

	if err := os.WriteFile(safe, []byte(patched), 0o644); err != nil {
		return "", fmt.Errorf("patch_file: write %s: %w", params.Path, err)
	}

	delta := len(patched) - len(content)
	return fmt.Sprintf("patched %s (old: %d bytes → new: %d bytes, delta: %+d)",
		params.Path, len(params.OldString), len(params.NewString), delta), nil
}
