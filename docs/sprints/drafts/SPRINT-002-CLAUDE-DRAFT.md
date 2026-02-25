# Sprint 002: OpenAI Provider Adapter

## Overview

Sprint 001 delivered a unified LLM client interface (`pkg/llm`) backed exclusively by
the Anthropic provider. The interface was deliberately designed to be multi-provider from
day one — `llm.RegisterProvider(name, factory)` called from `init()` means adding a new
provider requires zero changes to the core engine.

Sprint 002 makes good on the "unified" promise: a full OpenAI adapter using
`github.com/sashabaranov/go-openai`, enabling any pipeline node to route to GPT-4o,
GPT-4o-mini, or any other OpenAI model by setting `model="openai:gpt-4o"`. The adapter
must handle the several meaningful differences between OpenAI's message format and our
unified types, particularly around tool calls and tool results.

Secondary deliverables bring the project to a more professional footing: `CLAUDE.md`
documenting project conventions, a `docs/specs/` clone of the three NLSpec files for
offline reference, a `parallel.dot` example demonstrating fan-out, and a
`multi_provider.dot` example that uses both Anthropic and OpenAI nodes in a single
pipeline.

## Use Cases

1. **Single-provider pipeline with OpenAI**: `start → codergen[model=openai:gpt-4o] → exit`
   — full agent loop via GPT-4o
2. **Multi-provider pipeline**: two codergen nodes, one using `anthropic:claude-sonnet-4-6`
   and one using `openai:gpt-4o` — demonstrates per-node model routing
3. **Tool-use round-trip**: coding agent calls `read_file`, `write_file`, `run_command`
   through GPT-4o and completes a task — verifies tool call/result mapping
4. **Error handling**: 429 rate limit from OpenAI → `RateLimitError` → exponential-backoff
   retry; 401 → `AuthError` → immediate failure

## Architecture

```
pkg/llm/providers/
├── anthropic.go    (Sprint 001 — unchanged)
└── openai.go       (NEW — Sprint 002)

Message format translation (the key complexity):

  Unified → OpenAI
  ─────────────────────────────────────────────────────────────
  llm.Message{Role:"system", ...}
    → openai.ChatCompletionMessage{Role:"system", Content: text}

  llm.Message{Role:"user", Content:[{type:"text", text:"..."}]}
    → openai.ChatCompletionMessage{Role:"user", Content:"..."}

  llm.Message{Role:"assistant", Content:[{type:"tool_use",...}]}
    → openai.ChatCompletionMessage{
        Role:"assistant",
        ToolCalls:[{ID, Type:"function", Function:{Name, Arguments}}],
      }

  llm.Message{Role:"user", Content:[{type:"tool_result",...}]}
    → one openai.ChatCompletionMessage per tool_result block:
        {Role:"tool", ToolCallID:"...", Content:"..."}

  OpenAI → Unified
  ─────────────────────────────────────────────────────────────
  resp.Choices[0].Message.Content (non-empty)
    → llm.ContentBlock{Type:"text", Text:...}

  resp.Choices[0].Message.ToolCalls[i]
    → llm.ContentBlock{Type:"tool_use", ToolUse:&llm.ToolUse{
        ID: tc.ID, Name: tc.Function.Name,
        Input: []byte(tc.Function.Arguments),
      }}

  resp.Usage → llm.Usage{InputTokens, OutputTokens}

  resp.Choices[0].FinishReason
    "stop"       → StopReasonEndTurn
    "tool_calls" → StopReasonToolUse
    "length"     → StopReasonMaxTokens
```

The conversion is encapsulated in `openaiClient` — the rest of the codebase is untouched.

## Implementation Plan

### Phase 1: Dependency + Scaffolding (~10%)

**Files:**
- `go.mod` — add `github.com/sashabaranov/go-openai`
- `pkg/llm/providers/openai.go` — skeleton with `init()` registration

**Tasks:**
- [ ] `go get github.com/sashabaranov/go-openai`
- [ ] Create `pkg/llm/providers/openai.go` with `init()` calling `llm.RegisterProvider("openai", ...)`
- [ ] Stub `Complete()` and `Stream()` returning `ErrNotImplemented` — confirms wiring

