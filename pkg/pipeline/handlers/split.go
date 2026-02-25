package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ravi-parthasarathy/attractor/pkg/pipeline"
)

// SplitHandler splits a string stored in the pipeline context by a separator
// and stores the resulting elements as a JSON array under a new context key.
type SplitHandler struct{}

func (h *SplitHandler) Handle(_ context.Context, node *pipeline.Node, pctx *pipeline.PipelineContext) error {
	source := node.Attrs["source"]
	if source == "" {
		return fmt.Errorf("split node %q: missing required 'source' attribute", node.ID)
	}
	key := node.Attrs["key"]
	if key == "" {
		return fmt.Errorf("split node %q: missing required 'key' attribute", node.ID)
	}

	sep := node.Attrs["sep"]
	if sep == "" {
		sep = "\n"
	}

	raw := pctx.GetString(source)
	parts := strings.Split(raw, sep)

	if node.Attrs["trim"] == "true" {
		filtered := parts[:0]
		for _, p := range parts {
			if t := strings.TrimSpace(p); t != "" {
				filtered = append(filtered, t)
			}
		}
		parts = filtered
	}

	b, err := json.Marshal(parts)
	if err != nil {
		return fmt.Errorf("split node %q: marshal result: %w", node.ID, err)
	}
	pctx.Set(key, string(b))
	return nil
}
