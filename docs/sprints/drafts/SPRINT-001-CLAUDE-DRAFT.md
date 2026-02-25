# Sprint 001: Attractor — Core Pipeline Engine (Python)

## Overview

Attractor is a DOT-based pipeline runner for orchestrating multi-stage AI workflows.
This sprint builds the foundational three-layer stack in **Python**:

1. **Unified LLM Client** — a thin, provider-agnostic wrapper over `anthropic`, `openai`,
   and `google-generativeai` SDKs, implementing the `unified-llm-spec`
2. **Coding Agent Loop** — an autonomous tool-using LLM session implementing the
   `coding-agent-loop-spec`, built on top of the unified client
3. **Attractor Pipeline Engine** — the DOT-graph executor implementing `attractor-spec`,
   using the agent loop as its `codergen` handler

The result: a runnable `attractor run pipeline.dot` CLI that takes a seed, traverses a
graph of LLM-backed nodes, and produces observable output. Sprint 1 targets a working
end-to-end pipeline with a curated subset of node types. Full spec coverage is Sprint 2+.

## Use Cases

1. **Hello-World Pipeline**: `start → codergen("implement a fizzbuzz function") → exit`
   — verifies the full stack works end-to-end
2. **Iterative coding pipeline**: `start → implement → validate → [pass: exit | fail: implement]`
   — a two-node loop that retries until tests pass
3. **Human gate**: `start → implement → wait.human("review the code") → exit`
   — pauses for user approval before continuing
4. **Model switching**: Change the model on any node via the Model Stylesheet; the
   unified client routes to the correct provider transparently

## Architecture

```
┌─────────────────────────────────────────────────────┐
│                   CLI: attractor run                 │
└───────────────────┬─────────────────────────────────┘
                    │
┌───────────────────▼─────────────────────────────────┐
│              Pipeline Engine (attractor-spec)        │
│  ┌──────────┐  ┌──────────┐  ┌───────────────────┐  │
│  │  DOT     │  │  State   │  │  Handler Registry │  │
│  │  Parser  │  │  Context │  │  (pluggable)      │  │
│  └────┬─────┘  └────┬─────┘  └────────┬──────────┘  │
│       │             │                 │              │
│  ┌────▼─────────────▼─────────────────▼──────────┐  │
│  │           Execution Engine (traverse graph)    │  │
│  └───────────────────────┬────────────────────────┘  │
└──────────────────────────│──────────────────────────┘
                           │ codergen handler
┌──────────────────────────▼──────────────────────────┐
│           Coding Agent Loop (coding-agent-loop-spec) │
│  Session → LLM call → tool execution → loop detect  │
└──────────────────────────┬──────────────────────────┘
                           │
┌──────────────────────────▼──────────────────────────┐
│         Unified LLM Client (unified-llm-spec)        │
│  anthropic | openai | gemini  →  provider adapters   │
└─────────────────────────────────────────────────────┘
```

## Implementation Plan

### Phase 1: Unified LLM Client (~20%)

**Files:**
- `attractor/llm/__init__.py`
- `attractor/llm/types.py` — Message, ContentBlock, ToolCall, ToolResult, GenerateRequest, GenerateResponse
- `attractor/llm/client.py` — `UnifiedClient` with `complete()` and `stream()`
- `attractor/llm/providers/anthropic.py` — Anthropic adapter
- `attractor/llm/providers/openai.py` — OpenAI adapter (stub for Sprint 1)
- `attractor/llm/errors.py` — error taxonomy + retry logic

**Tasks:**
- [ ] Define `Message`, `ContentBlock` (text, image, tool_use, tool_result), `ToolDefinition` types
- [ ] `UnifiedClient.complete(request) -> GenerateResponse` — blocking, non-streaming
- [ ] `UnifiedClient.stream(request) -> AsyncIterator[StreamEvent]` — streaming with deltas
- [ ] Anthropic adapter: map unified types ↔ `anthropic` SDK types
- [ ] Exponential backoff on rate limit / server errors
- [ ] Model string parsing: `"anthropic:claude-sonnet-4-6"` → route to correct adapter
- [ ] Unit tests: mock provider, verify type mapping

### Phase 2: Coding Agent Loop (~25%)

**Files:**
- `attractor/agent/__init__.py`
- `attractor/agent/loop.py` — `CodingAgentLoop` class
- `attractor/agent/tools/` — filesystem tools (read_file, write_file, run_command, list_dir)
- `attractor/agent/events.py` — event types for real-time output
- `attractor/agent/session.py` — conversation history management

**Tasks:**
- [ ] `CodingAgentLoop(client, tools, env)` constructor
- [ ] `loop.run(instruction: str) -> AgentResult` — runs until model stops using tools
- [ ] Built-in tools: `read_file`, `write_file`, `run_command`, `list_dir`
- [ ] Conversation truncation: mark with `[TRUNCATED]`, preserve first/last N turns
- [ ] Loop detection: same tool call + args repeated 3x → inject steering message
- [ ] Event emission: `tool_call`, `tool_result`, `llm_turn`, `complete` events
- [ ] Unit tests: mock LLM, verify tool execution and loop termination

### Phase 3: Attractor Pipeline Engine (~35%)

**Files:**
- `attractor/pipeline/__init__.py`
- `attractor/pipeline/parser.py` — DOT file → `Pipeline` AST
- `attractor/pipeline/validator.py` — structural linting
- `attractor/pipeline/engine.py` — `PipelineEngine` traversal loop
- `attractor/pipeline/handlers/` — node handler registry + built-in handlers
- `attractor/pipeline/state.py` — `PipelineContext` key-value store + checkpoints
- `attractor/pipeline/conditions.py` — condition expression evaluator

