# Sprint 004: Agent Hardening

## Overview

Three spec-required features are missing from the current implementation:

1. **Max-turns limit** — the coding-agent-loop spec says the loop must stop when a
   configurable turn ceiling is reached, returning an error. The current implementation
   only has loop detection (repeated identical calls); a runaway agent that calls
   different tools indefinitely is unbounded.

2. **`search_file` tool** — the built-in tool set (`read_file`, `write_file`,
   `list_dir`, `run_command`) is missing a grep/search capability. Coding agents
   routinely need to search for symbols, patterns, or text across files; without this
   tool they fall back to `read_file` on every file, which is slow and token-expensive.

3. **Validator: fan_out/fan_in pairing** — `findFanIn` in the engine returns an error
   at runtime if no fan_in is reachable from a fan_out node. The validator should catch
   this structurally at lint time.

4. **Pipeline `--timeout` flag** — there is no wall-clock cap on pipeline execution.
   A hung tool call (e.g. `run_command` waiting on a subprocess) or a stuck agent can
   hold a pipeline forever. A top-level `context.WithTimeout` wired to a CLI flag
   closes this gap.

## Use Cases

1. **Bounded agent run**: `attractor run pipeline.dot --max-turns 10` — agent stops after
   10 LLM turns and returns whatever text it has produced so far
2. **Pattern-aware coding**: agent calls `search_file` with `pattern="func NewEngine"`
   to locate a function before reading it, instead of reading the whole file
3. **Lint-time fan_in check**: `attractor lint bad.dot` reports `fan_out node "fork"
   has no reachable fan_in node` before any execution
4. **Timed-out pipeline**: `attractor run pipeline.dot --timeout 5m` — pipeline is
   cancelled after 5 minutes with a clear timeout error

## Architecture

### Max-turns in agent loop

`CodingAgentLoop` gets a `maxTurns int` field (default 50). The main `for` loop
increments a counter; when it exceeds `maxTurns` it returns a `MaxTurnsError`.

A `WithMaxTurns(n int) Option` is added alongside the existing `WithMaxTokens`.

The `codergen` DOT node accepts a `max_turns` attribute:
```dot
agent [type=codergen, prompt="…", max_turns="10"]
```

### `search_file` tool

```go
// pkg/agent/tools/searchfile.go
type SearchFileTool struct { workdir string }

// Input: { "pattern": "func NewEngine", "path": "pkg/pipeline" }
// Output: matched lines with file:line prefix, or "no matches"
// Uses filepath.Walk + strings.Contains (no regex for simplicity)
```

Uses `filepath.Walk` over the provided `path` (or workdir if empty), reads each file,
and returns all matching lines in `file:line: content` format, capped at 200 lines.
Path traversal protection via `safePath` (same as `read_file`).

### Validator fan_out/fan_in pairing

New lint rule in `pkg/pipeline/validator.go`:

> For every node of type `fan_out`, perform a BFS on outgoing edges. If no node of
> type `fan_in` is reachable, emit a `LintError`.

### `--timeout` CLI flag

`run` and `resume` subcommands gain `--timeout duration` flag (default `0` = no limit).
When non-zero, `executePipeline` wraps the context with `context.WithTimeout`.

## Files

| File | Action | Description |
|------|--------|-------------|
| `pkg/agent/loop.go` | **Modify** | Add `maxTurns` field, `WithMaxTurns`, turn counter, `MaxTurnsError` |
| `pkg/agent/errors.go` | **Create** | `MaxTurnsError` type |
| `pkg/agent/tools/searchfile.go` | **Create** | `search_file` tool |
| `pkg/agent/tools/tools_test.go` | **Modify** | Tests for `search_file` |
| `pkg/agent/agent_test.go` | **Modify** | Test max-turns limit |
| `pkg/pipeline/validator.go` | **Modify** | Fan_out/fan_in pairing rule |
| `pkg/pipeline/pipeline_test.go` | **Modify** | Test new validator rule |
| `cmd/attractor/main.go` | **Modify** | `--timeout` flag on run/resume |

## Definition of Done

### Functional
- [ ] Agent loop stops at `maxTurns` and returns a descriptive error
- [ ] `search_file` tool finds matching lines and respects path traversal protection
- [ ] `attractor lint` reports an error for a fan_out with no reachable fan_in
- [ ] `attractor run pipeline.dot --timeout 100ms` cancels cleanly (no goroutine leak)

### Correctness
- [ ] `TestAgentLoop_MaxTurns` — loop terminates and error message mentions turns
- [ ] `TestSearchFileTool` — finds patterns, returns correct `file:line: text` format
- [ ] `TestSearchFileTool_NoTraversal` — `../` in path is rejected
- [ ] `TestValidate_FanOutNoFanIn` — lint error emitted

### Quality
- [ ] `go test -race ./...` — all green
- [ ] `golangci-lint run ./...` — 0 issues
- [ ] `codergen` handler passes `max_turns` from node attr to agent loop

## Dependencies

- Sprint 003 complete
- No new external dependencies
