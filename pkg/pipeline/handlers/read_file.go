package handlers

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/ravi-parthasarathy/attractor/pkg/pipeline"
)

// ReadFileHandler reads a file from disk and stores its contents in the
// pipeline context under the configured key.
type ReadFileHandler struct{}

func (h *ReadFileHandler) Handle(_ context.Context, node *pipeline.Node, pctx *pipeline.PipelineContext) error {
	key := node.Attrs["key"]
	if key == "" {
		return fmt.Errorf("read_file node %q: missing required 'key' attribute", node.ID)
	}
	pathTpl := node.Attrs["path"]
	if pathTpl == "" {
		return fmt.Errorf("read_file node %q: missing required 'path' attribute", node.ID)
	}

	path, err := renderTemplate(pathTpl, pctx.Snapshot())
	if err != nil {
		return fmt.Errorf("read_file node %q: path template: %w", node.ID, err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) && node.Attrs["required"] == "false" {
			pctx.Set(key, "")
			return nil
		}
		return fmt.Errorf("read_file node %q: read %q: %w", node.ID, path, err)
	}

	pctx.Set(key, string(data))
	return nil
}
