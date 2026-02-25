package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const maxSearchResults = 200

// SearchFileTool searches for a text pattern across files in a directory.
type SearchFileTool struct {
	workdir string
}

// NewSearchFileTool creates a SearchFileTool sandboxed to workdir.
func NewSearchFileTool(workdir string) *SearchFileTool {
	return &SearchFileTool{workdir: workdir}
}

func (t *SearchFileTool) Name() string { return "search_file" }
func (t *SearchFileTool) Description() string {
	return "Search for a text pattern across files in a directory. Returns matching lines in file:line: content format."
}
func (t *SearchFileTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"pattern": {
				"type": "string",
				"description": "Text to search for (case-sensitive substring match)"
			},
			"path": {
				"type": "string",
				"description": "Directory or file to search within, relative to the working directory. Defaults to the working directory if omitted."
			}
		},
		"required": ["pattern"]
	}`)
}

func (t *SearchFileTool) Execute(_ context.Context, input json.RawMessage) (string, error) {
	var params struct {
		Pattern string `json:"pattern"`
		Path    string `json:"path"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("search_file: invalid input: %w", err)
	}
	if params.Pattern == "" {
		return "", fmt.Errorf("search_file: pattern must not be empty")
	}

	searchRoot := "."
	if params.Path != "" {
		searchRoot = params.Path
	}

	safe, err := safePath(t.workdir, searchRoot)
	if err != nil {
		return "", err
	}

	var matches []string
	err = filepath.WalkDir(safe, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil // skip unreadable entries
		}
		if d.IsDir() {
			// Skip hidden directories.
			if strings.HasPrefix(d.Name(), ".") && d.Name() != "." {
				return filepath.SkipDir
			}
			return nil
		}
		// Only search text-like files (skip binaries heuristically by extension).
		if isBinaryExtension(d.Name()) {
			return nil
		}

		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil // skip unreadable files
		}

		rel, _ := filepath.Rel(t.workdir, path)
		for i, line := range strings.Split(string(data), "\n") {
			if strings.Contains(line, params.Pattern) {
				matches = append(matches, fmt.Sprintf("%s:%d: %s", rel, i+1, line))
				if len(matches) >= maxSearchResults {
					return fmt.Errorf("limit") // sentinel to stop walking
				}
			}
		}
		return nil
	})
	// Swallow the "limit" sentinel; surface real errors.
	if err != nil && err.Error() != "limit" {
		return "", fmt.Errorf("search_file: %w", err)
	}

	if len(matches) == 0 {
		return "no matches found", nil
	}
	result := strings.Join(matches, "\n")
	if len(matches) >= maxSearchResults {
		result += fmt.Sprintf("\n[truncated: showing first %d matches]", maxSearchResults)
	}
	return result, nil
}

// isBinaryExtension returns true for file extensions that are unlikely to be
// plain text and therefore unproductive to grep.
func isBinaryExtension(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".png", ".jpg", ".jpeg", ".gif", ".bmp", ".ico",
		".pdf", ".zip", ".tar", ".gz", ".bz2", ".xz",
		".exe", ".dll", ".so", ".dylib", ".a", ".o",
		".wasm", ".bin", ".dat":
		return true
	}
	return false
}
