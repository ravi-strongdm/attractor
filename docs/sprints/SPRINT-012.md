# Sprint 012: JSON Extraction & Smarter Linting

## Overview

Two improvements that make pipelines more capable and easier to validate:

1. **`json_extract` node** — extract a single field from a JSON string stored
   in the pipeline context using a simple dot-path expression.  Makes it
   trivial to pull structured values out of `http` responses without a
   `codergen` node.

2. **Validator attribute checking** — teach `attractor lint` to verify that
   every node carries its required attributes.  Currently the linter only
   checks graph structure; attribute errors are only discovered at runtime.
   After this sprint, `attractor lint` catches them immediately.

## Use Cases

1. **Parse API response**: `http` → `json_extract [source="body" path=".id"
   key="record_id"]` → `set [key="url" value="https://api/{{.record_id}}"]`.
2. **Nested extraction**: `json_extract [source="resp" path=".data.user.email"
   key="email"]`.
3. **Array indexing**: `json_extract [source="results" path=".items.0.name"
   key="first_name"]`.
4. **CI guard**: `attractor lint pipeline.dot` fails immediately if a `switch`
   node is missing its `key` attribute, rather than failing at runtime.

## Architecture

### `json_extract` node

```dot
parse [type=json_extract
       source="api_body"
       path=".data.user.name"
       key="username"
       default="anonymous"]
```

| Attribute | Required | Default | Description |
|-----------|----------|---------|-------------|
| `source`  | yes      | —       | Context key containing the JSON string |
| `path`    | yes      | —       | Dot-path to the target value (leading `.` optional) |
| `key`     | yes      | —       | Context key to store the extracted value |
| `default` | no       | `""`    | Value to store when path not found or source is empty |

**Path syntax**: dot-separated segments; numeric segments are treated as
array indices.  Examples: `.name`, `users.0.email`, `.meta.total`.

**Extracted value**: primitives (string, number, bool, null) are stored as
their string representation; objects and arrays are re-marshalled to compact
JSON.

Implementation: `pkg/pipeline/handlers/json_extract.go`

1. Get the JSON string from `pctx.Get(source)`.
2. If empty and `default` is set, store default and return.
3. `json.Unmarshal` into `any`.
4. Walk path segments: numeric → slice index, string → map key.
5. If path fails mid-walk and `default` is set, store default; else error.
6. Convert result to string and store in `pctx`.

### Validator attribute checking

`pkg/pipeline/validator.go` gains a new check driven by a package-level map:

```go
var nodeRequiredAttrs = map[NodeType][]string{
    NodeTypeSet:         {"key"},
    NodeTypeHTTP:        {"url"},
    NodeTypeAssert:      {"expr"},
    NodeTypeSleep:       {"duration"},
    NodeTypeSwitch:      {"key"},
    NodeTypeEnv:         {"key", "from"},
    NodeTypeReadFile:    {"key", "path"},
    NodeTypeWriteFile:   {"path", "content"},
    NodeTypeJSONExtract: {"source", "path", "key"},
}
```

For each node in the pipeline: look up its type in the map; for each
required attribute that is absent or empty, emit a `LintError`.

## Files

| File | Action | Description |
|------|--------|-------------|
| `pkg/pipeline/ast.go` | **Modify** | Add `NodeTypeJSONExtract` |
| `pkg/pipeline/handlers/json_extract.go` | **Create** | `JSONExtractHandler` |
| `pkg/pipeline/handlers/json_extract_test.go` | **Create** | Unit tests |
| `pkg/pipeline/validator.go` | **Modify** | `nodeRequiredAttrs` map + attr check loop |
| `pkg/pipeline/validator_test.go` | **Modify** | Tests for missing-attr lint errors |
| `cmd/attractor/main.go` | **Modify** | Register `json_extract` |
| `examples/json_extract.dot` | **Create** | Example pipeline |

## Definition of Done

### Functional
- [ ] `json_extract` extracts a top-level scalar from a JSON string
- [ ] `json_extract` traverses nested objects via dot-path
- [ ] `json_extract` indexes into JSON arrays with numeric segments
- [ ] `json_extract` uses `default` when path is not found
- [ ] `json_extract` re-marshals objects/arrays to JSON string
- [ ] `attractor lint` emits errors for nodes missing required attributes
- [ ] `attractor lint` reports all missing attrs (not just the first)
- [ ] `examples/json_extract.dot` passes `attractor lint`

### Correctness
- [ ] `TestJSONExtractScalar` — top-level string/number extracted
- [ ] `TestJSONExtractNested` — nested path traversal
- [ ] `TestJSONExtractArrayIndex` — numeric segment indexes array
- [ ] `TestJSONExtractDefault` — missing path uses default
- [ ] `TestJSONExtractObject` — sub-object marshalled back to JSON
- [ ] `TestJSONExtractEmptySource` — empty source + default
- [ ] `TestJSONExtractMissingAttrs` — error on missing required attrs
- [ ] `TestValidateRequiredAttrs` — lint catches missing attrs for all checked types
- [ ] `TestValidateRequiredAttrsPass` — valid nodes produce no errors

### Quality
- [ ] `go test -race ./...` — all green
- [ ] `golangci-lint run ./...` — 0 issues

## Dependencies

- Sprint 011 complete
- No new external dependencies (`encoding/json` is stdlib)
