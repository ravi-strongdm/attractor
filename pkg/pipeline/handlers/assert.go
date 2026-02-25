package handlers

import (
	"context"
	"fmt"

	"github.com/ravi-parthasarathy/attractor/pkg/pipeline"
)

// AssertHandler evaluates a condition expression against the pipeline context
// and returns an error if the condition is false.
type AssertHandler struct{}

func (h *AssertHandler) Handle(_ context.Context, node *pipeline.Node, pctx *pipeline.PipelineContext) error {
	expr := node.Attrs["expr"]
	if expr == "" {
		return fmt.Errorf("assert node %q: missing required 'expr' attribute", node.ID)
	}

	ok, err := pipeline.EvalCondition(expr, pctx.Snapshot())
	if err != nil {
		return fmt.Errorf("assert node %q: eval condition: %w", node.ID, err)
	}
	if !ok {
		msg := node.Attrs["message"]
		if msg == "" {
			msg = "assertion failed"
		}
		return fmt.Errorf("assert node %q: %s: expr=%q", node.ID, msg, expr)
	}
	return nil
}
