# Sprint 009: Resilience & Flow — `retry` attribute + `sleep` node

## Overview

Two additions that make pipelines more robust and controllable in
production environments:

1. **`retry` attribute (engine-level)** — any node can declare
   `retry_max="N"` and `retry_delay="Xs"` to automatically re-run on
   failure.  The engine wraps handler dispatch generically, so every
   existing and future node type benefits without any handler changes.

2. **`sleep` node** — pause pipeline execution for a fixed duration.
   Useful for rate-limiting between API calls, polling intervals, and
   simple back-off between fan-out branches.

## Use Cases

1. **Resilient LLM calls**: a `codergen` node with `retry_max="3"
   retry_delay="5s"` retries on transient API errors without pipeline
   authors writing error-handling logic.
2. **HTTP polling**: `http` node → `assert` (check `status != "ready"`) →
   `sleep [duration="10s"]` → back to `http` node — poll an endpoint until
   a job completes.
3. **Rate limiting**: `sleep [duration="1s"]` between two `http` nodes that
   call a rate-limited third-party API.
4. **Staged retry with delay**: retry a flaky build step up to 5 times with
   a 30-second delay between attempts.

## Architecture

### `retry` attribute (engine-level)

Add a helper `executeWithRetry` in `engine.go` that wraps any
`handler.Handle` call:

```go
func executeWithRetry(ctx context.Context, h Handler, node *Node, pctx *PipelineContext) error
```

Logic:
1. Read `node.Attrs["retry_max"]` (default `"0"` → no retry).
2. Read `node.Attrs["retry_delay"]` (default `"0s"`); parse with
   `time.ParseDuration`.
3. Attempt 0: call `h.Handle(ctx, node, pctx)`.  If nil → return nil.
4. If error is an `ExitSignal` → return immediately (do not retry exits).
5. For attempt 1…retry_max: log `slog.Warn("retrying node", …)`, sleep
   `retry_delay`, call again.
6. After exhausting retries, return the last error annotated with the
   attempt count.

The `run()` method replaces the direct `handler.Handle` call with
`executeWithRetry`.  The `fan_out` path is unaffected (it calls
`executeFanOut` directly and does not use `executeWithRetry`).

### `sleep` node

```dot
pause [type=sleep duration="2s"]
```

| Attribute  | Required | Default | Description |
|------------|----------|---------|-------------|
| `duration` | yes      | —       | Sleep time, parsed by `time.ParseDuration` |

Implementation: `pkg/pipeline/handlers/sleep.go`

- Parse `duration`; error if missing or unparseable.
- Use `time.NewTimer` + `select` on `ctx.Done()` so the sleep can be
  cancelled.

## Files

| File | Action | Description |
|------|--------|-------------|
| `pkg/pipeline/ast.go` | **Modify** | Add `NodeTypeSleep` |
| `pkg/pipeline/engine.go` | **Modify** | Add `executeWithRetry`; call it from `run()` |
| `pkg/pipeline/handlers/sleep.go` | **Create** | `SleepHandler` |
| `pkg/pipeline/handlers/sleep_test.go` | **Create** | Unit tests |
| `pkg/pipeline/engine_retry_test.go` | **Create** | Engine-level retry tests |
| `cmd/attractor/main.go` | **Modify** | Register `sleep` in `buildRegistry` |
| `examples/retry_sleep.dot` | **Create** | Example pipeline |

## Definition of Done

### Functional
- [ ] Node with `retry_max="2"` retries up to 2 extra times on error
- [ ] `retry_delay="100ms"` introduces the specified delay between attempts
- [ ] `ExitSignal` from a handler is never retried
- [ ] `sleep` node pauses for the given duration
- [ ] `sleep` node is cancelled immediately when the context is cancelled
- [ ] `sleep` node errors when `duration` attribute is missing

### Correctness
- [ ] `TestRetrySucceedsOnSecondAttempt` — handler fails once, succeeds on retry
- [ ] `TestRetryExhausted` — all attempts fail; error mentions attempt count
- [ ] `TestRetryNoRetryByDefault` — node with no `retry_max` fails immediately
- [ ] `TestRetryExitNotRetried` — `ExitSignal` returns immediately, no retry
- [ ] `TestSleepNormal` — sleep completes; no error
- [ ] `TestSleepCancelled` — context cancelled mid-sleep returns error
- [ ] `TestSleepMissingDuration` — missing attr returns error

### Quality
- [ ] `go test -race ./...` — all green
- [ ] `golangci-lint run ./...` — 0 issues

## Dependencies

- Sprint 008 complete
- No new external dependencies
