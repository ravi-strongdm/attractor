package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"sync"

	"github.com/ravi-parthasarathy/attractor/pkg/agent"
	"github.com/ravi-parthasarathy/attractor/pkg/agent/tools"
	"github.com/ravi-parthasarathy/attractor/pkg/llm"
	"github.com/ravi-parthasarathy/attractor/pkg/pipeline"
)

// MapHandler runs a codergen prompt for every element of a JSON array stored
// in the pipeline context, collecting the outputs as a new JSON array.
// Items are processed in parallel, bounded by the concurrency attribute.
type MapHandler struct {
	DefaultModel string
	Workdir      string
}

func (h *MapHandler) Handle(ctx context.Context, node *pipeline.Node, pctx *pipeline.PipelineContext) error {
	itemsKey := node.Attrs["items"]
	if itemsKey == "" {
		return fmt.Errorf("map node %q: missing required 'items' attribute", node.ID)
	}
	itemKey := node.Attrs["item_key"]
	if itemKey == "" {
		return fmt.Errorf("map node %q: missing required 'item_key' attribute", node.ID)
	}
	promptTpl := node.Attrs["prompt"]
	if promptTpl == "" {
		return fmt.Errorf("map node %q: missing required 'prompt' attribute", node.ID)
	}

	resultsKey := node.Attrs["results_key"]
	if resultsKey == "" {
		resultsKey = node.ID + "_results"
	}

	// Parse items JSON array.
	itemsJSON := pctx.GetString(itemsKey)
	if itemsJSON == "" {
		pctx.Set(resultsKey, "[]")
		pctx.Set("last_output", "[]")
		return nil
	}
	var items []any
	if err := json.Unmarshal([]byte(itemsJSON), &items); err != nil {
		return fmt.Errorf("map node %q: context key %q is not a valid JSON array: %w", node.ID, itemsKey, err)
	}
	if len(items) == 0 {
		pctx.Set(resultsKey, "[]")
		pctx.Set("last_output", "[]")
		return nil
	}

	// Resolve model.
	model := h.DefaultModel
	if m := node.Attrs["model"]; m != "" {
		model = m
	}
	if model == "" {
		model = "anthropic:claude-sonnet-4-6"
	}

	// Concurrency limit: 0 means "run all in parallel".
	concurrency := len(items)
	if cs := node.Attrs["concurrency"]; cs != "" {
		if n, err := strconv.Atoi(cs); err == nil && n > 0 && n < concurrency {
			concurrency = n
		}
	}

	results := make([]string, len(items))
	errs := make([]error, len(items))

	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	for i, item := range items {
		i, item := i, item
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			results[i], errs[i] = h.runItem(ctx, node, pctx, model, itemKey, promptTpl, item, i)
		}()
	}
	wg.Wait()

	// Collect first error (if any).
	for _, err := range errs {
		if err != nil {
			return fmt.Errorf("map node %q: %w", node.ID, err)
		}
	}

	b, err := json.Marshal(results)
	if err != nil {
		return fmt.Errorf("map node %q: marshal results: %w", node.ID, err)
	}
	pctx.Set(resultsKey, string(b))
	pctx.Set("last_output", string(b))
	return nil
}

func (h *MapHandler) runItem(
	ctx context.Context,
	node *pipeline.Node,
	pctx *pipeline.PipelineContext,
	model, itemKey, promptTpl string,
	item any,
	idx int,
) (string, error) {
	// Each item gets an independent copy of the context.
	branchCtx := pctx.Copy()
	branchCtx.Set(itemKey, fmt.Sprintf("%v", item))

	rendered, err := renderTemplate(promptTpl, branchCtx.Snapshot())
	if err != nil {
		return "", fmt.Errorf("item %d: prompt template: %w", idx, err)
	}

	client, err := llm.NewClient(model)
	if err != nil {
		return "", fmt.Errorf("item %d: create LLM client: %w", idx, err)
	}

	registry := tools.NewRegistry()
	registry.Register(tools.NewReadFileTool(h.Workdir))
	registry.Register(tools.NewWriteFileTool(h.Workdir))
	registry.Register(tools.NewRunCommandTool(h.Workdir))
	registry.Register(tools.NewListDirTool(h.Workdir))
	registry.Register(tools.NewSearchFileTool(h.Workdir))
	registry.Register(tools.NewPatchFileTool(h.Workdir))

	opts := []agent.Option{agent.WithModel(model)}
	if sp := node.Attrs["system_prompt"]; sp != "" {
		opts = append(opts, agent.WithSystem(sp))
	}
	if mt := node.Attrs["max_turns"]; mt != "" {
		if n, parseErr := strconv.Atoi(mt); parseErr == nil && n > 0 {
			opts = append(opts, agent.WithMaxTurns(n))
		}
	}

	eventCh := make(chan agent.Event, 64)
	opts = append(opts, agent.WithEvents(eventCh))
	loop := agent.NewCodingAgentLoop(client, registry, h.Workdir, opts...)

	done := make(chan struct{})
	go func() {
		defer close(done)
		for e := range eventCh {
			switch e.Type {
			case agent.EventTypeToolCall:
				slog.Debug("map tool call", "node", node.ID, "item", idx, "tool", e.ToolName)
			case agent.EventTypeError:
				slog.Warn("map agent error", "node", node.ID, "item", idx, "error", e.Content)
			case agent.EventTypeSteering:
				slog.Warn("map agent steering", "node", node.ID, "item", idx, "message", e.Content)
			}
		}
	}()

	result, agentErr := loop.Run(ctx, rendered)
	close(eventCh)
	<-done

	if agentErr != nil {
		return "", fmt.Errorf("item %d: agent loop: %w", idx, agentErr)
	}
	return result.Output, nil
}
