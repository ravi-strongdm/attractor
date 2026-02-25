# Unified LLM Client Spec Reference

This file is a stub pointing to the canonical specification.

**Canonical source**: https://github.com/strongdm/attractor

## Summary

The unified LLM client (`pkg/llm`) defines a provider-agnostic interface for
interacting with large language models. Providers register via `llm.RegisterProvider`
called from their package's `init()` function.

### Interface

```go
type Client interface {
    Complete(ctx context.Context, req GenerateRequest) (GenerateResponse, error)
    Stream(ctx context.Context, req GenerateRequest) (<-chan StreamEvent, error)
}
```

### Message Format

Messages use a content-block model:
- `ContentTypeText` — plain text
- `ContentTypeToolUse` — model requesting a tool call
- `ContentTypeToolResult` — result of a tool call

Tool results are sent as `role:user` messages with `ContentTypeToolResult` blocks
(the adapter maps these to each provider's native format).

### Error Types

- `RateLimitError` — HTTP 429, retryable
- `ServerError` — HTTP 5xx, retryable
- `AuthError` — HTTP 401/403, not retryable
- `ContextLengthError` — HTTP 400 / context too long

### Provider Selection

Model IDs use the format `provider:model-name`, e.g. `anthropic:claude-sonnet-4-6`
or `openai:gpt-4o`. The engine splits on the first `:` to select the registered factory.
