package handlers

import (
	"context"
	"time"

	"github.com/ravi-parthasarathy/attractor/pkg/pipeline"
)

// ExitHandler marks the pipeline as complete by returning an ExitSignal.
type ExitHandler struct{}

func (h *ExitHandler) Handle(_ context.Context, _ *pipeline.Node, pctx *pipeline.PipelineContext) error {
	pctx.Set("exit_time", time.Now().UTC().Format(time.RFC3339))
	return pipeline.ExitSignal{}
}
