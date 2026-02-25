package handlers

import (
	"context"
	"fmt"
	"strconv"

	"github.com/ravi-parthasarathy/attractor/pkg/llm"
	"github.com/ravi-parthasarathy/attractor/pkg/pipeline"
)

const defaultPromptMaxTokens = 1024

// PromptHandler performs a single-turn LLM call (no tool loop) and stores the
// text response in the context key named by the node's "key" attribute.
type PromptHandler struct {
	DefaultModel string
}

func (h *PromptHandler) Handle(ctx context.Context, node *pipeline.Node, pctx *pipeline.PipelineContext) error {
	// Resolve required attributes.
	promptTpl := node.Attrs["prompt"]
	if promptTpl == "" {
		return fmt.Errorf("prompt node %q: missing 'prompt' attribute", node.ID)
	}
	key := node.Attrs["key"]
	if key == "" {
		return fmt.Errorf("prompt node %q: missing 'key' attribute", node.ID)
	}

	// Render prompt template.
	rendered, err := renderTemplate(promptTpl, pctx.Snapshot())
	if err != nil {
		return fmt.Errorf("prompt node %q: template error: %w", node.ID, err)
	}

	// Resolve model.
	model := h.DefaultModel
	if m := node.Attrs["model"]; m != "" {
		model = m
	}
	if model == "" {
		model = "anthropic:claude-sonnet-4-6"
	}

	// Resolve max_tokens.
	maxTokens := defaultPromptMaxTokens
	if mt := node.Attrs["max_tokens"]; mt != "" {
		if n, parseErr := strconv.Atoi(mt); parseErr == nil && n > 0 {
			maxTokens = n
		}
	}

	// Build request.
	req := llm.GenerateRequest{
		Model:     model,
		Messages:  []llm.Message{llm.TextMessage(llm.RoleUser, rendered)},
		MaxTokens: maxTokens,
	}
	if sys := node.Attrs["system"]; sys != "" {
		req.System = sys
	}

	// Create client and call.
	client, err := llm.NewClient(model)
	if err != nil {
		return fmt.Errorf("prompt node %q: create LLM client: %w", node.ID, err)
	}
	resp, err := client.Complete(ctx, req)
	if err != nil {
		return fmt.Errorf("prompt node %q: LLM call: %w", node.ID, err)
	}

	// Extract first text block.
	var output string
	for _, block := range resp.Content {
		if block.Type == llm.ContentTypeText {
			output = block.Text
			break
		}
	}

	pctx.Set(key, output)
	pctx.Set("last_output", output)
	return nil
}
