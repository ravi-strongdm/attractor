# Sprint 001: Attractor — Core Pipeline Engine (Go)

## Overview

Attractor is a DOT-based pipeline runner for orchestrating multi-stage AI workflows,
conforming strictly to the three NLSpecs published at `github.com/strongdm/attractor`.

This sprint builds the complete three-layer stack in **Go**, following the specs in
dependency order:

1. **Unified LLM Client** (`pkg/llm/`) — provider-agnostic LLM interface over Anthropic,
   OpenAI, and Google Gemini, conforming to `unified-llm-spec.md`
2. **Coding Agent Loop** (`pkg/agent/`) — autonomous tool-using agent that reads files,
   edits code, and runs commands, conforming to `coding-agent-loop-spec.md`
3. **Attractor Pipeline Engine** (`pkg/pipeline/`) — DOT graph executor with node
   handlers, state management, and checkpoints, conforming to `attractor-spec.md`

The deliverable is a runnable `attractor run pipeline.dot --seed "..."` CLI. Go's
native concurrency model makes parallel fan-out handlers idiomatic, and explicit error
returns map cleanly onto the spec's error taxonomy.

The three spec files are cloned into `docs/specs/` for offline reference and treated as
the authoritative source of truth. Any deviation from a MUST-level requirement is a
sprint blocker.

## Use Cases

1. **Hello-world pipeline**: `start → codergen("write a hello world") → exit`
   — full stack smoke test
2. **Iterative coding loop**: `start → implement → validate → [pass→exit | fail→implement]`
   — verifies condition edge routing and cycles
3. **Parallel fan-out**: `start → fan_out → [node_a, node_b] → fan_in → exit`
   — verifies goroutine-based parallel execution and synchronization
4. **Human gate**: `start → implement → wait.human("approve?") → exit`
   — pauses pipeline for CLI input
5. **Checkpoint resume**: kill mid-run, `attractor resume pipeline.dot`
   — verifies context serialization and replay from checkpoint
6. **Model switching**: set `model="openai:gpt-4o"` on a node via the Model Stylesheet
   — verifies the unified client's provider routing

## Architecture

```
cmd/
└── attractor/
    └── main.go              CLI entry (cobra: run, lint, resume)

pkg/
├── llm/                     Unified LLM Client (unified-llm-spec.md)
│   ├── types.go             Message, ContentBlock, ToolCall, GenerateRequest/Response
│   ├── client.go            UnifiedClient interface + constructor
│   ├── errors.go            Error taxonomy, retryability, exponential backoff
│   ├── stream.go            Streaming via channels / iter.Seq
│   └── providers/
│       ├── anthropic.go     Anthropic adapter
│       ├── openai.go        OpenAI adapter (stub)
│       └── gemini.go        Gemini adapter (stub)
│
├── agent/                   Coding Agent Loop (coding-agent-loop-spec.md)
│   ├── loop.go              CodingAgentLoop: Run(instruction) → AgentResult
│   ├── session.go           Conversation history + truncation
│   ├── events.go            Event types (ToolCall, ToolResult, LLMTurn, Complete)
│   ├── loopdetect.go        Repeated-call detection + steering injection
│   └── tools/
│       ├── registry.go      ToolRegistry: register/lookup
│       ├── readfile.go      read_file
│       ├── writefile.go     write_file (sandboxed to workdir)
│       ├── runcommand.go    run_command (sandboxed workdir, timeout)
│       └── listdir.go       list_dir
│
└── pipeline/                Attractor Pipeline Engine (attractor-spec.md)
    ├── ast.go               Pipeline, Node, Edge, Condition types
    ├── parser.go            DOT → Pipeline AST (via go-graphviz)
    ├── validator.go         Structural linting (one start/exit, reachability, etc.)
    ├── engine.go            PipelineEngine: Execute(ctx) traversal loop
    ├── state.go             PipelineContext: thread-safe KV + checkpoint serialization
    ├── conditions.go        Condition expression evaluator
    ├── stylesheet.go        Model Stylesheet: CSS-like LLM config
    └── handlers/
        ├── registry.go      HandlerRegistry
        ├── start.go         start handler
        ├── exit.go          exit handler
        ├── codergen.go      codergen → CodingAgentLoop
        ├── human.go         wait.human → CLI Interviewer
        ├── set.go           set context variable
        ├── fanout.go        parallel fan-out (goroutines + WaitGroup)
        └── fanin.go         fan-in (barrier synchronization)

docs/specs/                  Cloned NLSpec files (offline reference)
├── attractor-spec.md
├── unified-llm-spec.md
└── coding-agent-loop-spec.md

examples/
├── hello_world.dot
├── coding_loop.dot
└── parallel.dot

go.mod
go.sum
```

