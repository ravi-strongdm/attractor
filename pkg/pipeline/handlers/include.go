package handlers

import (
	"context"
	"fmt"
	"os"

	"github.com/ravi-parthasarathy/attractor/pkg/pipeline"
)

// IncludeHandler executes another DOT pipeline file as an inline sub-pipeline,
// sharing the caller's PipelineContext so all changes propagate back.
//
// RegistryBuilder is a function that constructs a handler registry for the
// sub-pipeline; injected at registration time to avoid import cycles.
type IncludeHandler struct {
	Workdir         string
	DefaultModel    string
	RegistryBuilder func(workdir, defaultModel string) pipeline.HandlerRegistry
}

func (h *IncludeHandler) Handle(ctx context.Context, node *pipeline.Node, pctx *pipeline.PipelineContext) error {
	pathTpl := node.Attrs["path"]
	if pathTpl == "" {
		return fmt.Errorf("include node %q: missing 'path' attribute", node.ID)
	}

	// Render path template.
	rendered, err := renderTemplate(pathTpl, pctx.Snapshot())
	if err != nil {
		return fmt.Errorf("include node %q: path template error: %w", node.ID, err)
	}

	// Read and parse included pipeline.
	src, err := os.ReadFile(rendered)
	if err != nil {
		return fmt.Errorf("include node %q: read %q: %w", node.ID, rendered, err)
	}
	p, err := pipeline.ParseDOT(string(src))
	if err != nil {
		return fmt.Errorf("include node %q: parse %q: %w", node.ID, rendered, err)
	}
	if lintErr := pipeline.ValidateErr(p); lintErr != nil {
		return fmt.Errorf("include node %q: invalid pipeline %q: %w", node.ID, rendered, lintErr)
	}

	pipeline.ApplyStylesheet(p)

	// Build registry for sub-pipeline.
	workdir := h.Workdir
	if h.RegistryBuilder == nil {
		return fmt.Errorf("include node %q: RegistryBuilder not configured", node.ID)
	}
	reg := h.RegistryBuilder(workdir, h.DefaultModel)

	// Create engine with the shared context (no checkpoint for sub-pipelines).
	eng, err := pipeline.NewEngine(p, reg, pctx, "")
	if err != nil {
		return fmt.Errorf("include node %q: build engine: %w", node.ID, err)
	}

	if runErr := eng.Execute(ctx, ""); runErr != nil {
		return fmt.Errorf("include node %q: %w", node.ID, runErr)
	}
	return nil
}
