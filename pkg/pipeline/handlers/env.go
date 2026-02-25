package handlers

import (
	"context"
	"fmt"
	"os"

	"github.com/ravi-parthasarathy/attractor/pkg/pipeline"
)

// EnvHandler reads an OS environment variable and injects it into the
// pipeline context.
type EnvHandler struct{}

func (h *EnvHandler) Handle(_ context.Context, node *pipeline.Node, pctx *pipeline.PipelineContext) error {
	key := node.Attrs["key"]
	if key == "" {
		return fmt.Errorf("env node %q: missing required 'key' attribute", node.ID)
	}
	from := node.Attrs["from"]
	if from == "" {
		return fmt.Errorf("env node %q: missing required 'from' attribute", node.ID)
	}

	value := os.Getenv(from)
	if value == "" {
		if node.Attrs["required"] == "true" {
			return fmt.Errorf("env node %q: required environment variable %q is not set", node.ID, from)
		}
		if def := node.Attrs["default"]; def != "" {
			value = def
		}
	}

	pctx.Set(key, value)
	return nil
}
