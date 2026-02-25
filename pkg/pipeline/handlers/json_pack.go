package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ravi-parthasarathy/attractor/pkg/pipeline"
)

// JSONPackHandler packs a set of pipeline context keys into a single JSON
// object string and stores it under the key named by the "output" attribute.
type JSONPackHandler struct{}

func (h *JSONPackHandler) Handle(_ context.Context, node *pipeline.Node, pctx *pipeline.PipelineContext) error {
	keysAttr := node.Attrs["keys"]
	if keysAttr == "" {
		return fmt.Errorf("json_pack node %q: missing 'keys' attribute", node.ID)
	}
	output := node.Attrs["output"]
	if output == "" {
		return fmt.Errorf("json_pack node %q: missing 'output' attribute", node.ID)
	}

	// Split and trim the key names.
	names := strings.Split(keysAttr, ",")
	obj := make(map[string]string, len(names))
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		obj[name] = pctx.GetString(name)
	}

	data, err := json.Marshal(obj)
	if err != nil {
		return fmt.Errorf("json_pack node %q: marshal: %w", node.ID, err)
	}
	pctx.Set(output, string(data))
	return nil
}
