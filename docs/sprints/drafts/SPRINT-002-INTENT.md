# Sprint 002 Intent: OpenAI Provider Adapter

## Seed

> "I want to build sprint 2"

Clarified through interview: add the OpenAI LLM provider adapter so that Attractor
pipelines can route nodes to GPT-4o (and other OpenAI models) in addition to Claude.

## Context

Sprint 001 delivered a complete, tested Attractor pipeline engine with only the
Anthropic provider adapter implemented. The unified LLM client was designed from the
start to support multiple providers via a registry pattern (`llm.RegisterProvider` +
`init()`). Sprint 002 completes the "unified" promise for OpenAI.

### Sprint 001 State

- `pkg/llm/providers/anthropic.go` — full implementation, tested, deployed
- `pkg/llm/client.go` — registry pattern; `NewClient("provider:model")` factory
- `pkg/agent/loop.go` — uses `llm.Client` interface; provider-agnostic
- All tests pass; `golangci-lint` clean
- `docs/specs/` empty; `CLAUDE.md` absent; `tests/` empty; no `parallel.dot`

### Interview Decisions

| Decision | Choice |
|----------|--------|
| Primary focus | OpenAI provider adapter |
| Gemini adapter | No — out of scope for Sprint 002 |
| SDK | `github.com/sashabaranov/go-openai` (sashabaranov) |

## Relevant Codebase Areas

- `pkg/llm/providers/anthropic.go` — reference implementation to follow
- `pkg/llm/types.go` — unified types the adapter must map to/from
- `pkg/llm/client.go` — `RegisterProvider` + `NewClient` registry
- `cmd/attractor/main.go` — blank-import of providers package
- `go.mod` — needs new dependency

## Constraints

- Must follow the existing `init()` registry pattern (see `anthropic.go`)
- Must implement both `Complete()` and `Stream()` on `llm.Client`
- The OpenAI message format differs significantly from Anthropic:
  - Tool results: OpenAI uses `role: "tool"` messages; we use `role: "user"` + `tool_result` blocks
  - Tool calls: OpenAI uses `ToolCalls []ToolCall`; we use `ContentBlock{type: "tool_use"}`
  - System prompt: OpenAI sends as `role: "system"` in messages array
- Must respect context cancellation and propagate errors through the typed taxonomy
- Must not break existing Anthropic tests
- `go test -race ./...` and `golangci-lint` must remain clean

## Success Criteria

1. `attractor run pipeline.dot --seed "..."` works with `model="openai:gpt-4o"` on a node
2. The coding agent loop executes tool calls via GPT-4o and processes results correctly
3. Unit tests cover: request mapping, response mapping, tool call round-trip, error classification, rate-limit retry
4. Model-routing example (`examples/multi_provider.dot`) shows Anthropic + OpenAI nodes in one pipeline

## Verification Strategy

- **Unit tests**: mock HTTP server returning canned OpenAI JSON responses; verify our
  unified types are produced correctly for each response shape (text, tool_calls, error)
- **Integration test** (opt-in via `OPENAI_API_KEY`): run a minimal tool-use loop against
  real API; skipped in CI without the key
- **Edge cases**:
  - Assistant message with multiple tool calls
  - Tool result with `is_error: true` (sent as content, not an API error)
  - `finish_reason: "length"` → `StopReasonMaxTokens`
  - Rate limit 429 → `RateLimitError` → retry with backoff
  - Auth failure 401 → `AuthError` → no retry

## Uncertainty Assessment

- **Correctness uncertainty**: Medium — OpenAI ↔ unified type mapping has several non-obvious
  corners (especially tool message format differences)
- **Scope uncertainty**: Low — clearly bounded to one provider + CLAUDE.md/docs
- **Architecture uncertainty**: Low — follows existing Anthropic adapter pattern

## Open Questions

1. Does `sashabaranov/go-openai` support function-calling / tool-use in the same way
   as the OpenAI HTTP API? (Yes — via `Tools []Tool` in `ChatCompletionRequest`)
2. How should we handle OpenAI's `parallel_tool_calls` (multiple tool calls in one turn)?
   — Our loop already handles this; need to verify the adapter maps all calls, not just the first.
3. Should Sprint 002 also add `CLAUDE.md` and clone `docs/specs/`? — Yes, as secondary
   work items since they're quick and unblock future sprints.
