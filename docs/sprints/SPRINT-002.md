# Sprint 002: OpenAI Provider Adapter

## Overview

Sprint 001 delivered a complete Attractor pipeline engine backed exclusively by the
Anthropic provider. The `pkg/llm` layer was designed for multiple providers from day one
— `llm.RegisterProvider(name, factory)` called from a package `init()` is the entire
extension point. Sprint 002 makes good on the "unified" promise by adding a full OpenAI
adapter, enabling any pipeline node to use GPT-4o or any other OpenAI model via
`model="openai:gpt-4o"`.

The primary technical work is message-format translation. OpenAI's Chat Completions API
differs from Anthropic's in two important ways: tool results use a dedicated `role: "tool"`
message type (rather than content blocks inside a user message), and tool calls from the
assistant appear in a `ToolCalls` field rather than as `tool_use` content blocks. The
adapter encapsulates these differences completely; the engine, agent loop, and CLI are
untouched.

Secondary deliverables bring the project to a more navigable state: `CLAUDE.md`
documenting developer conventions, a `docs/specs/` reference pointing to the canonical
NLSpec files, and two new example pipelines (`parallel.dot` and `multi_provider.dot`).

## Use Cases

1. **Single-model via OpenAI**: `start → codergen[model=openai:gpt-4o] → exit` — full
   coding agent loop through GPT-4o, including tool execution
2. **Multi-provider pipeline**: one node on `anthropic:claude-sonnet-4-6`, another on
   `openai:gpt-4o` in a single pipeline — per-node model routing
3. **Tool call round-trip**: agent calls `read_file`, `write_file`, `run_command` through
   GPT-4o; results returned as `role: "tool"` messages; loop continues correctly
4. **Parallel fan-out**: `start → fan_out → [analyze, lint] → fan_in → exit` using `set`
   nodes — demonstrates engine parallelism without real API calls
5. **Rate-limit resilience**: OpenAI 429 → `RateLimitError` → exponential-backoff retry

## Architecture

```
pkg/llm/providers/
├── anthropic.go    (Sprint 001 — unchanged)
└── openai.go       (Sprint 002 — NEW)
    ├── init()                     registers "openai" provider
    ├── newOpenAIClient(model)
    ├── Complete(ctx, req)         → doComplete with WithRetry
    ├── Stream(ctx, req)           → channel of StreamEvents
    ├── buildMessages(msgs, sys)   ← key translation logic
    ├── buildTools(defs)
    ├── convertResponse(resp)
    └── mapError(err)

Message format translation:

  Unified → OpenAI
  ─────────────────────────────────────────────────────────────
  System prompt (req.System)
    → prepend {role:"system", content: req.System}

  llm.Message{role:"user", content:[{type:"text"}]}
    → {role:"user", content: text}

  llm.Message{role:"user", content:[{type:"tool_result",...}, ...]}
    → one {role:"tool", tool_call_id, content} per block
      (invariant: our loop never mixes text + tool_results in one user message)

  llm.Message{role:"assistant", content:[{type:"text"}]}
    → {role:"assistant", content: text}

  llm.Message{role:"assistant", content:[{type:"tool_use",...}, ...]}
    → {role:"assistant", tool_calls:[{id, type:"function", function:{name, arguments}}]}

  llm.Message{role:"assistant", content:[text, tool_use, ...]}  (mixed)
    → {role:"assistant", content: text, tool_calls:[...]}

  OpenAI → Unified
  ─────────────────────────────────────────────────────────────
  choices[0].message.content  → ContentBlock{type:"text"}
  choices[0].message.tool_calls[i]
    → ContentBlock{type:"tool_use", tool_use:&ToolUse{id, name, input:arguments}}
  finish_reason "stop"        → StopReasonEndTurn
  finish_reason "tool_calls"  → StopReasonToolUse
  finish_reason "length"      → StopReasonMaxTokens
  usage.prompt_tokens         → Usage.InputTokens
  usage.completion_tokens     → Usage.OutputTokens
```

## Implementation Plan

### Phase 1: Dependency + Skeleton (~10%)

**Files:**
- `go.mod` / `go.sum` — add `github.com/sashabaranov/go-openai`
- `pkg/llm/providers/openai.go` — scaffolding with `init()` + stub methods

**Tasks:**
- [ ] `go get github.com/sashabaranov/go-openai`
- [ ] Create `openai.go` with `init()` registering "openai" factory
- [ ] Stub `Complete()` and `Stream()` returning `fmt.Errorf("not implemented")`
- [ ] Verify `go build ./...` still passes

---

### Phase 2: Message Conversion (~30%)

