# Sprint 019: Sub-pipeline Composition — `include` node

## Overview

**`include` node** — execute another DOT pipeline file as an inline sub-pipeline,
sharing the caller's `PipelineContext`.  Enables composable, reusable pipeline
building blocks without copy-pasting node sequences.

## Use Cases

1. **Reusable setup step**:
   ```dot
   setup [type=include path="shared/load_config.dot"]
   work  [type=codergen prompt="Analyse {{.config}}"]
   ```

2. **Modular multi-stage pipeline**:
   ```dot
   ingest    [type=include path="steps/ingest.dot"]
   transform [type=include path="steps/transform.dot"]
   publish   [type=include path="steps/publish.dot"]
   ```

3. **Parameterised sub-pipeline via context**:
   ```dot
   // parent sets context before calling sub-pipeline
   set_env [type=set key="env" value="prod"]
   deploy  [type=include path="deploy.dot"]
   ```

## Architecture

```dot
n [type=include path="<path template>"]
```

| Attribute | Required | Default | Description |
|-----------|----------|---------|-------------|
| `path`    | yes      | —       | Path to DOT file; Go template rendered against pctx |

**Execution**:
1. Render `path` template against `pctx.Snapshot()`.
2. Read and parse the included DOT file.
3. Validate the included pipeline.
4. Apply its stylesheet.
5. Build a handler registry using the same `workdir` and `defaultModel` as the parent.
6. Create a new engine with the **shared** `pctx` (no copy — changes propagate back).
7. Execute from the sub-pipeline's start node; no checkpoint for the sub-pipeline.
8. Return any error from the sub-pipeline.

**Cycle prevention**: an `include` node that directly or transitively includes
itself will hit the pipeline validation step or run into the engine's existing
cycle detection and return an error.

**IncludeHandler** needs access to workdir and defaultModel:
```go
type IncludeHandler struct {
    Workdir      string
    DefaultModel string
}
```

Implementation: `pkg/pipeline/handlers/include.go`

## Files

| File | Action | Description |
|------|--------|-------------|
| `pkg/pipeline/ast.go` | **Modify** | Add `NodeTypeInclude` |
| `pkg/pipeline/handlers/include.go` | **Create** | `IncludeHandler` |
| `pkg/pipeline/handlers/include_test.go` | **Create** | Unit tests |
| `pkg/pipeline/validator.go` | **Modify** | Add required attrs |
| `cmd/attractor/main.go` | **Modify** | Register handler |
| `examples/include/` | **Create** | Example: main + sub pipeline |

## Definition of Done

### Functional
- [ ] `include` runs a sub-pipeline and shares caller context
- [ ] Context changes in sub-pipeline visible in parent after include
- [ ] `include` path is template-rendered against pctx
- [ ] `include` returns sub-pipeline errors to parent
- [ ] Missing include file returns clear error
- [ ] Invalid include DOT file returns parse/lint error
- [ ] `attractor lint` catches missing `path` attr

### Correctness
- [ ] `TestIncludeBasic` — sub-pipeline runs and sets context key
- [ ] `TestIncludeContextShared` — parent sees sub-pipeline's context changes
- [ ] `TestIncludePathTemplate` — path rendered against context
- [ ] `TestIncludeMissingFile` — error for missing DOT file
- [ ] `TestIncludeInvalidDOT` — error for parse/lint failure
- [ ] `TestIncludeMissingPathAttr` — validator catches missing path

### Quality
- [ ] `go test -race ./...` — all green
- [ ] `golangci-lint run ./...` — 0 issues

## Dependencies
- Sprint 018 complete
- No new external dependencies
