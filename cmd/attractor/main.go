package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/ravi-parthasarathy/attractor/pkg/pipeline"
	"github.com/ravi-parthasarathy/attractor/pkg/pipeline/handlers"

	// Register all LLM providers via their init() functions.
	_ "github.com/ravi-parthasarathy/attractor/pkg/llm/providers"
)

func main() {
	if err := rootCmd().Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func rootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "attractor",
		Short: "Attractor — agentic pipeline runner",
		Long: `Attractor executes DOT-graph pipelines of AI coding agents.

Each node in the graph is a typed handler (codergen, wait.human, set, …).
Edges carry natural-language or boolean conditions that control flow.`,
	}
	root.AddCommand(runCmd())
	root.AddCommand(lintCmd())
	root.AddCommand(resumeCmd())
	return root
}

// ─── run ──────────────────────────────────────────────────────────────────────

func runCmd() *cobra.Command {
	var (
		workdir        string
		defaultModel   string
		checkpointPath string
		seed           string
	)

	cmd := &cobra.Command{
		Use:   "run <pipeline.dot>",
		Short: "Execute a pipeline from the beginning",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dotFile := args[0]
			return executePipeline(cmd.Context(), dotFile, workdir, defaultModel, checkpointPath, seed, "")
		},
	}

	cmd.Flags().StringVar(&workdir, "workdir", ".", "working directory for agent file operations")
	cmd.Flags().StringVar(&defaultModel, "model", "anthropic:claude-sonnet-4-6", "default LLM model (provider:model-id)")
	cmd.Flags().StringVar(&checkpointPath, "checkpoint", "", "path to write/read checkpoint JSON (optional)")
	cmd.Flags().StringVar(&seed, "seed", "", "initial seed value stored in pipeline context")
	return cmd
}

// ─── lint ─────────────────────────────────────────────────────────────────────

func lintCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "lint <pipeline.dot>",
		Short: "Validate a pipeline DOT file without running it",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			dotFile := args[0]
			src, err := os.ReadFile(dotFile)
			if err != nil {
				return fmt.Errorf("read file: %w", err)
			}
			p, err := pipeline.ParseDOT(string(src))
			if err != nil {
				return fmt.Errorf("parse: %w", err)
			}
			if lintErr := pipeline.ValidateErr(p); lintErr != nil {
				return lintErr
			}
			fmt.Printf("OK: pipeline %q is valid (%d nodes, %d edges)\n",
				p.Name, len(p.Nodes), len(p.Edges))
			return nil
		},
	}
	return cmd
}

// ─── resume ───────────────────────────────────────────────────────────────────

func resumeCmd() *cobra.Command {
	var (
		workdir      string
		defaultModel string
	)

	cmd := &cobra.Command{
		Use:   "resume <pipeline.dot> <checkpoint.json>",
		Short: "Resume a pipeline from a checkpoint",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			dotFile, cpFile := args[0], args[1]

			// Load context from checkpoint.
			pctx, lastNodeID, err := pipeline.LoadCheckpoint(cpFile)
			if err != nil {
				return fmt.Errorf("load checkpoint: %w", err)
			}
			fmt.Printf("[attractor] resuming from node %q\n", lastNodeID)

			// Parse pipeline.
			src, err := os.ReadFile(dotFile)
			if err != nil {
				return fmt.Errorf("read pipeline file: %w", err)
			}
			p, err := pipeline.ParseDOT(string(src))
			if err != nil {
				return fmt.Errorf("parse pipeline: %w", err)
			}
			if lintErr := pipeline.ValidateErr(p); lintErr != nil {
				return fmt.Errorf("invalid pipeline: %w", lintErr)
			}

			// Apply any stylesheet.
			pipeline.ApplyStylesheet(p)

			// Build engine.
			reg := buildRegistry(workdir, defaultModel)
			eng, err := pipeline.NewEngine(p, reg, pctx, cpFile)
			if err != nil {
				return fmt.Errorf("build engine: %w", err)
			}

			ctx := signalContext(cmd.Context())
			return eng.Execute(ctx, lastNodeID)
		},
	}

	cmd.Flags().StringVar(&workdir, "workdir", ".", "working directory for agent file operations")
	cmd.Flags().StringVar(&defaultModel, "model", "anthropic:claude-sonnet-4-6", "default LLM model")
	return cmd
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func executePipeline(
	ctx context.Context,
	dotFile, workdir, defaultModel, checkpointPath, seed, resumeFromNodeID string,
) error {
	// Read and parse pipeline.
	src, err := os.ReadFile(dotFile)
	if err != nil {
		return fmt.Errorf("read pipeline file: %w", err)
	}
	p, err := pipeline.ParseDOT(string(src))
	if err != nil {
		return fmt.Errorf("parse pipeline: %w", err)
	}
	if lintErr := pipeline.ValidateErr(p); lintErr != nil {
		return fmt.Errorf("invalid pipeline: %w", lintErr)
	}

	// Apply stylesheet overrides.
	pipeline.ApplyStylesheet(p)

	// Initialise context.
	pctx := pipeline.NewPipelineContext()
	if seed != "" {
		pctx.Set("seed", seed)
	}

	// Build handler registry.
	reg := buildRegistry(workdir, defaultModel)

	// Build and run engine.
	eng, err := pipeline.NewEngine(p, reg, pctx, checkpointPath)
	if err != nil {
		return fmt.Errorf("build engine: %w", err)
	}

	sctx := signalContext(ctx)
	return eng.Execute(sctx, resumeFromNodeID)
}

// buildRegistry constructs a handler registry with all built-in handlers.
func buildRegistry(workdir, defaultModel string) *handlers.Registry {
	reg := handlers.NewRegistry()
	reg.Register("start", &handlers.StartHandler{})
	reg.Register("exit", &handlers.ExitHandler{})
	reg.Register("set", &handlers.SetHandler{})
	reg.Register("wait.human", &handlers.HumanHandler{})
	reg.Register("fan_out", &handlers.FanOutHandler{})
	reg.Register("fan_in", &handlers.FanInHandler{})
	reg.Register("codergen", &handlers.CodergenHandler{
		DefaultModel: defaultModel,
		Workdir:      workdir,
	})
	return reg
}

// signalContext returns a context that is cancelled on SIGINT or SIGTERM.
func signalContext(parent context.Context) context.Context {
	ctx, cancel := context.WithCancel(parent)
	go func() {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
		select {
		case <-ch:
			fmt.Fprintln(os.Stderr, "\n[attractor] interrupted — cancelling pipeline")
			cancel()
		case <-ctx.Done():
		}
	}()
	return ctx
}