## Implementation Plan

### Phase 1: Project Scaffold + Spec Reference (~5%)

**Files:**
- `go.mod` — module `github.com/[user]/attractor`, Go 1.22+
- `docs/specs/` — clone the three NLSpec files
- `Makefile` — `make build`, `make test`, `make lint`

**Tasks:**
- [ ] `go mod init github.com/[user]/attractor`
- [ ] Add deps: `github.com/goccy/go-graphviz`, `github.com/anthropics/anthropic-sdk-go`, `github.com/spf13/cobra`, `github.com/stretchr/testify`
- [ ] Clone spec files into `docs/specs/`
- [ ] Makefile targets: `build`, `test`, `lint` (golangci-lint)

---

### Phase 2: Unified LLM Client (~20%)

Conforms to: `docs/specs/unified-llm-spec.md`

**Files:** `pkg/llm/`

**Tasks:**
- [ ] `types.go`: `Role`, `ContentBlock` (Text, Image, ToolUse, ToolResult, Thinking), `Message`, `ToolDefinition`, `GenerateRequest`, `GenerateResponse`, `StreamEvent`
- [ ] `client.go`: `Client` interface with `Complete(ctx, req) (GenerateResponse, error)` and `Stream(ctx, req) (<-chan StreamEvent, error)`; `NewClient(modelID string) (Client, error)` factory that routes by prefix (`anthropic:`, `openai:`, `gemini:`)
- [ ] `errors.go`: `RateLimitError`, `ServerError`, `AuthError`, `ContextLengthError`, `ContentFilterError` — each implements `Retryable() bool`; `WithRetry(ctx, fn, maxAttempts)` with exponential backoff + jitter
- [ ] `providers/anthropic.go`: map `GenerateRequest` ↔ `anthropic-sdk-go` types; handle tool use, streaming, thinking blocks
- [ ] `providers/openai.go`: stub — returns `ErrNotImplemented`
- [ ] `providers/gemini.go`: stub — returns `ErrNotImplemented`
- [ ] Table-driven unit tests: mock provider, verify request/response mapping, verify retry on rate limit

**Spec conformance gates (MUST):**
- Model ID format `provider:model-name` parsed correctly
- `Complete()` MUST populate `StopReason`, `Usage.InputTokens`, `Usage.OutputTokens`
- Streaming MUST emit `StreamEvent{Type: "delta", Text: "..."}` then `StreamEvent{Type: "complete"}`
- Rate-limit retry MUST use exponential backoff with jitter, respect `Retry-After` header

---

### Phase 3: Coding Agent Loop (~25%)

Conforms to: `docs/specs/coding-agent-loop-spec.md`

**Files:** `pkg/agent/`

