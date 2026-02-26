package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"runtime/debug"
	"strings"
	"syscall"
	"time"

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
	var (
		logLevel  string
		logFormat string
	)

	root := &cobra.Command{
		Use:   "attractor",
		Short: "Attractor — agentic pipeline runner",
		Long: `Attractor executes DOT-graph pipelines of AI coding agents.

Each node in the graph is a typed handler (codergen, wait.human, set, …).
Edges carry natural-language or boolean conditions that control flow.`,
		PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
			return initLogger(logLevel, logFormat)
		},
	}

	root.PersistentFlags().StringVar(&logLevel, "log-level", "info", "log level: debug, info, warn, error")
	root.PersistentFlags().StringVar(&logFormat, "log-format", "text", "log format: text, json")

	root.AddCommand(runCmd())
	root.AddCommand(lintCmd())
	root.AddCommand(resumeCmd())
	root.AddCommand(versionCmd())
	root.AddCommand(graphCmd())
	return root
}

// initLogger configures the global slog default handler.
func initLogger(level, format string) error {
	var lvl slog.Level
	switch strings.ToLower(level) {
	case "debug":
		lvl = slog.LevelDebug
	case "info", "":
		lvl = slog.LevelInfo
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		return fmt.Errorf("unknown log level %q: use debug, info, warn, or error", level)
	}

	opts := &slog.HandlerOptions{Level: lvl}
	var handler slog.Handler
	switch strings.ToLower(format) {
	case "json":
		handler = slog.NewJSONHandler(os.Stderr, opts)
	case "text", "":
		handler = slog.NewTextHandler(os.Stderr, opts)
	default:
		return fmt.Errorf("unknown log format %q: use text or json", format)
	}
	slog.SetDefault(slog.New(handler))
	return nil
}

// ─── run ──────────────────────────────────────────────────────────────────────

func runCmd() *cobra.Command {
	var (
		workdir        string
		defaultModel   string
		checkpointPath string
		outContextPath string
		seed           string
		timeout        time.Duration
		vars           []string
		varFile        string
	)

	cmd := &cobra.Command{
		Use:   "run <pipeline.dot>",
		Short: "Execute a pipeline from the beginning",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dotFile := args[0]
			ctx := cmd.Context()
			if timeout > 0 {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, timeout)
				defer cancel()
			}
			return executePipeline(ctx, dotFile, workdir, defaultModel, checkpointPath, outContextPath, seed, varFile, vars, "")
		},
	}

	cmd.Flags().StringVar(&workdir, "workdir", ".", "working directory for agent file operations")
	cmd.Flags().StringVar(&defaultModel, "model", "anthropic:claude-sonnet-4-6", "default LLM model (provider:model-id)")
	cmd.Flags().StringVar(&checkpointPath, "checkpoint", "", "path to write/read checkpoint JSON (optional)")
	cmd.Flags().StringVar(&outContextPath, "output-context", "", "write final pipeline context as JSON to this file")
	cmd.Flags().StringVar(&seed, "seed", "", "initial seed value stored in pipeline context as 'seed'")
	cmd.Flags().DurationVar(&timeout, "timeout", 0, "maximum wall-clock time for the pipeline (e.g. 5m, 30s); 0 means no limit")
	cmd.Flags().StringArrayVar(&vars, "var", nil, "set a pipeline context variable: --var key=value (repeatable)")
	cmd.Flags().StringVar(&varFile, "var-file", "", "load pipeline context variables from a JSON object file")
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
		workdir        string
		defaultModel   string
		outContextPath string
		timeout        time.Duration
		vars           []string
		varFile        string
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
			slog.Info("resuming from checkpoint", "node", lastNodeID)

			// Apply --var-file values, then --var overrides.
			if err := applyVarFile(pctx, varFile); err != nil {
				return err
			}
			if err := applyVars(pctx, vars); err != nil {
				return err
			}

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
			if timeout > 0 {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, timeout)
				defer cancel()
			}
			if runErr := eng.Execute(ctx, lastNodeID); runErr != nil {
				return runErr
			}
			return writeOutputContext(outContextPath, pctx)
		},
	}

	cmd.Flags().StringVar(&workdir, "workdir", ".", "working directory for agent file operations")
	cmd.Flags().StringVar(&defaultModel, "model", "anthropic:claude-sonnet-4-6", "default LLM model")
	cmd.Flags().StringVar(&outContextPath, "output-context", "", "write final pipeline context as JSON to this file")
	cmd.Flags().DurationVar(&timeout, "timeout", 0, "maximum wall-clock time for the pipeline (e.g. 5m, 30s); 0 means no limit")
	cmd.Flags().StringArrayVar(&vars, "var", nil, "set a pipeline context variable: --var key=value (repeatable)")
	cmd.Flags().StringVar(&varFile, "var-file", "", "load pipeline context variables from a JSON object file")
	return cmd
}

