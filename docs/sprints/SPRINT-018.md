# Sprint 018: Sequential Iteration — `for_each` node

## Overview

**`for_each` node** — iterate sequentially over a JSON array, running a shell
command template once per element and collecting outputs into a results array.

Unlike `map` (parallel LLM agent per item), `for_each` is:
- **Sequential** — items processed one at a time in order
- **Shell-based** — runs a `cmd` template (like `exec`) rather than an LLM
- **Lightweight** — no API calls, no concurrency overhead

This fills the gap for loops that need to process items without an LLM:
building file lists, running per-repo commands, accumulating results.

## Use Cases

1. **Run a command for each repo**:
   ```dot
   each [type=for_each
         items="repos"
         item_key="repo"
         cmd="git -C {{.repo}} log --oneline -1"
         results_key="last_commits"]
   ```

2. **Build file summaries without LLM**:
   ```dot
   each [type=for_each
         items="files"
         item_key="f"
         cmd="wc -l {{.f}}"
         results_key="line_counts"]
   ```

3. **Chain with split**:
   ```dot
   split [type=split source="raw_list" sep="," trim="true" key="items"]
   each  [type=for_each items="items" item_key="item"
          cmd="echo processed:{{.item}}" results_key="processed"]
   ```

## Architecture

```dot
n [type=for_each
   items="ctx_key"
   item_key="current"
   cmd="<shell template>"
   results_key="<optional>"
   fail_on_error="true"
   timeout="30s"
   workdir="."]
```

| Attribute      | Required | Default              | Description |
|----------------|----------|----------------------|-------------|
| `items`        | yes      | —                    | Context key holding a JSON array string |
| `item_key`     | yes      | —                    | Per-iteration context key for the current element |
| `cmd`          | yes      | —                    | Shell command template rendered per item |
| `results_key`  | no       | `<nodeID>_results`   | Context key for JSON array of stdout strings |
| `fail_on_error`| no       | `"true"`             | Fail on non-zero exit code |
| `timeout`      | no       | `""`                 | Per-item timeout (time.ParseDuration string) |
| `workdir`      | no       | handler `Workdir`    | Working directory (template rendered) |

**Execution**:
1. Parse JSON array from `pctx.GetString(items)`.  Empty/missing → store `"[]"` and return nil.
2. For each element `v` (index `i`):
   a. Copy `pctx` into a branch context.
   b. Set `item_key` = `fmt.Sprintf("%v", v)` in branch context.
   c. Render `cmd` template against branch context.
   d. Run `/bin/sh -c renderedCmd` with optional timeout.
   e. Capture stdout into `results[i]`.
   f. If exit code ≠ 0 and `fail_on_error` ≠ `"false"`: return error immediately.
3. `json.Marshal(results)` → store in `results_key`.
4. Set `last_output` = results JSON.

**ForEachHandler** struct mirrors ExecHandler:
```go
type ForEachHandler struct {
    Workdir string
}
```

Implementation: `pkg/pipeline/handlers/for_each.go`

## Files

| File | Action | Description |
|------|--------|-------------|
| `pkg/pipeline/ast.go` | **Modify** | Add `NodeTypeForEach` |
| `pkg/pipeline/handlers/for_each.go` | **Create** | `ForEachHandler` |
| `pkg/pipeline/handlers/for_each_test.go` | **Create** | Unit tests |
| `pkg/pipeline/validator.go` | **Modify** | Add required attrs |
| `cmd/attractor/main.go` | **Modify** | Register handler |
| `examples/for_each.dot` | **Create** | Example pipeline |

## Definition of Done

### Functional
- [ ] `for_each` with empty array stores `"[]"` without error
- [ ] `for_each` runs cmd once per item in order
- [ ] `for_each` sets item_key in branch context for template rendering
- [ ] `for_each` stores results as JSON array in results_key
- [ ] `for_each` sets `last_output` = results JSON
- [ ] `for_each` fails on non-zero exit by default
- [ ] `for_each` succeeds with fail_on_error=false
- [ ] `for_each` respects per-item timeout
- [ ] `attractor lint` catches missing attrs

### Correctness
- [ ] `TestForEachBasic` — cmd run for each item, results collected
- [ ] `TestForEachEmptyArray` — empty → `"[]"`, no error
- [ ] `TestForEachItemKey` — item_key set correctly per iteration
- [ ] `TestForEachFailOnError` — non-zero exit stops iteration
- [ ] `TestForEachNoFailOnError` — fail_on_error=false continues
- [ ] `TestForEachTimeout` — item killed on timeout
- [ ] `TestForEachMissingAttrs` — validator catches missing attrs

### Quality
- [ ] `go test -race ./...` — all green
- [ ] `golangci-lint run ./...` — 0 issues

## Dependencies
- Sprint 017 complete
- No new external dependencies
