# Sprint 017: String Utilities — `regex` node + `string_transform` node

## Overview

Two pure-computation nodes for text manipulation — no LLM required.

1. **`regex` node** — apply a regular expression to a context value to extract
   a capture group or test for a match.  Fills the gap between `json_extract`
   (structured data) and `exec` (shell grep) for simple pattern matching.

2. **`string_transform` node** — apply a chain of deterministic string
   operations (trim, upper, lower, replace) to a context value and store the
   result.  Eliminates the need for `exec "echo … | tr …"` workarounds.

## Use Cases

1. **Extract version from build output**:
   ```dot
   ver [type=regex source="build_out" pattern="v(\\d+\\.\\d+\\.\\d+)" group="1" key="version"]
   ```

2. **Normalise user input for switch routing**:
   ```dot
   norm [type=string_transform source="user_input" ops="trim,lower" key="cmd"]
   route [type=switch key="cmd"]
   ```

3. **Test for match (boolean branch)**:
   ```dot
   check [type=regex source="status" pattern="^ok" key="matched"]
   gate  [type=assert expr="matched"]
   ```

4. **Replace placeholders**:
   ```dot
   sub [type=string_transform source="template_str"
        ops="replace" old="__NAME__" new="{{.username}}" key="filled"]
   ```

## Architecture

### `regex` node

```dot
n [type=regex
   source="ctx_key"
   pattern="<RE2 regexp>"
   key="output_key"
   group="0"
   no_match=""]
```

| Attribute  | Required | Default | Description |
|------------|----------|---------|-------------|
| `source`   | yes      | —       | Context key of the input string |
| `pattern`  | yes      | —       | RE2 regular expression |
| `key`      | yes      | —       | Context key for the result |
| `group`    | no       | `"0"`   | Capture group index (0 = whole match) |
| `no_match` | no       | `""`    | Value stored when pattern does not match |

**Execution**:
1. Compile pattern (return error on invalid RE2).
2. `regexp.FindStringSubmatch(source_value)`.
3. If no match: store `no_match` value in `key`.
4. If match: store `matches[group]` in `key`; error if `group` index out of range.

Implementation: `pkg/pipeline/handlers/regex.go`

### `string_transform` node

```dot
n [type=string_transform
   source="ctx_key"
   ops="trim,lower"
   key="output_key"
   old="find"
   new="replace"]
```

| Attribute | Required | Default | Description |
|-----------|----------|---------|-------------|
| `source`  | yes      | —       | Context key of the input string |
| `ops`     | yes      | —       | Comma-separated list of operations (see below) |
| `key`     | yes      | —       | Context key for the result |
| `old`     | no       | `""`    | Search string for `replace` op (template rendered) |
| `new`     | no       | `""`    | Replacement string for `replace` op (template rendered) |

**Supported operations** (applied left-to-right):

| Op        | Description |
|-----------|-------------|
| `trim`    | `strings.TrimSpace` |
| `upper`   | `strings.ToUpper` |
| `lower`   | `strings.ToLower` |
| `replace` | `strings.ReplaceAll(s, old, new)` |

Unknown ops return an error.

Implementation: `pkg/pipeline/handlers/string_transform.go`

## Files

| File | Action | Description |
|------|--------|-------------|
| `pkg/pipeline/ast.go` | **Modify** | Add `NodeTypeRegex`, `NodeTypeStringTransform` |
| `pkg/pipeline/handlers/regex.go` | **Create** | `RegexHandler` |
| `pkg/pipeline/handlers/regex_test.go` | **Create** | Unit tests |
| `pkg/pipeline/handlers/string_transform.go` | **Create** | `StringTransformHandler` |
| `pkg/pipeline/handlers/string_transform_test.go` | **Create** | Unit tests |
| `pkg/pipeline/validator.go` | **Modify** | Add required attrs |
| `cmd/attractor/main.go` | **Modify** | Register handlers |
| `examples/string_utils.dot` | **Create** | Example pipeline |

## Definition of Done

### Functional
- [ ] `regex` extracts whole match (group=0) by default
- [ ] `regex` extracts named capture group by index
- [ ] `regex` stores `no_match` value when pattern doesn't match
- [ ] `regex` returns error for invalid pattern
- [ ] `regex` returns error for out-of-range group index
- [ ] `string_transform` trim removes leading/trailing whitespace
- [ ] `string_transform` upper/lower convert case
- [ ] `string_transform` replace substitutes all occurrences
- [ ] `string_transform` applies ops in order (chain)
- [ ] `string_transform` returns error for unknown op
- [ ] Both nodes validated by `attractor lint`

### Correctness
- [ ] `TestRegexWholeMatch` — group=0 returns full match
- [ ] `TestRegexCaptureGroup` — group=1 returns first capture
- [ ] `TestRegexNoMatch` — no_match value stored
- [ ] `TestRegexInvalidPattern` — error for bad RE2
- [ ] `TestRegexGroupOutOfRange` — error when group > submatches
- [ ] `TestStringTransformTrim` — whitespace trimmed
- [ ] `TestStringTransformUpperLower` — case conversion
- [ ] `TestStringTransformReplace` — all occurrences replaced
- [ ] `TestStringTransformChain` — multiple ops applied in order
- [ ] `TestStringTransformUnknownOp` — error for unknown op

### Quality
- [ ] `go test -race ./...` — all green
- [ ] `golangci-lint run ./...` — 0 issues

## Dependencies
- Sprint 016 complete
- No new external dependencies
