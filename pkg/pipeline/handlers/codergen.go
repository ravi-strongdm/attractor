package handlers

import (
	"context"
	"fmt"

	"github.com/ravi-parthasarathy/attractor/pkg/agent"
	"github.com/ravi-parthasarathy/attractor/pkg/agent/tools"
	"github.com/ravi-parthasarathy/attractor/pkg/llm"
	"github.com/ravi-parthasarathy/attractor/pkg/pipeline"
)

// CodergenHandler runs a CodingAgentLoop for a codergen node.
type CodergenHandler struct {
	DefaultModel string
	Workdir      string
}

func (h *CodergenHandler) Handle(ctx context.Context, node *pipeline.Node, pctx *pipeline.PipelineContext) error {
	// Node attr overrides the default model
	model := h.DefaultModel
	if m := node.Attrs["model"]; m != "" {
		model = m
	}
	if model == "" {
		model = "anthropic:claude-sonnet-4-6"
	}

	// Render the prompt template against the pipeline context
	promptTpl := node.Attrs["prompt"]
	if promptTpl == "" {
		promptTpl = pctx.GetString("seed")
	}
	rendered, err := renderTemplate(promptTpl, pctx.Snapshot())
	if err != nil {
		return fmt.Errorf("codergen node %q: template error: %w", node.ID, err)
	}

	client, err := llm.NewClient(model)
	if err != nil {
		return fmt.Errorf("codergen node %q: create LLM client: %w", node.ID, err)
	}

	workdir := h.Workdir
	registry := tools.NewRegistry()
	registry.Register(tools.NewReadFileTool(workdir))
	registry.Register(tools.NewWriteFileTool(workdir))
	registry.Register(tools.NewRunCommandTool(workdir))
	registry.Register(tools.NewListDirTool(workdir))

	eventCh := make(chan agent.Event, 64)
	loop := agent.NewCodingAgentLoop(client, registry, workdir,
		agent.WithModel(model),
		agent.WithEvents(eventCh),
	)

	// Drain events in a goroutine for observability
	done := make(chan struct{})
	go func() {
		defer close(done)
		for e := range eventCh {
			switch e.Type {
			case agent.EventTypeToolCall:
				fmt.Printf("  [tool] %s: %s\n", e.ToolName, e.Content)
			case agent.EventTypeError:
				fmt.Printf("  [error] %s\n", e.Content)
			case agent.EventTypeSteering:
				fmt.Printf("  [steering] %s\n", e.Content)
			}
		}
	}()

	result, agentErr := loop.Run(ctx, rendered)
	close(eventCh)
	<-done

	if agentErr != nil {
		return fmt.Errorf("codergen node %q: agent loop: %w", node.ID, agentErr)
	}

	pctx.Set("last_output", result.Output)
	pctx.Set(node.ID+"_output", result.Output)
	return nil
}
