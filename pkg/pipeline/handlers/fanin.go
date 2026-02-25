package handlers

import (
	"context"

	"github.com/ravi-parthasarathy/attractor/pkg/pipeline"
)

// FanInHandler is a synchronisation barrier. It records that parallel branches
// have converged. Actual waiting is handled by the engine.
type FanInHandler struct{}

func (h *FanInHandler) Handle(_ context.Context, node *pipeline.Node, pctx *pipeline.PipelineContext) error {
	pctx.Set(node.ID+"_fanin", "complete")
	return nil
}
