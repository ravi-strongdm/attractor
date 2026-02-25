# Sprint 001 Intent: Build Your Own Attractor

## Seed

> "I want to build my own attractor"

Inspired by StrongDM's open-spec Attractor project: a non-interactive coding agent
sufficient for use in a Software Factory.

## Context

This is a brand new project with no prior codebase. The reference is
`github.com/strongdm/attractor` — a repository containing three NLSpec (natural language
specification) markdown files that define a complete agentic software factory system.

StrongDM's principle: **seed → validation harness → feedback loop**.

Attractor is the execution engine: a DOT-based pipeline runner where nodes are
development phases (implement, optimize, validate), edges are natural-language conditions
evaluated by an LLM, and the engine traverses the graph until convergence.

## The Three Specs

1. **`attractor-spec.md`** — Core graph/pipeline runner
   - DOT (Graphviz) syntax for defining directed pipelines
   - Node handlers: codergen (LLM), wait.human, parallel fan-out/fan-in, tools, etc.
   - State management: thread-safe key-value context, checkpoints, artifacts
   - Condition expressions: minimal boolean language for edge routing
   - Model Stylesheet: CSS-like LLM config centralization
   - Observable, resumable, composable

2. **`unified-llm-spec.md`** — Multi-provider LLM client
   - Single interface over OpenAI, Anthropic, Gemini, etc.
   - blocking `complete()` and streaming `stream()` modes
   - Tool calling, multimodal content, thinking blocks
   - Error taxonomy with retryability + exponential backoff

3. **`coding-agent-loop-spec.md`** — Autonomous coding agent
   - LLM + developer tools (read/edit/run)
   - Session management, provider-aligned toolsets
   - Pluggable execution environments (local, Docker, SSH)
   - Loop detection, output truncation, steering injection

## Recent Sprint Context

No prior sprints — this is Sprint 001.

## Relevant Codebase Areas

None yet — greenfield project.

Community reference implementations for orientation:
- TypeScript: `github.com/brynary/attractor` (most mature)
- Python, Go, Rust, Ruby, Scala, F#, C implementations also exist

## Constraints

- Must conform to the NLSpecs in `github.com/strongdm/attractor`
- Language TBD (Python and TypeScript are the most natural given ecosystem)
- Sprint 1 scope TBD — could target one spec, two, or all three
- Must be runnable and testable by end of sprint

## Success Criteria

- A working pipeline executor that can load a `.dot` file, traverse nodes, call an LLM,
  and produce observable output
- At minimum: start node → codergen node (LLM call) → exit node works end-to-end
- Ideally: the system can run a simple agentic coding task from a seed prompt

## Verification Strategy

- Reference implementation: `github.com/brynary/attractor` (TypeScript) or spec text
- Spec/documentation: The three NLSpec `.md` files in `strongdm/attractor`
- Edge cases: DOT parsing edge cases, LLM provider switching, parallel fan-out
- Testing approach: Unit tests per spec section + integration test with a real pipeline

## Uncertainty Assessment

- Correctness uncertainty: **High** — three large, detailed specs to conform to
- Scope uncertainty: **High** — language, which spec(s) first, MVP vs full
- Architecture uncertainty: **Medium** — specs define layering well, implementation is flexible

## Open Questions

1. What language? (Python / TypeScript / Go / other)
2. Which spec first? All three or start with one layer?
3. MVP "hello world" pipeline or fuller implementation?
4. Should Sprint 1 include the unified LLM client, or use an existing SDK (e.g. `anthropic`)?
5. Verification: conform strictly to spec, or pragmatic MVP that deviates where convenient?