---

### Phase 2: Message Conversion (~35%)

**Files:**
- `pkg/llm/providers/openai.go` — `buildMessages()`, `buildTools()`, `convertResponse()`

**Tasks:**
- [ ] `buildMessages(msgs []llm.Message, system string) []openai.ChatCompletionMessage`:
  - Prepend `{Role:"system", Content:system}` if non-empty
  - For each message:
    - `role: user` with text blocks → `{Role:"user", Content: concatenated_text}`
    - `role: user` with tool_result blocks → one `{Role:"tool", ToolCallID, Content}` per block
    - `role: user` with mixed blocks → text blocks first as user message, then tool messages
    - `role: assistant` with text blocks → `{Role:"assistant", Content: text}`
    - `role: assistant` with tool_use blocks → `{Role:"assistant", ToolCalls:[...]}`
    - `role: assistant` with mixed (text + tool_use) → `{Role:"assistant", Content:text, ToolCalls:[...]}`
- [ ] `buildTools(defs []llm.ToolDefinition) []openai.Tool`:
  - Map name, description, input schema (JSON Schema object)
- [ ] `convertResponse(*openai.ChatCompletionResponse) llm.GenerateResponse`:
  - Extract text content from `Choices[0].Message.Content`
  - Extract tool calls from `Choices[0].Message.ToolCalls`
  - Map `FinishReason` → `StopReason`
  - Map `Usage` fields

---

### Phase 3: Complete() + Error Mapping (~20%)

**Files:**
- `pkg/llm/providers/openai.go` — `doComplete()`, `mapError()`

**Tasks:**
- [ ] `doComplete(ctx, req) (llm.GenerateResponse, error)`:
  - Build params using `buildMessages` and `buildTools`
  - Call `client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{...})`
  - Call `convertResponse` on success
  - Call `mapError` on failure
- [ ] `mapError(err error) error`:
  - Check for `*openai.APIError` — map StatusCode:
    - `429` → `&llm.RateLimitError{}`
    - `401, 403` → `&llm.AuthError{}`
    - `400` → check message for "context length" → `&llm.ContextLengthError{}`, else `&llm.LLMError{}`
    - `500, 502, 503` → `&llm.ServerError{}`
  - Wrap in `Complete()` with `llm.WithRetry` for retryable errors (same as Anthropic)

---

### Phase 4: Stream() (~10%)

**Files:**
- `pkg/llm/providers/openai.go` — `Stream()`

**Tasks:**
- [ ] Implement `Stream()` using `client.CreateChatCompletionStream(ctx, req)`
- [ ] Emit `StreamEvent{Type: StreamEventDelta, Text: chunk}` for each content delta
- [ ] Emit `StreamEvent{Type: StreamEventComplete, Response: &resp}` at end
- [ ] Close channel on completion or error
- [ ] Handle stream errors and emit nothing further after error (consistent with Anthropic adapter)

---

### Phase 5: Tests (~20%)

**Files:**
- `pkg/llm/providers/openai_test.go`

**Tasks:**
- [ ] Table-driven test for `buildMessages`:
  - User text message
  - User with tool_result blocks (→ "tool" role messages)
  - Assistant with tool_use blocks (→ ToolCalls)
  - Assistant with mixed text + tool_use
  - System prompt injection
- [ ] Test for `convertResponse`:
  - Text-only response
  - Tool-call response (multiple calls)
  - Empty content + FinishReason mapping
- [ ] Test for `mapError`:
  - 429 → RateLimitError, Retryable() = true
  - 401 → AuthError, Retryable() = false
  - 500 → ServerError, Retryable() = true
- [ ] Integration test (`TestOpenAIIntegration`, skipped without `OPENAI_API_KEY`):
  - Simple completion: "Say hello"
  - Tool-use: agent calls `read_file` once and returns result

---

### Phase 6: Secondary Deliverables (~5%)

