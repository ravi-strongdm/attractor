package handlers

import (
	"context"
	"fmt"

	"github.com/ravi-parthasarathy/attractor/pkg/pipeline"
)

// SetHandler evaluates the node's "value" attribute as a Go template and stores
// the result under the node's "key" attribute in the context.
type SetHandler struct{}

func (h *SetHandler) Handle(_ context.Context, node *pipeline.Node, pctx *pipeline.PipelineContext) error {
	key := node.Attrs["key"]
	valueTpl := node.Attrs["value"]
	if key == "" {
		return fmt.Errorf("set node %q: missing 'key' attribute", node.ID)
	}
	val, err := renderTemplate(valueTpl, pctx.Snapshot())
	if err != nil {
		return fmt.Errorf("set node %q: template error: %w", node.ID, err)
	}
	pctx.Set(key, val)
	return nil
}