// ─── version ──────────────────────────────────────────────────────────────────

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version and build information",
		RunE: func(_ *cobra.Command, _ []string) error {
			info, ok := debug.ReadBuildInfo()
			if !ok {
				fmt.Println("attractor (build info unavailable)")
				return nil
			}

			version := info.Main.Version
			if version == "" || version == "(devel)" {
				version = "dev"
			}

			var revision, buildTime string
			for _, s := range info.Settings {
				switch s.Key {
				case "vcs.revision":
					revision = s.Value
					if len(revision) > 12 {
						revision = revision[:12]
					}
				case "vcs.time":
					buildTime = s.Value
				}
			}

			fmt.Printf("attractor %s\n", version)
			fmt.Printf("  module:  %s\n", info.Main.Path)
			fmt.Printf("  go:      %s\n", info.GoVersion)
			if revision != "" {
				fmt.Printf("  commit:  %s\n", revision)
			}
			if buildTime != "" {
				fmt.Printf("  built:   %s\n", buildTime)
			}
			return nil
		},
	}
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func executePipeline(
	ctx context.Context,
	dotFile, workdir, defaultModel, checkpointPath, outContextPath, seed string,
	varFile string,
	vars []string,
	resumeFromNodeID string,
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
	if err := applyVarFile(pctx, varFile); err != nil {
		return err
	}
	if err := applyVars(pctx, vars); err != nil {
		return err
	}

	// Build handler registry.
	reg := buildRegistry(workdir, defaultModel)

	// Build and run engine.
	eng, err := pipeline.NewEngine(p, reg, pctx, checkpointPath)
	if err != nil {
		return fmt.Errorf("build engine: %w", err)
	}

	sctx := signalContext(ctx)
	if runErr := eng.Execute(sctx, resumeFromNodeID); runErr != nil {
		return runErr
	}
	return writeOutputContext(outContextPath, pctx)
}

// writeOutputContext marshals pctx as JSON and writes it to path.
// A blank path is a no-op.
func writeOutputContext(path string, pctx *pipeline.PipelineContext) error {
	if path == "" {
		return nil
	}
	data, err := json.MarshalIndent(pctx.Snapshot(), "", "  ")
	if err != nil {
		return fmt.Errorf("marshal output context: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write output context %q: %w", path, err)
	}
	slog.Info("output context written", "path", path)
	return nil
}

// applyVars parses a slice of "key=value" strings and injects them into pctx.
// Returns an error for any entry that does not contain an "=" separator.
func applyVars(pctx *pipeline.PipelineContext, vars []string) error {
	for _, v := range vars {
		idx := strings.IndexByte(v, '=')
		if idx < 0 {
			return fmt.Errorf("--var %q: expected key=value format", v)
		}
		key, val := v[:idx], v[idx+1:]
		if key == "" {
			return fmt.Errorf("--var %q: key must not be empty", v)
		}
		pctx.Set(key, val)
	}
	return nil
}

// applyVarFile loads a JSON object from path and injects each key into pctx.
// All values are stored as strings (fmt.Sprintf("%v", v)) for consistency with --var.
// A blank path is a no-op. Returns an error if the file is missing, not valid JSON,
// or the top-level value is not a JSON object.
func applyVarFile(pctx *pipeline.PipelineContext, path string) error {
	if path == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("--var-file: read %q: %w", path, err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		// Could be non-object JSON — give a clear message.
		var top any
		if jsonErr := json.Unmarshal(data, &top); jsonErr == nil {
			return fmt.Errorf("--var-file %q: top-level value must be a JSON object", path)
		}
		return fmt.Errorf("--var-file %q: invalid JSON: %w", path, err)
	}
	for k, v := range raw {
		pctx.Set(k, fmt.Sprintf("%v", v))
	}
	return nil
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
	reg.Register("http", &handlers.HTTPHandler{})
	reg.Register("assert", &handlers.AssertHandler{})
	reg.Register("sleep", &handlers.SleepHandler{})
	reg.Register("switch", &handlers.SwitchHandler{})
	reg.Register("env", &handlers.EnvHandler{})
	reg.Register("read_file", &handlers.ReadFileHandler{})
	reg.Register("write_file", &handlers.WriteFileHandler{})
	reg.Register("json_extract", &handlers.JSONExtractHandler{})
	reg.Register("split", &handlers.SplitHandler{})
	reg.Register("map", &handlers.MapHandler{DefaultModel: defaultModel, Workdir: workdir})
	reg.Register("prompt", &handlers.PromptHandler{DefaultModel: defaultModel})
	reg.Register("json_decode", &handlers.JSONDecodeHandler{})
	reg.Register("exec", &handlers.ExecHandler{Workdir: workdir})
	reg.Register("json_pack", &handlers.JSONPackHandler{})
	reg.Register("regex", &handlers.RegexHandler{})
	reg.Register("string_transform", &handlers.StringTransformHandler{})
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
