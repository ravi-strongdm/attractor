package handlers

import (
	"context"
	"time"

	"github.com/ravi-parthasarathy/attractor/pkg/pipeline"
)

// StartHandler initialises the pipeline context.
type StartHandler struct {
	// Seed is the CLI-provided seed instruction. If empty, the node's "seed"
	// attribute is used, or whatever was already set in the context.
	Seed string
}

func (h *StartHandler) Handle(_ context.Context, node *pipeline.Node, pctx *pipeline.PipelineContext) error {
	seed := h.Seed
	if seed == "" {
		// Fall back to node attr, then existing context value
		if s := node.Attrs["seed"]; s != "" {
			seed = s
		}
	}
	if seed != "" {
		pctx.Set("seed", seed)
	}
	pctx.Set("start_time", time.Now().UTC().Format(time.RFC3339))
	return nil
}
