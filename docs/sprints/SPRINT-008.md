# Sprint 008: External Integration — `http` node + `assert` node

## Overview

Two new node types that make pipelines useful for real-world integration
and self-validation:

1. **`http` node** — make an HTTP request from inside a pipeline, storing the
   response body and status code in the pipeline context so that downstream
   nodes (including conditional edges) can act on them without a `codergen`
   detour.

2. **`assert` node** — evaluate a condition expression against the pipeline
   context and fail fast if it is false.  Acts as a guardrail between stages.

## Use Cases

1. **Call a webhook**: POST a payload to a CI endpoint after a `codergen` step,
   then branch on HTTP status.
2. **Fetch configuration**: GET a JSON config file from an object-storage URL
   at runtime and inject values into subsequent nodes via `set`.
3. **Validate preconditions**: `assert [expr="seed != ''"]` at the pipeline
   start to catch misconfigured invocations immediately.
4. **Contract check**: after a `codergen` generates code, `assert` that
   `last_output != ''` before writing it to disk.

## Architecture

### `http` node

```dot
fetch [type=http url="https://api.example.com/data/{{.id}}"
       method="GET"
       response_key="api_body"
       status_key="api_status"]
```

| Attribute       | Required | Default                    | Description |
|-----------------|----------|----------------------------|-------------|
| `url`           | yes      | —                          | URL; template-rendered against context |
| `method`        | no       | `GET`                      | HTTP method: `GET`, `POST`, `PUT`, `DELETE`, … |
| `body`          | no       | `""`                       | Request body; template-rendered |
| `headers`       | no       | `""`                       | Semicolon-separated `Key:Value` pairs, e.g. `"Authorization:Bearer {{.token}};Content-Type:application/json"` |
| `response_key`  | no       | `<nodeID>_body`            | Context key for response body |
| `status_key`    | no       | `<nodeID>_status`          | Context key for HTTP status code (stored as string, e.g. `"200"`) |
| `timeout`       | no       | `"30s"`                    | Per-request timeout (e.g. `"10s"`, `"2m"`) |
| `fail_non2xx`   | no       | `"false"`                  | If `"true"`, return an error on non-2xx responses |

Implementation: `pkg/pipeline/handlers/http.go`

- Template-render URL, body, and each header value.
- Parse `timeout` with `time.ParseDuration`; fall back to `30s` if empty or
  invalid.
- Use `net/http` standard library only — no new dependencies.
- Store response body as a string; store status code as its decimal string
  (e.g., `"200"`, `"404"`).

### `assert` node

```dot
check [type=assert expr="api_status == '200' && last_output != ''"
       message="API call failed or produced empty output"]
```

| Attribute | Required | Default          | Description |
|-----------|----------|------------------|-------------|
| `expr`    | yes      | —                | Condition expression (same grammar as edge conditions) |
| `message` | no       | `"assertion failed"` | Error message on failure |

Implementation: `pkg/pipeline/handlers/assert.go`

- Calls the existing `pipeline.EvalCondition(expr, pctx.Snapshot())`.
- If `false`, returns an error: `"assert node %q: %s: expr=%q"`.

### AST changes

`pkg/pipeline/ast.go` — add two new `NodeType` constants:

```go
NodeTypeHTTP   NodeType = "http"
NodeTypeAssert NodeType = "assert"
```

### Registration

`cmd/attractor/main.go` — register both handlers in `buildRegistry`.

## Files

| File | Action | Description |
|------|--------|-------------|
| `pkg/pipeline/ast.go` | **Modify** | Add `NodeTypeHTTP`, `NodeTypeAssert` |
| `pkg/pipeline/handlers/http.go` | **Create** | `HttpHandler` implementation |
| `pkg/pipeline/handlers/assert.go` | **Create** | `AssertHandler` implementation |
| `cmd/attractor/main.go` | **Modify** | Register `http` and `assert` in `buildRegistry` |
| `pkg/pipeline/handlers/http_test.go` | **Create** | Unit tests using `httptest.NewServer` |
| `pkg/pipeline/handlers/assert_test.go` | **Create** | Unit tests for pass and fail cases |
| `examples/http_assert.dot` | **Create** | Example pipeline demonstrating both nodes |

## Definition of Done

### Functional
- [ ] `http` node makes GET requests and stores body + status in context
- [ ] `http` node makes POST requests with a template-rendered body
- [ ] `http` node applies custom `headers` to each request
- [ ] `http` node respects `timeout` attribute
- [ ] `http` node with `fail_non2xx="true"` returns error on non-2xx response
- [ ] `assert` node passes when expr is true
- [ ] `assert` node fails with descriptive error when expr is false
- [ ] Both nodes appear in `attractor lint` output for valid pipelines
- [ ] Example pipeline `examples/http_assert.dot` is valid (`attractor lint`)

### Correctness
- [ ] `TestHTTPNodeGet` — GET response body and status stored correctly
- [ ] `TestHTTPNodePost` — POST body forwarded; response stored correctly
- [ ] `TestHTTPNodeHeaders` — custom headers present on server-side
- [ ] `TestHTTPNodeTimeout` — slow server causes timeout error
- [ ] `TestHTTPNodeFail2xx` — `fail_non2xx=true` returns error on 404
- [ ] `TestHTTPNodeAllow2xx` — `fail_non2xx=false` (default) stores 404 without error
- [ ] `TestAssertPass` — true expression returns nil error
- [ ] `TestAssertFail` — false expression returns error containing the message
- [ ] `TestAssertMissingExpr` — missing `expr` attr returns error

### Quality
- [ ] `go test -race ./...` — all green
- [ ] `golangci-lint run ./...` — 0 issues

## Dependencies

- Sprint 007 complete
- No new external dependencies (`net/http` and `httptest` are stdlib)
