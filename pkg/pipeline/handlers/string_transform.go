package handlers

import (
	"context"
	"fmt"
	"strings"

	"github.com/ravi-parthasarathy/attractor/pkg/pipeline"
)

// StringTransformHandler applies a chain of string operations to a context
// value and stores the result in the output key.
type StringTransformHandler struct{}

func (h *StringTransformHandler) Handle(_ context.Context, node *pipeline.Node, pctx *pipeline.PipelineContext) error {
	source := node.Attrs["source"]
	if source == "" {
		return fmt.Errorf("string_transform node %q: missing 'source' attribute", node.ID)
	}
	opsAttr := node.Attrs["ops"]
	if opsAttr == "" {
		return fmt.Errorf("string_transform node %q: missing 'ops' attribute", node.ID)
	}
	key := node.Attrs["key"]
	if key == "" {
		return fmt.Errorf("string_transform node %q: missing 'key' attribute", node.ID)
	}

	val := pctx.GetString(source)
	snapshot := pctx.Snapshot()

	for _, op := range strings.Split(opsAttr, ",") {
		op = strings.TrimSpace(op)
		switch op {
		case "trim":
			val = strings.TrimSpace(val)
		case "upper":
			val = strings.ToUpper(val)
		case "lower":
			val = strings.ToLower(val)
		case "replace":
			oldTpl := node.Attrs["old"]
			newTpl := node.Attrs["new"]
			oldStr, err := renderTemplate(oldTpl, snapshot)
			if err != nil {
				return fmt.Errorf("string_transform node %q: 'old' template error: %w", node.ID, err)
			}
			newStr, err := renderTemplate(newTpl, snapshot)
			if err != nil {
				return fmt.Errorf("string_transform node %q: 'new' template error: %w", node.ID, err)
			}
			val = strings.ReplaceAll(val, oldStr, newStr)
		default:
			return fmt.Errorf("string_transform node %q: unknown op %q (supported: trim, upper, lower, replace)", node.ID, op)
		}
	}

	pctx.Set(key, val)
	return nil
}