**Tasks:**
- [ ] `session.go`: `Session` holds `[]Message`; `Append(msg)`, `Truncate(maxTokens int)` — truncation inserts `[TRUNCATED — N messages omitted]` marker between preserved head/tail
- [ ] `events.go`: `Event` interface; `ToolCallEvent`, `ToolResultEvent`, `LLMTurnEvent`, `CompleteEvent`, `ErrorEvent`
- [ ] `tools/registry.go`: `ToolRegistry` maps name → `Tool` interface (`Name() string`, `Description() string`, `Schema() json.RawMessage`, `Execute(ctx, input json.RawMessage) (json.RawMessage, error)`)
- [ ] `tools/readfile.go`: reads file relative to workdir; path traversal check
- [ ] `tools/writefile.go`: writes file relative to workdir; path traversal check; creates parent dirs
- [ ] `tools/runcommand.go`: `exec.CommandContext` with 30s timeout; capture stdout+stderr; run in workdir
- [ ] `tools/listdir.go`: `os.ReadDir` with optional pattern glob
- [ ] `loop.go`: `CodingAgentLoop` with `client llm.Client`, `tools ToolRegistry`, `workdir string`, `eventCh chan<- Event`; `Run(ctx, instruction) (AgentResult, error)` — LLM call → tool execution loop until no tool calls; emit events on each step
- [ ] `loopdetect.go`: track last N `(toolName, inputHash)` pairs; if same call appears 3× inject steering message `"You appear to be stuck. Try a different approach."`
- [ ] Unit tests: mock client returning canned responses with tool_use blocks; verify tools execute correctly; verify loop detection triggers at 3 repeats; verify truncation

**Spec conformance gates (MUST):**
- Agent loop MUST continue until model response contains zero tool_use blocks
- Truncation MUST preserve first turn (seed instruction) and last N turns
- Loop detection MUST inject steering, not abort, on first detection
- `run_command` MUST have a timeout (30s default, configurable)
- All tool errors MUST be returned as `tool_result` with `is_error: true`, not propagated as Go errors to the LLM caller

---

### Phase 4: Attractor Pipeline Engine (~35%)

Conforms to: `docs/specs/attractor-spec.md`

**Files:** `pkg/pipeline/`

**Tasks:**
- [ ] `ast.go`: `Pipeline`, `Node{ID, Type, Attrs map[string]string, Prompt string}`, `Edge{From, To, Condition string}`, `Stylesheet`
- [ ] `parser.go`: use `go-graphviz` to parse DOT; extract node types from `type=` attr; prompts from `prompt=`; conditions from edge `label=`; stylesheet from graph-level attrs
- [ ] `validator.go`: enforce exactly one `start` node, exactly one `exit` node; all edge targets exist; graph is connected; no self-loops on non-loop nodes; emit `[]LintError` with node ID + message
- [ ] `conditions.go`: parse and evaluate `"key == 'value'"`, `"key != 'value'"`, `"key"` (truthy), `"!key"` (falsy), `"a && b"`, `"a || b"` — pure boolean, no arithmetic
- [ ] `stylesheet.go`: parse `model_stylesheet` graph attribute (CSS-like rules); apply model overrides to nodes; supports `type[codergen] { model: "anthropic:claude-opus-4-6" }`
- [ ] `state.go`: `PipelineContext` with `sync.RWMutex`-protected `map[string]any`; `Set(k, v)`, `Get(k) (any, bool)`, `Snapshot() map[string]any`; `SaveCheckpoint(path)` → JSON; `LoadCheckpoint(path)` → restores state + last completed node ID
- [ ] `handlers/start.go`: set `ctx["seed"]` from CLI `--seed` arg; set `ctx["start_time"]`
- [ ] `handlers/exit.go`: set `ctx["exit_time"]`; return `ExitSignal` to stop traversal
- [ ] `handlers/codergen.go`: instantiate `CodingAgentLoop`; render node prompt as Go template against context; call `loop.Run(ctx, renderedPrompt)`; store result in `ctx["last_output"]`
- [ ] `handlers/human.go`: print prompt to stdout; read line from stdin; store in `ctx[node.ID + "_response"]`
- [ ] `handlers/set.go`: evaluate `value=` attr as Go template; set `ctx[key]`
- [ ] `handlers/fanout.go`: collect all outgoing unconditional edges; spawn goroutine per edge target; use `errgroup` for parallel execution with context cancellation
- [ ] `handlers/fanin.go`: barrier — wait until all expected incoming branches have completed (tracked via context counter)
- [ ] `engine.go`: `PipelineEngine{pipeline, context, handlers, checkpointPath}`; `Execute(ctx context.Context) error` — start at start node; call handler; evaluate edges (condition expressions against state); select next node; save checkpoint; repeat until exit node or error; detect cycles via visit count (error if node visited > 50 times)

