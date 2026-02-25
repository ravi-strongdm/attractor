# Sprint 013: Batch Processing — `split` node + `map` node

## Overview

Two nodes that unlock list-oriented workflows:

1. **`split` node** — converts a string (e.g., a newline-delimited file list)
   into a JSON array stored in the pipeline context.  Acts as the producer
   side of any batch pipeline.

2. **`map` node** — runs a codergen prompt in parallel for every element in a
   JSON array, collecting the outputs into a new JSON array.  The most powerful
   node type added so far: enables "process N things in parallel" without any
   fan_out/fan_in scaffolding.

## Use Cases

1. **Batch file analysis**:
   `read_file [key="file_list" path="files.txt"]`
   → `split [source="file_list" sep="\n" trim="true" key="files"]`
   → `map [items="files" item_key="path" prompt="Summarise {{.path}}" results_key="summaries"]`

2. **Multi-repo audit**:
   `env [key="repos" from="REPO_LIST"]`
   → `split [source="repos" sep="," key="repo_arr"]`
   → `map [items="repo_arr" item_key="repo" prompt="Audit security of {{.repo}}"]`

3. **Rate-limited parallel calls**: `map [concurrency="2" ...]` limits to 2
   simultaneous LLM calls.

## Architecture

### `split` node

```dot
tokenise [type=split source="raw" sep="\n" trim="true" key="lines"]
```

| Attribute | Required | Default | Description |
|-----------|----------|---------|-------------|
| `source`  | yes      | —       | Context key containing the input string |
| `key`     | yes      | —       | Context key to store the resulting JSON array |
| `sep`     | no       | `"\n"`  | Separator string |
| `trim`    | no       | `"false"` | Strip whitespace from each element; drop empty strings |

Implementation: `pkg/pipeline/handlers/split.go`

- Get string from `pctx.GetString(source)`.
- `strings.Split(raw, sep)`.
- If `trim == "true"`: `strings.TrimSpace` each element, filter out empties.
- `json.Marshal(parts)` → store in `key`.

### `map` node

```dot
analyse [type=map
         items="files"
         item_key="current_file"
         prompt="Analyse this path: {{.current_file}}"
         results_key="analyses"
         concurrency="3"
         model="anthropic:claude-sonnet-4-6"
         system_prompt="You are a code reviewer."
         max_turns="10"]
```

| Attribute      | Required | Default                 | Description |
|----------------|----------|-------------------------|-------------|
| `items`        | yes      | —                       | Context key holding a JSON array string |
| `item_key`     | yes      | —                       | Per-iteration context key for the current element |
| `prompt`       | yes      | —                       | Go template for the codergen prompt; rendered per item |
| `results_key`  | no       | `<nodeID>_results`      | Where to store the JSON array of outputs |
| `concurrency`  | no       | `"0"` (all parallel)    | Max simultaneous goroutines; 0 = unbounded |
| `model`        | no       | `DefaultModel`          | LLM model override |
| `system_prompt`| no       | `""`                    | System prompt for each sub-agent |
| `max_turns`    | no       | `"50"`                  | Max turns per item |

**Execution model**:

1. Parse the JSON array from `pctx.Get(items)`.
2. If the array is empty, store `"[]"` in `results_key` and return nil.
3. Spawn one goroutine per item, bounded by a semaphore of size `concurrency`
   (0 means `len(items)`).
4. Each goroutine:
   a. Copies `pctx` (independent context per item).
   b. Sets `item_key` = `fmt.Sprintf("%v", item)`.
   c. Renders `prompt` against the branch context.
   d. Creates an LLM client, builds a `CodingAgentLoop`, runs it.
   e. Stores `result.Output` in the results slice at the correct index.
5. Wait for all goroutines; collect errors.
6. On any error: return the first error (wrapped with item index).
7. Marshal `[]string{results...}` → store in `results_key`.
8. Also set `last_output` = JSON array (consistent with other nodes).

**MapHandler** mirrors the shape of `CodergenHandler`:

```go
type MapHandler struct {
    DefaultModel string
    Workdir      string
}
```

Implementation: `pkg/pipeline/handlers/map.go`

## Files

| File | Action | Description |
|------|--------|-------------|
| `pkg/pipeline/ast.go` | **Modify** | Add `NodeTypeSplit`, `NodeTypeMap` |
| `pkg/pipeline/handlers/split.go` | **Create** | `SplitHandler` |
| `pkg/pipeline/handlers/split_test.go` | **Create** | Unit tests |
| `pkg/pipeline/handlers/map.go` | **Create** | `MapHandler` |
| `pkg/pipeline/handlers/map_test.go` | **Create** | Unit tests (non-LLM paths) |
| `pkg/pipeline/validator.go` | **Modify** | Add required attrs for `split` and `map` |
| `cmd/attractor/main.go` | **Modify** | Register `split` and `map` |
| `examples/batch.dot` | **Create** | Example pipeline |

## Definition of Done

### Functional
- [ ] `split` splits by custom separator and stores JSON array
- [ ] `split` with `trim="true"` strips whitespace and drops empty elements
- [ ] `split` with default separator (`\n`) works correctly
- [ ] `map` with empty array stores `"[]"` without error
- [ ] `map` propagates item errors with index information
- [ ] `map` stores results as JSON array in `results_key`
- [ ] `map` respects `concurrency` limit
- [ ] `attractor lint` catches missing `split` attrs (`source`, `key`)
- [ ] `attractor lint` catches missing `map` attrs (`items`, `item_key`, `prompt`)
- [ ] `examples/batch.dot` passes `attractor lint`

### Correctness
- [ ] `TestSplitNewline` — split by `\n`, no trim
- [ ] `TestSplitCustomSep` — split by `,`
- [ ] `TestSplitTrim` — whitespace stripped, empties dropped
- [ ] `TestSplitEmpty` — empty source → `[""]` (no trim) or `[]` (trim)
- [ ] `TestSplitMissingAttrs` — error on missing `source` or `key`
- [ ] `TestMapEmptyArray` — empty items array → `"[]"` stored, nil error
- [ ] `TestMapInvalidJSON` — non-array JSON in items key → error
- [ ] `TestMapMissingItemsKey` — items key absent from context → error
- [ ] `TestMapMissingAttrs` — missing required attrs → error
- [ ] `TestMapConcurrencyDefault` — concurrency=0 runs all items

### Quality
- [ ] `go test -race ./...` — all green
- [ ] `golangci-lint run ./...` — 0 issues

## Dependencies

- Sprint 012 complete
- No new external dependencies
