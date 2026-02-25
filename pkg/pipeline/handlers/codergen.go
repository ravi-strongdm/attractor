package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"

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
	registry.Register(tools.NewSearchFileTool(workdir))
	registry.Register(tools.NewPatchFileTool(workdir))

	opts := []agent.Option{
		agent.WithModel(model),
	}

	// Optional system_prompt from node attribute.
	if sp := node.Attrs["system_prompt"]; sp != "" {
		opts = append(opts, agent.WithSystem(sp))
	}

	// Optional max_turns from node attribute.
	if mt := node.Attrs["max_turns"]; mt != "" {
		if n, parseErr := strconv.Atoi(mt); parseErr == nil && n > 0 {
			opts = append(opts, agent.WithMaxTurns(n))
		}
	}

	eventCh := make(chan agent.Event, 64)
	opts = append(opts, agent.WithEvents(eventCh))

	loop := agent.NewCodingAgentLoop(client, registry, workdir, opts...)

	// Drain events in a goroutine for observability
	done := make(chan struct{})
	go func() {
		defer close(done)
		for e := range eventCh {
			switch e.Type {
			case agent.EventTypeToolCall:
				slog.Debug("tool call", "node", node.ID, "tool", e.ToolName, "input", e.Content)
			case agent.EventTypeError:
				slog.Warn("agent error", "node", node.ID, "error", e.Content)
			case agent.EventTypeSteering:
				slog.Warn("agent steering", "node", node.ID, "message", e.Content)
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
