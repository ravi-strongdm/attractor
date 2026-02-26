package handlers

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/ravi-parthasarathy/attractor/pkg/pipeline"
)

// HumanHandler pauses the pipeline and prompts the user for input via stdin.
// Supports a "key" attr to control the context key and an "options" attr to
// display a numbered menu and validate the response.
type HumanHandler struct {
	// In and Out allow tests to inject alternate stdin/stdout.
	In  io.Reader
	Out io.Writer
}

func (h *HumanHandler) Handle(_ context.Context, node *pipeline.Node, pctx *pipeline.PipelineContext) error {
	promptText := node.Attrs["prompt"]
	if promptText == "" {
		promptText = fmt.Sprintf("Node %q requires your input", node.ID)
	}

	// Resolve output key.
	key := node.Attrs["key"]
	if key == "" {
		key = node.ID + "_response"
	}

	// Resolve I/O streams.
	in := h.In
	if in == nil {
		in = os.Stdin
	}
	out := h.Out
	if out == nil {
		out = os.Stdout
	}

	// Parse options if provided.
	var options []string
	if raw := node.Attrs["options"]; raw != "" {
		for _, o := range strings.Split(raw, ",") {
			if trimmed := strings.TrimSpace(o); trimmed != "" {
				options = append(options, trimmed)
			}
		}
	}

	reader := bufio.NewReader(in)

	for {
		// Print prompt.
		_, _ = fmt.Fprintf(out, "\n[wait.human] %s\n", promptText)
		if len(options) > 0 {
			for i, o := range options {
				_, _ = fmt.Fprintf(out, "  %d) %s\n", i+1, o)
			}
		}
		_, _ = fmt.Fprint(out, "> ")

		line, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("human node %q: read error: %w", node.ID, err)
		}
		response := strings.TrimSpace(line)

		if len(options) == 0 {
			// No validation — accept any input.
			pctx.Set(key, response)
			return nil
		}

		// Try numeric selection first.
		if n, parseErr := strconv.Atoi(response); parseErr == nil {
			if n >= 1 && n <= len(options) {
				pctx.Set(key, options[n-1])
				return nil
			}
		}

		// Try case-insensitive text match.
		lower := strings.ToLower(response)
		for _, o := range options {
			if strings.ToLower(o) == lower {
				pctx.Set(key, o)
				return nil
			}
		}

		// Invalid — re-prompt.
		_, _ = fmt.Fprintf(out, "[wait.human] Invalid choice %q — please enter a number (1-%d) or one of: %s\n",
			response, len(options), strings.Join(options, ", "))
	}
}
