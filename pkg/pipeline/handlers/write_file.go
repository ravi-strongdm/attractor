package handlers

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"

	"github.com/ravi-parthasarathy/attractor/pkg/pipeline"
)

// WriteFileHandler renders path and content as Go templates and writes the
// result to disk, optionally in append mode.
type WriteFileHandler struct{}

func (h *WriteFileHandler) Handle(_ context.Context, node *pipeline.Node, pctx *pipeline.PipelineContext) error {
	snap := pctx.Snapshot()

	pathTpl := node.Attrs["path"]
	if pathTpl == "" {
		return fmt.Errorf("write_file node %q: missing required 'path' attribute", node.ID)
	}
	contentTpl := node.Attrs["content"]
	if contentTpl == "" {
		return fmt.Errorf("write_file node %q: missing required 'content' attribute", node.ID)
	}

	path, err := renderTemplate(pathTpl, snap)
	if err != nil {
		return fmt.Errorf("write_file node %q: path template: %w", node.ID, err)
	}
	content, err := renderTemplate(contentTpl, snap)
	if err != nil {
		return fmt.Errorf("write_file node %q: content template: %w", node.ID, err)
	}

	// Parse file mode (octal string, default 0644).
	mode := fs.FileMode(0o644)
	if modeStr := node.Attrs["mode"]; modeStr != "" {
		parsed, parseErr := strconv.ParseUint(modeStr, 8, 32)
		if parseErr != nil {
			return fmt.Errorf("write_file node %q: invalid mode %q: %w", node.ID, modeStr, parseErr)
		}
		mode = fs.FileMode(parsed)
	}

	// Create parent directories if necessary.
	if dir := filepath.Dir(path); dir != "." {
		if mkErr := os.MkdirAll(dir, 0o755); mkErr != nil {
			return fmt.Errorf("write_file node %q: create dirs %q: %w", node.ID, dir, mkErr)
		}
	}

	if node.Attrs["append"] == "true" {
		f, openErr := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, mode)
		if openErr != nil {
			return fmt.Errorf("write_file node %q: open %q: %w", node.ID, path, openErr)
		}
		_, writeErr := f.WriteString(content)
		closeErr := f.Close()
		if writeErr != nil {
			return fmt.Errorf("write_file node %q: write %q: %w", node.ID, path, writeErr)
		}
		if closeErr != nil {
			return fmt.Errorf("write_file node %q: close %q: %w", node.ID, path, closeErr)
		}
		return nil
	}

	if writeErr := os.WriteFile(path, []byte(content), mode); writeErr != nil {
		return fmt.Errorf("write_file node %q: write %q: %w", node.ID, path, writeErr)
	}
	return nil
}
