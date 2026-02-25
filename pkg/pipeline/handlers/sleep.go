package handlers

import (
	"context"
	"fmt"
	"time"

	"github.com/ravi-parthasarathy/attractor/pkg/pipeline"
)

// SleepHandler pauses pipeline execution for a fixed duration.
// The sleep is cancellable via the context.
type SleepHandler struct{}

func (h *SleepHandler) Handle(ctx context.Context, node *pipeline.Node, _ *pipeline.PipelineContext) error {
	durStr := node.Attrs["duration"]
	if durStr == "" {
		return fmt.Errorf("sleep node %q: missing required 'duration' attribute", node.ID)
	}
	dur, err := time.ParseDuration(durStr)
	if err != nil {
		return fmt.Errorf("sleep node %q: invalid duration %q: %w", node.ID, durStr, err)
	}

	timer := time.NewTimer(dur)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return fmt.Errorf("sleep node %q: cancelled: %w", node.ID, ctx.Err())
	case <-timer.C:
		return nil
	}
}
