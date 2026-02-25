# Sprint 015: LLM Utilities — `prompt` node + `json_decode` node

## Overview

Two nodes that improve LLM-based workflows:

1. **`prompt` node** — a lightweight single-turn LLM call with no tool loop.
   Renders a Go template, calls `client.Complete()` once, and stores the text
   response in a context key.  Much cheaper and simpler than `codergen` for
   tasks like classification, summarisation, or text transformation.

2. **`json_decode` node** — unpacks a JSON object stored in a context key into
   individual context keys (with an optional prefix).  Pairs naturally with
   `prompt` when the LLM response is structured JSON.

## Use Cases

1. **Sentiment classification**:
   ```dot
   classify [type=prompt
             prompt="Classify as positive/negative/neutral: {{.text}}"
             key="sentiment"
             system="Reply with a single word."
             model="anthropic:claude-haiku-4-5-20251001"
             max_tokens="5"]
   ```

2. **JSON extraction from LLM**:
   ```dot
   extract [type=prompt
            prompt="Extract author and title from: {{.raw}}\nRespond as JSON."
            key="meta_json"]
   unpack  [type=json_decode source="meta_json" prefix="meta_"]
   ```
   After `unpack`, `meta_author` and `meta_title` are set in context.

3. **Summarisation step** inside a larger pipeline:
   ```dot
   summarise [type=prompt
              prompt="Summarise in one sentence: {{.article}}"
              key="summary"]
   ```

4. **Config unpacking** — unpack a JSON config blob loaded via `read_file` or
   `env` into individual context keys:
   ```dot
   decode_cfg [type=json_decode source="config_json"]
   ```

## Architecture

### `prompt` node

```dot
n [type=prompt
   prompt="<Go template>"
   key="output_key"
   system="<optional system prompt>"
   model="<optional model override>"
   max_tokens="<optional int, default 1024>"]
```

| Attribute    | Required | Default        | Description |
|--------------|----------|----------------|-------------|
| `prompt`     | yes      | —              | Go template for the user message |
| `key`        | yes      | —              | Context key where the response text is stored |
| `system`     | no       | `""`           | System prompt passed to the model |
| `model`      | no       | `DefaultModel` | LLM model override (`provider:model-id`) |
| `max_tokens` | no       | `"1024"`       | Maximum tokens in the response |

**Execution**:

1. Render `prompt` template against `pctx.Snapshot()`.
2. Build `llm.GenerateRequest` with a single user message, optional system, and
   `MaxTokens` (default 1024 if attr absent or ≤ 0).
3. Call `client.Complete(ctx, req)` — one HTTP round-trip, no tools.
4. Walk `resp.Content` for the first `ContentTypeText` block; use its `Text`
   field as the output string.
5. `pctx.Set(key, output)` and `pctx.Set("last_output", output)`.

**PromptHandler** mirrors `CodergenHandler`'s shape:

```go
type PromptHandler struct {
    DefaultModel string
}
```

No `Workdir` field — `prompt` never touches the filesystem.

Implementation: `pkg/pipeline/handlers/prompt.go`

### `json_decode` node

```dot
n [type=json_decode source="ctx_key" prefix="out_"]
```

| Attribute | Required | Default | Description |
|-----------|----------|---------|-------------|
| `source`  | yes      | —       | Context key containing a JSON object string |
| `prefix`  | no       | `""`    | String prepended to each extracted key name |

**Execution**:

1. `raw := pctx.GetString(source)` — empty string treated as `{}`.
2. `json.Unmarshal([]byte(raw), &map[string]any{})`.
3. Return error if top-level value is not a JSON object (array, scalar, etc.).
4. For each key `k` and value `v`: `pctx.Set(prefix+k, fmt.Sprintf("%v", v))`.
   — scalar values are stored as their string representation.
   — nested objects/arrays are re-marshalled to compact JSON string.

Implementation: `pkg/pipeline/handlers/json_decode.go`

## Files

| File | Action | Description |
|------|--------|-------------|
| `pkg/pipeline/ast.go` | **Modify** | Add `NodeTypePrompt`, `NodeTypeJSONDecode` |
| `pkg/pipeline/handlers/prompt.go` | **Create** | `PromptHandler` |
| `pkg/pipeline/handlers/prompt_test.go` | **Create** | Unit tests (non-LLM paths) |
| `pkg/pipeline/handlers/json_decode.go` | **Create** | `JSONDecodeHandler` |
| `pkg/pipeline/handlers/json_decode_test.go` | **Create** | Unit tests |
| `pkg/pipeline/validator.go` | **Modify** | Add required attrs for `prompt` and `json_decode` |
| `cmd/attractor/main.go` | **Modify** | Register `prompt` and `json_decode` |
| `examples/prompt_decode.dot` | **Create** | Example pipeline |

## Definition of Done

### Functional
- [ ] `prompt` renders template and stores `key` in context
- [ ] `prompt` also sets `last_output`
- [ ] `prompt` respects `system`, `model`, `max_tokens` attrs
- [ ] `prompt` with invalid model returns clear error
- [ ] `json_decode` unpacks JSON object fields into context
- [ ] `json_decode` with `prefix` prepends prefix to each key
- [ ] `json_decode` with empty source (`""`) is a no-op (treats as `{}`)
- [ ] `json_decode` with non-object JSON returns clear error
- [ ] `attractor lint` catches missing `prompt` attrs (`prompt`, `key`)
- [ ] `attractor lint` catches missing `json_decode` attr (`source`)
- [ ] `examples/prompt_decode.dot` passes `attractor lint`

### Correctness
- [ ] `TestPromptMissingPromptAttr` — error on missing `prompt` attr
- [ ] `TestPromptMissingKeyAttr` — error on missing `key` attr
- [ ] `TestPromptInvalidModel` — error on bad model string
- [ ] `TestPromptDefaultMaxTokens` — max_tokens defaults to 1024
- [ ] `TestJSONDecodeBasic` — fields extracted into context
- [ ] `TestJSONDecodePrefix` — keys prefixed correctly
- [ ] `TestJSONDecodeEmptySource` — empty string → no keys set, no error
- [ ] `TestJSONDecodeNestedObject` — nested object → re-marshalled JSON string
- [ ] `TestJSONDecodeNonObject` — error for JSON array/scalar at top level
- [ ] `TestJSONDecodeMissingSourceAttr` — error on missing `source` attr

### Quality
- [ ] `go test -race ./...` — all green
- [ ] `golangci-lint run ./...` — 0 issues

## Dependencies

- Sprint 014 complete
- No new external dependencies
