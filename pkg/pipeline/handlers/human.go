package handlers

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/ravi-parthasarathy/attractor/pkg/pipeline"
)

// HumanHandler pauses the pipeline and prompts the user for input via stdin.
type HumanHandler struct{}

func (h *HumanHandler) Handle(_ context.Context, node *pipeline.Node, pctx *pipeline.PipelineContext) error {
	prompt := node.Attrs["prompt"]
	if prompt == "" {
		prompt = fmt.Sprintf("Node %q requires your input", node.ID)
	}
	_, _ = fmt.Fprintf(os.Stdout, "\n[wait.human] %s\n> ", prompt)

	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("human node %q: read error: %w", node.ID, err)
	}
	response := strings.TrimSpace(line)
	pctx.Set(node.ID+"_response", response)
	return nil
}
