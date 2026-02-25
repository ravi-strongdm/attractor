# Sprint 005: Google Gemini Provider

## Overview

Sprints 001 and 002 established Anthropic and OpenAI adapters. The unified-llm-spec
lists Google Gemini as the third required provider. Sprint 005 delivers it, completing
the "unified" promise: any pipeline node can select any of the three providers via
`model="gemini:gemini-2.0-flash"`.

The primary technical work is message-format translation. Gemini's API differs from
both Anthropic and OpenAI in key ways:

- **System prompt** is set via `model.SystemInstruction`, not as a message in the
  history — it is stripped from the messages slice before building the request.
- **Assistant role** is `"model"` in Gemini (not `"assistant"`).
- **Tool calls** appear as `genai.FunctionCall` parts in a `"model"` content block.
- **Tool results** appear as `genai.FunctionResponse` parts in a `"user"` content
  block. The response payload must be `map[string]any`, so plain-text tool results
  are wrapped as `{"result": "..."}`.
- **Tool definitions** use `*genai.Schema` instead of raw JSON Schema bytes; a
  recursive JSON Schema → `genai.Schema` converter is required.
- **Chat history** is passed via `model.StartChat()` / `cs.History`; the last user
  message is sent via `cs.SendMessage()`.

## Use Cases

1. **Single-model via Gemini**: `start → codergen[model=gemini:gemini-2.0-flash] → exit`
2. **Three-provider pipeline**: three codergen nodes, one per provider, in sequence
3. **Tool call round-trip**: agent calls `search_file` through Gemini; results returned
   as `FunctionResponse`; loop continues correctly

## Architecture

```
pkg/llm/providers/
├── anthropic.go    (Sprint 001 — unchanged)
├── openai.go       (Sprint 002 — unchanged)
└── gemini.go       (Sprint 005 — NEW)
    ├── init()                    registers "gemini" provider
    ├── newGeminiClient(model)    reads GEMINI_API_KEY
    ├── Complete(ctx, req)        → doComplete with WithRetry
    ├── Stream(ctx, req)          → channel of StreamEvents
    ├── buildContents(msgs)       ← key translation logic
    ├── buildGeminiTools(defs)    ← JSON Schema → genai.Schema
    ├── jsonSchemaToGenai(raw)    ← recursive schema converter
    ├── convertResponse(resp)
    └── mapGeminiError(err)
```

### Message format translation

```
Unified → Gemini
─────────────────────────────────────────────────────────────
System (req.System)
  → model.SystemInstruction = &genai.Content{Parts:[Text(sys)]}

llm.Message{role:"user", content:[{type:"text"}]}
  → &genai.Content{Role:"user", Parts:[genai.Text(text)]}

llm.Message{role:"user", content:[{type:"tool_result",...}]}
  → &genai.Content{Role:"user", Parts:[genai.FunctionResponse{
        Name: <from ToolUseID lookup>,
        Response: map[string]any{"result": content},
    }]}

llm.Message{role:"assistant", content:[{type:"text"}]}
  → &genai.Content{Role:"model", Parts:[genai.Text(text)]}

llm.Message{role:"assistant", content:[{type:"tool_use"}]}
  → &genai.Content{Role:"model", Parts:[genai.FunctionCall{
        Name: name,
        Args: map[string]any{...}, // unmarshal Input JSON
    }]}
```

### Tool result name resolution

Gemini's `FunctionResponse` requires the function name, but `ToolResult` carries a
`ToolUseID` (the call ID), not the name. `buildContents` performs a backward scan of
the history to find the matching `ToolUse` block and extract its `Name`.

## Files

| File | Action | Description |
|------|--------|-------------|
| `go.mod` / `go.sum` | **Modify** | Add `github.com/google/generative-ai-go` |
| `pkg/llm/providers/gemini.go` | **Create** | Gemini adapter |
| `pkg/llm/providers/gemini_test.go` | **Create** | Unit tests for message conversion |
| `examples/gemini.dot` | **Create** | Example pipeline using Gemini |

## Definition of Done

### Functional
- [ ] `attractor run examples/gemini.dot` (requires `GEMINI_API_KEY`) completes a
      simple codergen turn via Gemini
- [ ] `attractor lint examples/gemini.dot` passes without API key

### Correctness
- [ ] `TestBuildContents_UserText` — user text → `{Role:"user", Parts:[Text]}`
- [ ] `TestBuildContents_AssistantText` — assistant text → `{Role:"model", Parts:[Text]}`
- [ ] `TestBuildContents_ToolCall` — tool_use → FunctionCall part
- [ ] `TestBuildContents_ToolResult` — tool_result → FunctionResponse part (name resolved)
- [ ] `TestBuildContents_SystemStripped` — system role messages excluded from history
- [ ] `TestJsonSchemaToGenai` — type/properties/required/description mapped correctly
- [ ] `TestConvertGeminiResponse_Text` — text part → ContentTypeText block
- [ ] `TestConvertGeminiResponse_ToolCall` — FunctionCall part → ContentTypeToolUse block
- [ ] `TestMapGeminiError` — HTTP 429 → RateLimitError, 401 → AuthError

### Quality
- [ ] `go test -race ./...` — all green
- [ ] `golangci-lint run ./...` — 0 issues
- [ ] Integration test skips cleanly without `GEMINI_API_KEY`
- [ ] No hardcoded API keys

## Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| `genai.Schema` type mapping is incomplete | Medium | Low | Cover common cases (object/string/integer/boolean/array); leave unknown types as TypeUnspecified |
| `FunctionResponse.Name` resolution fails for multi-turn history | Low | Medium | Backward scan covers all previous turns; tool IDs are unique per session |
| Gemini API changes between SDK versions | Low | Medium | Pin a specific module version in go.mod |

## Dependencies

- Sprint 004 complete
- `github.com/google/generative-ai-go` — no CGO
- `google.golang.org/api` — transitive dep (option package)