**Files:**
- `pkg/llm/providers/openai.go` — `buildMessages`, `buildTools`, `convertResponse`

**Tasks:**
- [ ] `buildMessages(msgs []llm.Message, system string) []openai.ChatCompletionMessage`:
  - If `system != ""`, prepend `{Role:"system", Content:system}`
  - For each `llm.Message`:
    - `role:"user"` with all-text content → `{Role:"user", Content: text}`
    - `role:"user"` with all-tool_result content → N × `{Role:"tool", ToolCallID, Content}`
    - `role:"assistant"` with all-text content → `{Role:"assistant", Content: text}`
    - `role:"assistant"` with tool_use blocks → `{Role:"assistant", ToolCalls:[...]}`
    - `role:"assistant"` with mixed text+tool_use → `{Role:"assistant", Content:text, ToolCalls:[...]}`
- [ ] `buildTools(defs []llm.ToolDefinition) []openai.Tool`:
  - Map each to `openai.Tool{Type:"function", Function:&openai.FunctionDefinition{Name, Description, Parameters}}`
  - `Parameters` is the raw JSON Schema bytes from `def.InputSchema`
- [ ] `convertResponse(resp openai.ChatCompletionResponse) llm.GenerateResponse`:
  - Extract text from `resp.Choices[0].Message.Content`
  - Extract ALL tool calls from `resp.Choices[0].Message.ToolCalls` (not just first)
  - Map `FinishReason` string → `llm.StopReason`
  - Map `resp.Usage` → `llm.Usage`

---

### Phase 3: Complete() + Error Mapping (~20%)

**Files:**
- `pkg/llm/providers/openai.go` — `doComplete`, `mapError`, `Complete`

**Tasks:**
- [ ] `doComplete(ctx context.Context, req llm.GenerateRequest) (llm.GenerateResponse, error)`:
  - Build `openai.ChatCompletionRequest{Model, MaxTokens, Messages, Tools}`
  - Call `client.CreateChatCompletion(ctx, params)`
  - Return `convertResponse(resp)` on success or `mapError(err)` on failure
- [ ] `Complete()` wraps `doComplete` with `llm.WithRetry(ctx, 4, fn)`
- [ ] `mapError(err error) error`:
  - Check `*openai.APIError` with `HTTPStatusCode`:
    - `429` → `&llm.RateLimitError{LLMError: base}`
    - `401, 403` → `&llm.AuthError{LLMError: base}`
    - `400` → `&llm.ContextLengthError{LLMError: base}` (conservative; improve later)
    - `500, 502, 503` → `&llm.ServerError{LLMError: base}`
    - other → `&llm.LLMError{Code: status}`
  - Non-APIError → wrap with `fmt.Errorf("openai: %w", err)`

---

### Phase 4: Stream() (~10%)

**Files:**
- `pkg/llm/providers/openai.go` — `Stream`

**Tasks:**
- [ ] Implement `Stream(ctx, req)` using `client.CreateChatCompletionStream(ctx, params)`:
  - Emit `StreamEvent{Type: StreamEventDelta, Text: delta.Content}` for each text delta
  - After stream completes, call `Complete(ctx, req)` to get the full response for the
    `StreamEventComplete` payload (simpler than accumulating tool calls across chunks)
  - Close channel when done; emit no events after error

> **Note**: Full streaming of tool call deltas (accumulating `Arguments` fragments across
> chunks) is deferred to a future sprint. The current implementation uses a hybrid
> approach: stream text deltas for interactive feel, fall back to blocking `Complete()`
> for the final response with tool calls.

---

### Phase 5: Tests (~20%)

**Files:**
- `pkg/llm/providers/openai_test.go`

**Tasks:**
- [ ] `TestBuildMessages`:
  - `user` text message → correct role and content
  - `user` tool_result message → role:"tool" messages with correct ToolCallID
  - `assistant` tool_use message → ToolCalls populated, content empty
  - `assistant` mixed text+tool_use → both content and ToolCalls set
  - System prompt → first message is role:"system"
- [ ] `TestConvertResponse`:
  - Text-only response → one text ContentBlock
  - Two tool calls → two tool_use ContentBlocks
  - `finish_reason: "tool_calls"` → `StopReasonToolUse`
  - `finish_reason: "length"` → `StopReasonMaxTokens`
  - Usage fields mapped correctly
- [ ] `TestMapError`:
  - 429 → `*llm.RateLimitError`, `Retryable() == true`
  - 401 → `*llm.AuthError`, `Retryable() == false`
  - 500 → `*llm.ServerError`, `Retryable() == true`