**Spec conformance gates (MUST):**
- Engine MUST save checkpoint after every node completion
- Engine MUST evaluate ALL outgoing edges and select the first whose condition is true (deterministic: edges ordered by DOT definition order)
- If no condition is true and no unconditional edge exists, engine MUST return an error
- Validator MUST reject pipelines with unreachable nodes
- Parallel fan-out MUST propagate context mutations from sub-branches back to main context after fan-in (merge strategy: last-write-wins per key)

---

### Phase 5: CLI + Examples (~15%)

**Files:** `cmd/attractor/main.go`, `examples/`

**Tasks:**
- [ ] `cobra` root command with subcommands:
  - `attractor run <pipeline.dot> [--seed "..."] [--workdir "."] [--checkpoint ".attractor-checkpoint.json"]`
  - `attractor lint <pipeline.dot>` — validate and print errors
  - `attractor resume <pipeline.dot>` — load checkpoint and continue
- [ ] Progress output: `[node_id] (type) starting...` / `[node_id] ✓ done` per node
- [ ] `examples/hello_world.dot`: `start → codergen → exit`
- [ ] `examples/coding_loop.dot`: `start → implement → run_tests → exit | implement` with condition edges
- [ ] `examples/parallel.dot`: `start → fan_out → [analyze, lint] → fan_in → exit`
- [ ] Integration test: `TestRunHelloWorld` calls `attractor run examples/hello_world.dot --seed "write a Go fizzbuzz"` against real Claude API (skipped in CI unless `INTEGRATION=1`)

## Files Summary

| File | Action | Purpose |
|------|--------|---------|
| `go.mod` / `go.sum` | Create | Go module definition |
| `Makefile` | Create | build, test, lint targets |
| `docs/specs/*.md` | Create | Cloned NLSpec files |
| `pkg/llm/types.go` | Create | Unified type system |
| `pkg/llm/client.go` | Create | Provider-agnostic client interface |
| `pkg/llm/errors.go` | Create | Error taxonomy + retry |
| `pkg/llm/stream.go` | Create | Streaming via channels |
| `pkg/llm/providers/anthropic.go` | Create | Anthropic adapter |
| `pkg/llm/providers/openai.go` | Create | OpenAI stub |
| `pkg/agent/loop.go` | Create | Agent loop: LLM + tools |
| `pkg/agent/session.go` | Create | History + truncation |
| `pkg/agent/events.go` | Create | Event types |
| `pkg/agent/loopdetect.go` | Create | Loop detection + steering |
| `pkg/agent/tools/*.go` | Create | read_file, write_file, run_command, list_dir |
| `pkg/pipeline/ast.go` | Create | Pipeline AST types |
| `pkg/pipeline/parser.go` | Create | DOT → AST parser |
| `pkg/pipeline/validator.go` | Create | Structural linter |
| `pkg/pipeline/engine.go` | Create | Graph traversal engine |
| `pkg/pipeline/state.go` | Create | Context + checkpoints |
| `pkg/pipeline/conditions.go` | Create | Boolean condition evaluator |
| `pkg/pipeline/stylesheet.go` | Create | Model Stylesheet parser |
| `pkg/pipeline/handlers/*.go` | Create | All node handlers |
| `cmd/attractor/main.go` | Create | CLI entry point |
| `examples/*.dot` | Create | Sample pipelines |

