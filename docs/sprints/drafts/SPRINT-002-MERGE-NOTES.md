# Sprint 002 Merge Notes

## Situation

Codex failed to produce a competing draft (no interactive TTY available in the execution
environment). Merge is based on Claude's draft alone, cross-checked against:

- The intent document (`SPRINT-002-INTENT.md`)
- Sprint 001 precedents (how Anthropic adapter was structured)
- Known OpenAI API surface (`sashabaranov/go-openai`)

## Claude Draft Strengths

- Correctly identifies the tool-result → `role: "tool"` mapping as the primary complexity
- Clear phased breakdown: scaffolding → message conversion → error handling → streaming → tests
- Good edge case enumeration: parallel tool calls, mixed content (text + tool_use), rate-limit retry
- Secondary deliverables (CLAUDE.md, docs/specs/, examples) are well-scoped as a distinct phase

## Gaps / Self-Critique

1. **`buildMessages` for mixed user messages** (text + tool_result in same `llm.Message`):
   The draft says "text blocks first as user message, then tool messages" but the loop in
   `loop.go` never produces mixed user messages — tool results are always their own user turn.
   Simplify: a user message has EITHER text OR tool_results, never both. Document this assumption.

2. **Streaming implementation** deserves more detail: `CreateChatCompletionStream` returns
   a stream object with `Recv()` calls. We need to accumulate tool calls across stream chunks
   (OpenAI sends tool calls piecemeal across deltas). The draft understates this complexity.

3. **`CLAUDE.md` content** — the draft mentions writing it but doesn't outline what goes in.
   Should include: build commands, provider registration pattern, sprint conventions.

4. **Docs/specs** — rather than writing summarized versions, we should note that the canonical
   specs live at `github.com/strongdm/attractor`. CLAUDE.md should link there.

## Interview Refinements Applied

- OpenAI only (no Gemini)
- `sashabaranov/go-openai` SDK chosen

## Final Decisions

1. **Streaming**: implement but note that streaming tool call accumulation is complex.
   Initial implementation can use `CreateChatCompletion` (non-streaming) for `Complete()`
   and a simple streaming wrapper for `Stream()` that collects then emits. Document the
   limitation and leave proper streaming as a future improvement.

2. **Mixed user messages**: document the invariant that our `loop.go` always sends tool
   results as a dedicated user message with no text content. Assert this in `buildMessages`
   for safety.

3. **`parallel.dot`**: use `set` nodes (not `codergen`) so the example works without API keys.

4. **`CLAUDE.md`**: write it as a proper developer guide covering: commands, adding providers,
   the `init()` registration pattern, and sprint planning links.
