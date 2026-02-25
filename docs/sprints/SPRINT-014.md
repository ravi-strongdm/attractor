# Sprint 014: Developer Experience — `graph` subcommand + `--var-file`

## Overview

Two quality-of-life improvements for pipeline authors and operators:

1. **`attractor graph <pipeline.dot>`** — print a human-readable summary of
   a pipeline: nodes by type with their key attributes, and edges with
   conditions.  A `--format dot` option re-emits a canonically-formatted DOT
   file.  Makes it easy to document, review, and audit pipelines without
   reading raw DOT syntax.

2. **`--var-file <path.json>`** — load initial pipeline context variables
   from a JSON file on `run` and `resume`.  Individual `--var` flags override
   file values.  Enables complex configurations and shared context fixtures
   to be version-controlled rather than spelled out on the command line.

## Use Cases

1. **Pipeline review**: `attractor graph pipeline.dot` in CI to print a
   readable summary for PR comments.
2. **Canonical formatting**: `attractor graph pipeline.dot --format dot`
   re-emits a clean DOT file (useful for normalising hand-written pipelines).
3. **Shared config**: `attractor run pipeline.dot --var-file prod.json` loads
   a shared JSON config; `--var key=override` overrides individual keys.
4. **Test fixtures**: `attractor run pipeline.dot --var-file fixtures/test.json`
   drives pipeline tests with structured input data.

## Architecture

### `attractor graph` subcommand

```
attractor graph <pipeline.dot> [--format text|dot]
```

**Text format** (default):

```
Pipeline: batch  (7 nodes, 6 edges)

Nodes:
  start          start
  load_topics    env         from=TOPICS required=true key=topics_raw
  split_topics   split       source=topics_raw sep=\n trim=true key=topics
  check          assert      expr=topics
  analyse        map         items=topics item_key=topic concurrency=2
  save           write_file  path={{.output_dir}}/summaries.json
  done           exit

Edges:
  start        → load_topics
  load_topics  → split_topics
  split_topics → check
  check        → analyse
  analyse      → save
  save         → done
```

Rules for text format:
- Nodes are printed in topological order (BFS from start); unordered nodes
  appended at end.
- Attrs are printed as `key=value` pairs, longest value truncated at 60 chars
  with `…`.
- Edge conditions are printed in `[label]` notation when non-empty.
- Node ID column is padded to align types.

**DOT format** (`--format dot`):

Re-emits a canonical DOT digraph:
```dot
digraph <name> {
    <id> [type=<type> key1=val1 key2=val2]
    ...
    <from> -> <to>
    <from> -> <to> [label="<condition>"]
}
```
Values containing spaces or special characters are double-quoted.

Implementation: `cmd/attractor/graph.go` (new file, keeps `main.go` clean).

### `--var-file` flag

Added to `run` and `resume` subcommands as `--var-file string`.

```
attractor run pipeline.dot --var-file context.json --var model=override
```

Execution order:
1. Parse pipeline.
2. `applyVarFile(pctx, varFile)` — load JSON object; set each key in pctx.
3. `applyVars(pctx, vars)` — `--var` flags override file values.

`applyVarFile`:
- Read and `json.Unmarshal` into `map[string]any`.
- Reject if top-level value is not an object.
- `pctx.Set(k, fmt.Sprintf("%v", v))` for each key — all values are stored
  as strings for consistency with `--var`.

## Files

| File | Action | Description |
|------|--------|-------------|
| `cmd/attractor/graph.go` | **Create** | `graphCmd()` implementation |
| `cmd/attractor/main.go` | **Modify** | Register `graphCmd`; add `--var-file` to run/resume |
| `cmd/attractor/main_test.go` | **Create** | CLI-level tests for graph and var-file |

## Definition of Done

### Functional
- [ ] `attractor graph pipeline.dot` prints node table and edge list
- [ ] Node attributes truncated at 60 chars with `…`
- [ ] `attractor graph pipeline.dot --format dot` emits valid DOT
   (re-parseable by `attractor lint`)
- [ ] `attractor run pipeline.dot --var-file vars.json` loads context from JSON
- [ ] `--var` overrides `--var-file` values for the same key
- [ ] `--var-file` with a non-object JSON value returns a clear error
- [ ] `--var-file` with a missing file returns a clear error

### Correctness
- [ ] `TestGraphTextOutput` — output contains node IDs and types
- [ ] `TestGraphDOTRoundtrip` — DOT output re-parses without lint errors
- [ ] `TestVarFileBasic` — JSON vars loaded into context
- [ ] `TestVarFileOverriddenByVar` — `--var` wins over `--var-file`
- [ ] `TestVarFileMissing` — error on missing file
- [ ] `TestVarFileNonObject` — error for JSON array/scalar at top level

### Quality
- [ ] `go test -race ./...` — all green
- [ ] `golangci-lint run ./...` — 0 issues

## Dependencies

- Sprint 013 complete
- No new external dependencies
