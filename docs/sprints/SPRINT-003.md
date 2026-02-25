# Sprint 003: True Parallel Fan-Out Engine

## Overview

Sprints 001 and 002 delivered a complete pipeline engine with Anthropic and OpenAI
providers. However, the engine's `Execute` loop is purely sequential: for `fan_out`
nodes it calls `selectNext`, which picks only the first outgoing edge. The second (and
any additional) branch is silently skipped.

Sprint 003 fixes this fundamental correctness gap by implementing true parallel branch
execution in the engine, backed by goroutines and proper context merging. It also adds
the `docs/sprints/ledger.py` sprint-management script required by the `/sprint` skill.

## Use Cases

1. **Parallel analysis**: `start → fan_out → [analyze, summarize] → fan_in → report → exit`
   — both `analyze` and `summarize` run concurrently; their outputs merge before `report`
2. **Multi-branch pipelines**: Any fan-out with N branches all complete before fan-in
3. **Sprint management**: `python3 docs/sprints/ledger.py stats` shows sprint statuses

## Architecture

### Engine parallel execution

The sequential `Execute` loop detects `fan_out` nodes and takes a different code path:

1. Collect all outgoing edge targets from the `fan_out` node
2. Find the downstream `fan_in` node using BFS on the pipeline graph
3. For each branch start, copy the current `PipelineContext` and run a sub-execution
   goroutine that stops when it reaches the `fan_in` node (exclusive)
4. Wait for all goroutines with `handlers.RunParallel`
5. Merge all branch context snapshots into the main context (last-write-wins per key)
6. Continue the main loop from the `fan_in` node

```
Engine.Execute(...)
  │
  ├─ ... sequential nodes ...
  │
  ├─ [fan_out node detected]
  │     findFanIn(fanOutID) → fanInID
  │     branchIDs := outgoing edges of fanOutID
  │     RunParallel(branchIDs, func(branchID) {
  │         pctxCopy := pctx.Copy()
  │         executeUntil(branchID, "fan_in", pctxCopy)
  │         return pctxCopy.Snapshot()
  │     })
  │     pctx.Merge(mergedSnapshot)
  │     currentID = fanInID
  │
  ├─ [fan_in node] — runs its handler normally
  │
  └─ ... sequential nodes ...
```

### PipelineContext.Copy()

A new method on `PipelineContext` that returns a deep copy (snapshot-initialised new
context). Branch sub-executions operate on their copy; the main context is only updated
after all branches complete and results are merged.

### findFanIn

A BFS from the `fan_out` node that searches all forward-reachable nodes and returns the
first node of type `fan_in`. Panics/errors if none is found (validation could catch
this, but BFS makes it explicit at runtime).

## Files

| File | Action | Description |
|------|--------|-------------|
| `pkg/pipeline/engine.go` | **Modify** | Add parallel fan-out branch in `Execute`; add `findFanIn`, `executeUntil` |
| `pkg/pipeline/state.go` | **Modify** | Add `PipelineContext.Copy()` |
| `pkg/pipeline/pipeline_test.go` | **Modify** | Add `TestEngine_ParallelFanOut` |
| `docs/sprints/ledger.py` | **Create** | Sprint ledger CLI tool |

## Definition of Done

### Functional
- [ ] `attractor run examples/parallel.dot` completes successfully — both `analyze` and
      `summarize` branches execute and their outputs appear in the final context
- [ ] `python3 docs/sprints/ledger.py stats` prints sprint status table
- [ ] `python3 docs/sprints/ledger.py start 003` marks sprint in-progress
- [ ] `python3 docs/sprints/ledger.py complete 003` marks sprint complete

### Correctness
- [ ] `TestEngine_ParallelFanOut` passes: verifies both branches ran and both keys exist
      in the merged context
- [ ] Main context is not mutated during branch execution (branches use copies)
- [ ] Context merge is last-write-wins, not partial (all branch keys present)

### Quality
- [ ] `go test -race ./...` passes — no data races in parallel fan-out
- [ ] `golangci-lint run ./...` — 0 issues
- [ ] Existing tests unchanged and green

## Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| `findFanIn` fails for complex topologies | Low | Medium | BFS handles all DAG shapes; cycle guard already in engine |
| Data race on shared `pctx` during parallel branches | Medium | High | Branches use `Copy()`; main context updated only after `RunParallel` returns |
| Nested fan-out (fan_out inside a branch) | Low | Low | Out of scope; document as unsupported for Sprint 003 |

## Security Considerations

No new security surface — parallel execution uses existing sandboxed tool handlers.

## Dependencies

- Sprint 001 and 002 complete
- No new external dependencies
