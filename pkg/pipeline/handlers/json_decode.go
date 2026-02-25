package handlers

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ravi-parthasarathy/attractor/pkg/pipeline"
)

// JSONDecodeHandler unpacks a JSON object stored in a context key into
// individual context keys, optionally prefixed.
type JSONDecodeHandler struct{}

func (h *JSONDecodeHandler) Handle(_ context.Context, node *pipeline.Node, pctx *pipeline.PipelineContext) error {
	source := node.Attrs["source"]
	if source == "" {
		return fmt.Errorf("json_decode node %q: missing 'source' attribute", node.ID)
	}
	prefix := node.Attrs["prefix"]

	raw := pctx.GetString(source)
	// Empty source is treated as an empty object — no keys to set.
	if raw == "" {
		return nil
	}

	// Unmarshal into a generic map to detect non-objects before typed decode.
	var top any
	if err := json.Unmarshal([]byte(raw), &top); err != nil {
		return fmt.Errorf("json_decode node %q: invalid JSON in %q: %w", node.ID, source, err)
	}
	if _, ok := top.(map[string]any); !ok {
		return fmt.Errorf("json_decode node %q: value of %q must be a JSON object", node.ID, source)
	}

	fields := top.(map[string]any)
	for k, v := range fields {
		var strVal string
		switch tv := v.(type) {
		case string:
			strVal = tv
		case nil:
			strVal = ""
		case bool, float64:
			strVal = fmt.Sprintf("%v", tv)
		default:
			// Nested object or array — re-marshal to compact JSON.
			b, err := json.Marshal(tv)
			if err != nil {
				return fmt.Errorf("json_decode node %q: marshal field %q: %w", node.ID, k, err)
			}
			strVal = string(b)
		}
		pctx.Set(prefix+k, strVal)
	}
	return nil
}