**Files:**
- `CLAUDE.md` — project conventions
- `docs/specs/` — NLSpec reference files
- `examples/parallel.dot` — fan-out example
- `examples/multi_provider.dot` — Anthropic + OpenAI in one pipeline

**Tasks:**
- [ ] Write `CLAUDE.md` with: build/test/lint commands, adding providers, sprint process
- [ ] Write `docs/specs/attractor-spec.md`, `unified-llm-spec.md`, `coding-agent-loop-spec.md`
  (summarized versions of the spec; link to canonical `github.com/strongdm/attractor`)
- [ ] `examples/parallel.dot`: `start → fan_out → [analyze, lint] → fan_in → exit`
- [ ] `examples/multi_provider.dot`: two codergen nodes on different providers
- [ ] `bin/attractor lint examples/parallel.dot && bin/attractor lint examples/multi_provider.dot`

---

## Files Summary

| File | Action | Purpose |
|------|--------|---------|
| `go.mod` / `go.sum` | Modify | Add `sashabaranov/go-openai` dependency |
| `pkg/llm/providers/openai.go` | **Create** | Full OpenAI adapter |
| `pkg/llm/providers/openai_test.go` | **Create** | Unit + integration tests |
| `CLAUDE.md` | **Create** | Project conventions reference |
| `docs/specs/*.md` | **Create** | NLSpec offline reference files |
| `examples/parallel.dot` | **Create** | Fan-out/fan-in example |
| `examples/multi_provider.dot` | **Create** | Multi-provider example |

## Definition of Done

### Functional
- [ ] `attractor run pipeline.dot` works with `model="openai:gpt-4o"` on a codergen node
- [ ] Coding agent loop executes tool calls via GPT-4o and processes results correctly
- [ ] Multi-provider pipeline routes different nodes to Anthropic and OpenAI respectively
- [ ] `bin/attractor lint examples/parallel.dot` passes
- [ ] `bin/attractor lint examples/multi_provider.dot` passes

### Correctness
- [ ] All `buildMessages` test cases pass (especially tool_result → role:tool mapping)
- [ ] All `convertResponse` test cases pass
- [ ] All `mapError` test cases pass (correct error types, retryability)
- [ ] Multiple tool calls in one assistant turn handled correctly (not just first)

### Quality
- [ ] `go test -race ./...` passes (all packages)
- [ ] `golangci-lint run ./...` passes (0 issues)
- [ ] `OPENAI_API_KEY` integration test skipped without the env var
- [ ] No hardcoded API keys anywhere

## Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| `sashabaranov/go-openai` API surface differs from OpenAI HTTP spec | Low | Medium | Check SDK source and HTTP API docs before mapping |
| Mixed user messages (text + tool_result) — ordering ambiguity | Medium | High | Emit tool messages AFTER any text user message; document in code |
| OpenAI `parallel_tool_calls` — multiple calls in one turn | Medium | Medium | Loop over all `ToolCalls`, not just `[0]` — the unified loop already handles it |
| Rate-limit backoff interaction with context cancellation | Low | Medium | `WithRetry` already respects `ctx.Done()` — no change needed |
| Stream implementation complexity | Low | Low | Same pattern as Anthropic — use `CreateChatCompletionStream` |

## Security Considerations

- `OPENAI_API_KEY` from environment only; never logged, never stored in checkpoints
- All existing sandbox protections (path traversal, command timeout) are in the agent
  layer — unaffected by this sprint

## Dependencies

- Sprint 001 (core engine + Anthropic provider) — complete
- External: `github.com/sashabaranov/go-openai` — stable, widely used, no CGO

## Open Questions

1. Does `sashabaranov/go-openai` handle streaming via a `CreateChatCompletionStream`
   method? → Yes — returns a `*ChatCompletionStream` with `Recv()` method.
2. How does the SDK expose `APIError` for error classification? → `*openai.APIError`
   with `HTTPStatusCode` field.
3. For `parallel.dot`, should fan-out nodes be `codergen` or `set` (to avoid real API
   calls in CI)? → Use `set` nodes so `attractor lint` and `attractor run` (dry-mode)
   work without API keys.
