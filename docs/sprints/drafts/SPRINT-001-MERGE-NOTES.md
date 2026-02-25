# Sprint 001 Merge Notes

## Codex Status
Codex skipped — requires interactive TTY. Proceeding with interview refinements only.

## Claude Draft Strengths
- Clear three-layer architecture diagram
- Good separation of the three specs into distinct packages
- Identified parallel fan-out and condition expressions as scope risks
- Security section (sandboxed run_command, path-locked write_file) is strong
- Checkpoint / resumable design captured

## Interview Refinements Applied

| Question | Answer | Impact |
|----------|--------|--------|
| Language | **Go** | Entire draft rewritten: Python packages → Go modules, pyproject.toml → go.mod, pytest → testing package, `click` → `cobra` or `flag` |
| Scope | **All three specs** | Keep all three layers (unified LLM client, agent loop, pipeline engine) |
| Conformance | **Strict from day 1** | Added explicit spec-section references in tasks; MUST-level items are non-negotiable DoD |

## Key Changes From Claude Draft → Final Sprint

1. **Language pivot Python → Go**: idiomatic Go module layout (`cmd/attractor/`, `internal/`, `pkg/`)
2. **Go concurrency**: parallel fan-out handlers use goroutines + `sync.WaitGroup`
3. **Go error handling**: explicit error returns instead of exceptions; error taxonomy as typed errors
4. **Go streaming**: `iter.Seq` or channels for streaming LLM responses
5. **Testing**: `testing` package + `testify` for assertions; table-driven tests
6. **CLI**: `cobra` for subcommands (`run`, `lint`, `resume`)
7. **Spec conformance gates added to DoD**: each spec's MUST-level requirements listed as checkboxes

## Final Decisions

- **Module path**: `github.com/[user]/attractor`
- **DOT parsing**: use `github.com/goccy/go-graphviz` (maintained, pure Go)
- **Anthropic SDK**: use official `github.com/anthropics/anthropic-sdk-go`
- **OpenAI SDK**: stub for Sprint 1
- **Condition evaluator**: implement the spec's minimal boolean language (not a full expression engine)
- **Parallel fan-out**: include in Sprint 1 (Go makes this natural with goroutines)