- [ ] `TestOpenAIIntegration` (skipped without `OPENAI_API_KEY`):
  - `t.Skipf("set OPENAI_API_KEY to run")`
  - Call `Complete()` with a simple prompt, verify non-empty text response
  - Call `Complete()` with a tool definition, verify tool_use block returned

---

### Phase 6: Secondary Deliverables (~10%)

**Files:**
- `CLAUDE.md`
- `docs/specs/attractor-spec.md`, `docs/specs/unified-llm-spec.md`, `docs/specs/coding-agent-loop-spec.md`
- `examples/parallel.dot`
- `examples/multi_provider.dot`

**Tasks:**
- [ ] Write `CLAUDE.md`:
  - Build: `export PATH="/opt/homebrew/bin:$PATH" && go build ./...`
  - Test: `go test -race ./...`
  - Lint: `golangci-lint run ./...`
  - Adding a new LLM provider: copy `pkg/llm/providers/anthropic.go`, call `llm.RegisterProvider("name", ...)` in `init()`
  - Blank-import the provider in `cmd/attractor/main.go`
  - Sprint docs in `docs/sprints/`; specs at `github.com/strongdm/attractor`
- [ ] Write stub `docs/specs/*.md` files that describe the spec briefly and link to the canonical source
- [ ] `examples/parallel.dot`: `start → fan_out → [analyze, summarize] → fan_in → exit`
  using `set` nodes (no real API calls needed)
- [ ] `examples/multi_provider.dot`: two codergen nodes with different models; one
  `model="anthropic:claude-sonnet-4-6"`, one `model="openai:gpt-4o"`
- [ ] `bin/attractor lint` both new examples

---

## Files Summary

| File | Action | Purpose |
|------|--------|---------|
| `go.mod` / `go.sum` | Modify | Add `sashabaranov/go-openai` |
| `pkg/llm/providers/openai.go` | **Create** | Full OpenAI adapter |
| `pkg/llm/providers/openai_test.go` | **Create** | Unit + integration tests |
| `CLAUDE.md` | **Create** | Developer conventions |
| `docs/specs/attractor-spec.md` | **Create** | Spec reference stub |
| `docs/specs/unified-llm-spec.md` | **Create** | Spec reference stub |
| `docs/specs/coding-agent-loop-spec.md` | **Create** | Spec reference stub |
| `examples/parallel.dot` | **Create** | Fan-out/fan-in example |
| `examples/multi_provider.dot` | **Create** | Multi-provider example |

## Definition of Done

### Functional
- [ ] `attractor run` works with `model="openai:gpt-4o"` on a codergen node (requires `OPENAI_API_KEY`)
- [ ] Coding agent loop executes multiple tool calls via GPT-4o and returns a final result
- [ ] `bin/attractor lint examples/parallel.dot` passes
- [ ] `bin/attractor lint examples/multi_provider.dot` passes

### Correctness
- [ ] `TestBuildMessages` all cases pass (especially tool_result → role:tool)
- [ ] `TestConvertResponse` all cases pass (multiple tool calls, finish reason mapping)
- [ ] `TestMapError` all cases pass (correct types + retryability)
- [ ] All tool calls in a multi-call turn are mapped (not just `[0]`)

### Quality
- [ ] `go test -race ./...` passes — all packages
- [ ] `golangci-lint run ./...` — 0 issues
- [ ] Integration test skips cleanly without `OPENAI_API_KEY`
- [ ] No hardcoded API keys

## Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| `sashabaranov/go-openai` API differs from what the adapter assumes | Low | Medium | Read SDK source before writing adapter; check `ChatCompletionMessage` fields |
| Mixed text+tool_use assistant messages edge-cased incorrectly | Medium | High | Explicit test case; document invariant that loop never mixes text+tool_result in user message |
| Streaming tool call accumulation complexity | Medium | Low | Use hybrid approach (stream text, fallback Complete for final response) |
| `openai.APIError` struct field names differ from assumed | Low | Low | Check SDK source; `HTTPStatusCode` is confirmed field name |

## Security Considerations

- `OPENAI_API_KEY` from environment only; never logged or checkpointed
- All existing sandbox protections (path traversal, run_command timeout) are at the agent
  layer — unaffected by this sprint

## Dependencies

- Sprint 001 complete (engine + Anthropic provider)
- `github.com/sashabaranov/go-openai` — no CGO, stable

## Open Questions

1. Should the parallel.dot fan-out branches use `codergen` (requiring API keys to run)
   or `set` (no API keys)? → **`set` nodes** — makes the example runnable in CI and as
   a demo without credentials.
2. For the multi_provider example, should it be a real runnable pipeline or just a
   lintable DOT file? → **Lintable only** for Sprint 002; real run is a stretch goal.
