package handlers

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/ravi-parthasarathy/attractor/pkg/pipeline"
)

// SwitchHandler is a routing node whose outgoing-edge selection is performed
// by the engine (exact string matching against a context key value).
// The handler itself only validates that the required attribute is present
// and logs the routing key and its current value.
type SwitchHandler struct{}

func (h *SwitchHandler) Handle(_ context.Context, node *pipeline.Node, pctx *pipeline.PipelineContext) error {
	key := node.Attrs["key"]
	if key == "" {
		return fmt.Errorf("switch node %q: missing required 'key' attribute", node.ID)
	}
	val, ok := pctx.Get(key)
	if !ok {
		slog.Warn("switch node: context key not set", "node", node.ID, "key", key)
	} else {
		slog.Debug("switch node routing", "node", node.ID, "key", key, "value", val)
	}
	return nil
}
