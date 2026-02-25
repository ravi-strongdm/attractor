package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/ravi-parthasarathy/attractor/pkg/pipeline"
)

// JSONExtractHandler extracts a value from a JSON string in the pipeline
// context using a simple dot-path expression and stores the result under
// a new context key.
type JSONExtractHandler struct{}

func (h *JSONExtractHandler) Handle(_ context.Context, node *pipeline.Node, pctx *pipeline.PipelineContext) error {
	sourceKey := node.Attrs["source"]
	if sourceKey == "" {
		return fmt.Errorf("json_extract node %q: missing required 'source' attribute", node.ID)
	}
	pathStr := node.Attrs["path"]
	if pathStr == "" {
		return fmt.Errorf("json_extract node %q: missing required 'path' attribute", node.ID)
	}
	destKey := node.Attrs["key"]
	if destKey == "" {
		return fmt.Errorf("json_extract node %q: missing required 'key' attribute", node.ID)
	}
	defaultVal := node.Attrs["default"]

	raw := pctx.GetString(sourceKey)
	if raw == "" {
		pctx.Set(destKey, defaultVal)
		return nil
	}

	var root any
	if err := json.Unmarshal([]byte(raw), &root); err != nil {
		return fmt.Errorf("json_extract node %q: unmarshal source %q: %w", node.ID, sourceKey, err)
	}

	// Strip optional leading dot and split path into segments.
	clean := strings.TrimPrefix(pathStr, ".")
	segments := strings.Split(clean, ".")

	val, err := walkPath(root, segments)
	if err != nil {
		if defaultVal != "" {
			pctx.Set(destKey, defaultVal)
			return nil
		}
		return fmt.Errorf("json_extract node %q: path %q: %w", node.ID, pathStr, err)
	}

	pctx.Set(destKey, anyToString(val))
	return nil
}

// walkPath navigates a parsed JSON value following the given path segments.
// Numeric segments are used as array indices; all others as map keys.
func walkPath(v any, segments []string) (any, error) {
	cur := v
	for _, seg := range segments {
		if seg == "" {
			continue
		}
		switch c := cur.(type) {
		case map[string]any:
			next, ok := c[seg]
			if !ok {
				return nil, fmt.Errorf("key %q not found", seg)
			}
			cur = next
		case []any:
			idx, err := strconv.Atoi(seg)
			if err != nil {
				return nil, fmt.Errorf("segment %q is not a valid array index", seg)
			}
			if idx < 0 || idx >= len(c) {
				return nil, fmt.Errorf("index %d out of range (len=%d)", idx, len(c))
			}
			cur = c[idx]
		default:
			return nil, fmt.Errorf("cannot index into %T with segment %q", cur, seg)
		}
	}
	return cur, nil
}

// anyToString converts a JSON value to its string representation.
// Primitives use fmt; objects and arrays are re-marshalled to compact JSON.
func anyToString(v any) string {
	switch t := v.(type) {
	case nil:
		return "null"
	case string:
		return t
	case bool:
		if t {
			return "true"
		}
		return "false"
	case float64:
		// Avoid scientific notation for integers.
		if t == float64(int64(t)) {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'f', -1, 64)
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprintf("%v", v)
		}
		return string(b)
	}
}
