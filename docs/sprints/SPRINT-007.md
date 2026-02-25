# Sprint 007: Observability & Configuration

## Overview

Three improvements that make attractor easier to operate and integrate into
scripts, pipelines, and CI systems:

1. **Structured logging (`log/slog`)** — replace all diagnostic `fmt.Printf`
   calls with `slog` so that the log level and output format are configurable.
   `--log-level` (debug/info/warn/error, default info) and `--log-format`
   (text/json, default text) are added as persistent flags on the root command.

2. **`--output-context file.json`** — after the pipeline finishes, write the
   final `PipelineContext` snapshot as JSON to the given path.  Enables
   shell-script chaining: the next stage reads context variables produced by
   the pipeline without needing a checkpoint file.

3. **`codergen` `system_prompt` attribute** — allow per-node system-prompt
   override in the DOT file.  The `WithSystem` option already exists on
   `CodingAgentLoop`; this sprint simply wires the node attribute into the
   handler.

## Use Cases

1. **CI JSON log aggregation**: `attractor run pipeline.dot --log-format json 2>&1 | jq .`
2. **Inter-pipeline data passing**: `attractor run stage1.dot --output-context ctx.json && attractor run stage2.dot --var result=$(jq -r .last_output ctx.json)`
3. **Domain expert system prompt**: `system_prompt="You are a security expert. …"` on a specific node without changing the default for other nodes.

## Architecture

### Structured logging

Use `log/slog` package-level functions (`slog.Info`, `slog.Debug`, etc.) after
configuring `slog.SetDefault` in `main.go`.  No logger threading needed.

Mapping:
- `[attractor] executing node …`  → `slog.Info("executing node", …)`
- `[attractor] pipeline complete …` → `slog.Info("pipeline complete", …)`
- `[attractor] fan_out branch starting/complete` → `slog.Debug`
- `[tool] …` → `slog.Debug("tool call", …)`
- `[error] …` → `slog.Warn("agent error", …)`
- `[steering] …` → `slog.Warn("agent steering", …)`
- `lint OK` and `resuming from node` → `slog.Info`

The `version` and `lint OK` output is user-facing (not diagnostic), so those
`fmt.Printf` calls are kept as-is.

### `--output-context`

Added to `run` and `resume` subcommands. `executePipeline` gains an
`outContextPath string` parameter; after `eng.Execute` returns nil, if the
path is non-empty the context snapshot is marshalled with `encoding/json` and
written with `os.WriteFile`.

### `codergen` `system_prompt` attribute

In `codergen.go`, read `node.Attrs["system_prompt"]` and append
`agent.WithSystem(sp)` to the option slice before constructing the loop.

## Files

| File | Action | Description |
|------|--------|-------------|
| `cmd/attractor/main.go` | **Modify** | `--log-level`, `--log-format` persistent flags; `--output-context` on run/resume; slog setup |
| `pkg/pipeline/engine.go` | **Modify** | Replace `fmt.Printf` with `slog.Info`/`slog.Debug` |
| `pkg/pipeline/handlers/codergen.go` | **Modify** | Replace `fmt.Printf` with `slog.*`; wire `system_prompt` attribute |

## Definition of Done

### Functional
- [ ] `attractor run … --log-format json` emits JSON log lines to stderr
- [ ] `attractor run … --log-level debug` shows per-node and branch-level logs
- [ ] `attractor run pipeline.dot --output-context out.json` writes context JSON
- [ ] `codergen` node with `system_prompt="…"` passes it to the agent loop

### Correctness
- [ ] `TestStructuredLogging` — slog handler receives expected records
- [ ] `TestOutputContext` — JSON file written with correct keys after run
- [ ] `TestCodergenSystemPrompt` — system_prompt attr wired into WithSystem

### Quality
- [ ] `go test -race ./...` — all green
- [ ] `golangci-lint run ./...` — 0 issues

## Dependencies

- Sprint 006 complete
- No new external dependencies (`log/slog` is stdlib since Go 1.21)
