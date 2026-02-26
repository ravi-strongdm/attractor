package handlers

import (
	"context"
	"fmt"
	"regexp"
	"strconv"

	"github.com/ravi-parthasarathy/attractor/pkg/pipeline"
)

// RegexHandler applies a regular expression to a context value and stores a
// capture group (or the whole match) in the output key.
type RegexHandler struct{}

func (h *RegexHandler) Handle(_ context.Context, node *pipeline.Node, pctx *pipeline.PipelineContext) error {
	source := node.Attrs["source"]
	if source == "" {
		return fmt.Errorf("regex node %q: missing 'source' attribute", node.ID)
	}
	pattern := node.Attrs["pattern"]
	if pattern == "" {
		return fmt.Errorf("regex node %q: missing 'pattern' attribute", node.ID)
	}
	key := node.Attrs["key"]
	if key == "" {
		return fmt.Errorf("regex node %q: missing 'key' attribute", node.ID)
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return fmt.Errorf("regex node %q: invalid pattern: %w", node.ID, err)
	}

	group := 0
	if g := node.Attrs["group"]; g != "" {
		n, parseErr := strconv.Atoi(g)
		if parseErr != nil || n < 0 {
			return fmt.Errorf("regex node %q: group must be a non-negative integer, got %q", node.ID, g)
		}
		group = n
	}

	noMatch := node.Attrs["no_match"] // default: ""

	input := pctx.GetString(source)
	matches := re.FindStringSubmatch(input)
	if matches == nil {
		pctx.Set(key, noMatch)
		return nil
	}
	if group >= len(matches) {
		return fmt.Errorf("regex node %q: group %d out of range (pattern has %d groups)", node.ID, group, len(matches)-1)
	}
	pctx.Set(key, matches[group])
	return nil
}