## Definition of Done

### Functional
- [ ] `attractor run examples/hello_world.dot --seed "write Go fizzbuzz"` completes without error
- [ ] `attractor lint examples/hello_world.dot` prints no errors
- [ ] `attractor resume` correctly loads checkpoint and skips already-completed nodes
- [ ] `wait.human` node pauses and accepts stdin input
- [ ] `fan_out` / `fan_in` run branches in parallel and merge context correctly
- [ ] Model Stylesheet overrides apply to individual nodes

### Spec Conformance (MUST-level)
- [ ] `unified-llm-spec`: model ID format `provider:model-name` routes correctly
- [ ] `unified-llm-spec`: rate-limit retry uses exponential backoff with jitter
- [ ] `unified-llm-spec`: streaming emits delta events then complete event
- [ ] `coding-agent-loop-spec`: loop terminates when no tool_use blocks returned
- [ ] `coding-agent-loop-spec`: truncation preserves first + last N turns with marker
- [ ] `coding-agent-loop-spec`: loop detection injects steering at 3 repeats
- [ ] `attractor-spec`: checkpoint saved after every node completion
- [ ] `attractor-spec`: edge evaluation is deterministic (DOT definition order)
- [ ] `attractor-spec`: validator rejects unreachable nodes

### Quality
- [ ] `go test ./...` passes
- [ ] `golangci-lint run` passes (no errors)
- [ ] No hardcoded API keys; all credentials from environment
- [ ] `run_command` sandboxed to workdir; path traversal blocked in file tools

## Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| `go-graphviz` DOT parsing edge cases (multiline attrs, escaping) | Medium | High | Fixture-based parser tests; fall back to manual DOT parsing if needed |
| Spec ambiguities between the three docs | Medium | Medium | `attractor-spec.md` is authoritative for the engine; document deviations inline |
| Sprint scope too large (3 specs in one sprint) | High | Medium | Phases 1–3 (LLM client + agent loop) are independently shippable; cut Phase 5 if needed |
| Parallel fan-out state merging races | Medium | High | Fan-in copies sub-context under mutex; test with `-race` flag |
| Condition evaluator edge cases | Low | Medium | Limit to spec's grammar; return error on unsupported syntax |

## Security Considerations

- `run_command`: execute via `exec.CommandContext` only; no shell interpolation; run in
  sandboxed `workdir`; enforce 30s timeout; never run as root
- `write_file`: resolve path with `filepath.Clean`; reject if outside `workdir` via
  `strings.HasPrefix(resolved, workdir)`
- API keys from environment variables only (`ANTHROPIC_API_KEY`, `OPENAI_API_KEY`);
  never logged or stored in checkpoints
- Checkpoint files: store only pipeline state, never API keys or secrets

## Dependencies

- No prior sprints
- External Go dependencies:
  - `github.com/anthropics/anthropic-sdk-go` — Anthropic provider
  - `github.com/goccy/go-graphviz` — DOT parsing
  - `github.com/spf13/cobra` — CLI
  - `github.com/stretchr/testify` — test assertions
  - `golang.org/x/sync/errgroup` — parallel fan-out
- Spec reference: `github.com/strongdm/attractor` (read-only)
- Community reference: `github.com/brynary/attractor` (TypeScript, for spec clarification)

## Open Questions

1. **Module path**: use `github.com/[user]/attractor` or a different path?
2. **go-graphviz vs manual DOT parser**: `go-graphviz` is CGO; pure-Go alternative?
   (`github.com/awalterschulze/gographviz` is pure Go — worth considering)
3. **Condition expression grammar**: spec says "minimal boolean language" but doesn't
   give a formal grammar — need to define and document our grammar explicitly
4. **Fan-in sync mechanism**: spec doesn't prescribe how fan-in waits — using a context
   counter is pragmatic but may differ from other implementations