**Tasks:**
- [ ] DOT parser: use `pydot` or `graphviz` lib; extract nodes, edges, attributes
- [ ] `Pipeline` dataclass: nodes (id, type, prompt, attrs), edges (src, dst, condition)
- [ ] Validator: one start node, one exit node, all referenced nodes exist, reachable
- [ ] `PipelineContext`: thread-safe `dict`, `set(key, val)`, `get(key)`, serializable
- [ ] Execution engine: `start → handler(node) → evaluate_edges → next_node → repeat`
- [ ] Condition evaluator: parse `"status == 'pass'"` expressions against context
- [ ] Handlers (Sprint 1 subset):
  - `start` — initializes context with seed
  - `exit` — finalizes, returns result
  - `codergen` — runs `CodingAgentLoop` with node's prompt
  - `wait.human` — prompts user via CLI, stores response in context
  - `set` — sets a context variable (useful for testing)
- [ ] Checkpoint: serialize context to `.attractor-checkpoint.json` on each node completion
- [ ] Unit tests: parse sample DOT files, verify traversal, mock handlers

### Phase 4: CLI + Integration (~20%)

**Files:**
- `attractor/cli.py` — `attractor run <pipeline.dot> [--seed "..."]`
- `examples/hello_world.dot` — trivial pipeline
- `examples/coding_loop.dot` — implement + validate loop
- `pyproject.toml` — package config, deps, entry points
- `README.md` — quickstart

**Tasks:**
- [ ] `attractor run pipeline.dot --seed "build a REST API"` — full end-to-end
- [ ] `attractor lint pipeline.dot` — validates without running
- [ ] `attractor resume pipeline.dot` — resumes from checkpoint
- [ ] Progress output: print node name + type as pipeline traverses
- [ ] `examples/hello_world.dot`: start → codergen → exit
- [ ] `examples/coding_loop.dot`: start → implement → run_tests → [pass: exit | fail: implement]
- [ ] Integration test: run hello_world.dot against real Claude API

## Files Summary

| File | Action | Purpose |
|------|--------|---------|
| `attractor/llm/types.py` | Create | Unified type system for LLM messages |
| `attractor/llm/client.py` | Create | Provider-agnostic LLM client |
| `attractor/llm/providers/anthropic.py` | Create | Anthropic adapter |
| `attractor/llm/errors.py` | Create | Error taxonomy + retry |
| `attractor/agent/loop.py` | Create | Coding agent loop |
| `attractor/agent/tools/` | Create | File/shell tools |
| `attractor/agent/events.py` | Create | Event types |
| `attractor/pipeline/parser.py` | Create | DOT → Pipeline AST |
| `attractor/pipeline/validator.py` | Create | Structural linting |
| `attractor/pipeline/engine.py` | Create | Graph traversal engine |
| `attractor/pipeline/handlers/` | Create | Node handler registry |
| `attractor/pipeline/state.py` | Create | Context + checkpoints |
| `attractor/cli.py` | Create | CLI entrypoint |
| `examples/*.dot` | Create | Sample pipelines |
| `pyproject.toml` | Create | Package config |
| `tests/` | Create | Unit + integration tests |

## Definition of Done

- [ ] `attractor run examples/hello_world.dot --seed "write a hello world in Python"` runs end-to-end
- [ ] Pipeline traverses start → codergen → exit, calling Claude API
- [ ] `attractor lint examples/hello_world.dot` passes cleanly
- [ ] `attractor resume` loads from checkpoint and continues
- [ ] wait.human node pauses and accepts CLI input
- [ ] Model can be overridden via `model_stylesheet` attribute in DOT file
- [ ] Unit tests pass for all three layers
- [ ] All three NLSpec files cloned into `docs/specs/` for offline reference

## Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| DOT parsing is complex (nested attrs, escaping) | Medium | High | Use `pydot` library; add test fixtures |
| Condition expression eval is ambiguous | Medium | Medium | Start with simple `key == "value"` only; expand later |
| Coding agent loop ↔ handler coupling is unclear | Low | High | Keep codergen handler thin — just wraps agent loop |
| Spec has contradictions between docs | Low | Medium | Treat attractor-spec as authoritative; note deviations |
| Sprint scope too large for one iteration | High | Medium | Phase 1–2 (LLM client + agent loop) are independently useful; ship those first |

## Security Considerations

- `run_command` tool in agent loop executes arbitrary shell commands — must run in a
  sandboxed working directory, never as root
- `write_file` tool must be path-sandboxed (only allow writes within project directory)
- API keys from environment only — never hardcoded or stored in checkpoints

## Dependencies

- No prior sprints
- External: `anthropic`, `pydot`, `click` (CLI), `pytest`, `python-dotenv`
- Optional: `openai`, `google-generativeai` (adapters stubbed for Sprint 1)

## Open Questions

1. **Language confirmed as Python?** TypeScript has the most mature community impl
   (brynary/attractor) — should we use it instead for easier spec comparison?
2. **Full unified-llm-spec or just wrap `anthropic` SDK directly for Sprint 1?**
   The unified client is a significant chunk of work; wrapping anthropic directly is faster.
3. **How strict is spec conformance for Sprint 1?** Cover core happy path now, fill gaps in Sprint 2?
4. **Parallel fan-out handler** — skip for Sprint 1 or include?
