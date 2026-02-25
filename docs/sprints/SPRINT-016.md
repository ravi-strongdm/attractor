# Sprint 016: Shell Integration — `exec` node + `json_pack` node

## Overview

Two nodes that make attractor useful for real-world automation:

1. **`exec` node** — run an arbitrary shell command via `/bin/sh -c`, capture
   stdout, stderr, and exit code into separate pipeline context keys.  Makes
   it trivial to call `git`, `make`, `npm test`, `curl`, or any CLI tool as
   a first-class pipeline step without spinning up a full codergen agent.

2. **`json_pack` node** — pack a set of pipeline context keys into a single
   JSON object string.  The inverse of `json_decode`: where `json_decode`
   expands one key into many, `json_pack` collapses many keys into one.
   Useful for constructing payloads for HTTP or prompt nodes.

## Use Cases

1. **Run tests, capture output**:
   ```dot
   test [type=exec
         cmd="go test ./... 2>&1"
         stdout_key="test_output"
         exit_code_key="test_exit"]
   check [type=assert expr="test_exit == \"0\"" message="Tests failed"]
   ```

2. **Git status in pipeline**:
   ```dot
   status [type=exec
           cmd="git log --oneline -5"
           stdout_key="recent_commits"]
   ```

3. **Build a JSON payload from context**:
   ```dot
   pack [type=json_pack
         keys="model,temperature,max_tokens"
         output="request_body"]
   post [type=http url="https://api.example.com/generate"
         method="POST" body="{{.request_body}}"]
   ```

4. **Pipe exec output into prompt**:
   ```dot
   lint  [type=exec cmd="golangci-lint run ./..." stdout_key="lint_out"]
   fix   [type=prompt
          prompt="Fix these lint errors:\n{{.lint_out}}"
          key="fix_plan"]
   ```

## Architecture

### `exec` node

```dot
n [type=exec
   cmd="<shell command template>"
   stdout_key="<optional, default: nodeID_stdout>"
   stderr_key="<optional, default: not stored>"
   exit_code_key="<optional, default: not stored>"
   workdir="<optional, template, default: handler Workdir>"
   timeout="<optional duration string, e.g. 30s>"
   fail_on_error="<true|false, default: true>"]
```

| Attribute      | Required | Default           | Description |
|----------------|----------|-------------------|-------------|
| `cmd`          | yes      | —                 | Shell command; Go template rendered against pctx |
| `stdout_key`   | no       | `<nodeID>_stdout` | Context key for captured stdout |
| `stderr_key`   | no       | `""` (not stored) | Context key for captured stderr |
| `exit_code_key`| no       | `""` (not stored) | Context key for exit code string |
| `workdir`      | no       | handler `Workdir` | Working directory; template rendered |
| `timeout`      | no       | `""` (none)       | `time.ParseDuration` string; 0 = no extra timeout |
| `fail_on_error`| no       | `"true"`          | Return error when exit code ≠ 0 |

**Execution**:

1. Render `cmd` as a Go template against `pctx.Snapshot()`.
2. Render `workdir` template (use handler default if empty).
3. Build `exec.CommandContext(ctx, "/bin/sh", "-c", renderedCmd)`.
4. If `timeout` attr is set and parses to > 0, wrap `ctx` with that timeout.
5. Capture `cmd.Stdout` and `cmd.Stderr` into `bytes.Buffer`.
6. Call `cmd.Run()`.
7. Store outputs:
   - stdout → `stdout_key` (default `<nodeID>_stdout`).
   - stderr → `stderr_key` if non-empty attr.
   - exit code as decimal string → `exit_code_key` if non-empty attr.
   - Also set `last_output` = stdout.
8. If exit code ≠ 0 and `fail_on_error` ≠ `"false"`, return error wrapping
   the exit code and first line of stderr.

**ExecHandler** struct:
```go
type ExecHandler struct {
    Workdir string
}
```

Implementation: `pkg/pipeline/handlers/exec.go`

### `json_pack` node

```dot
n [type=json_pack
   keys="key1,key2,key3"
   output="packed_json"]
```

| Attribute | Required | Default | Description |
|-----------|----------|---------|-------------|
| `keys`    | yes      | —       | Comma-separated list of context key names to include |
| `output`  | yes      | —       | Context key to store the resulting JSON object string |

**Execution**:

1. Split `keys` on commas; trim whitespace from each.
2. For each key name: read `pctx.GetString(name)`, add to a `map[string]string`.
3. `json.Marshal` the map → store compact JSON in `output`.
4. Empty `keys` value → store `"{}"` without error.

The values in the resulting JSON object are always JSON strings (consistent
with how `--var` and context vars are stored).

Implementation: `pkg/pipeline/handlers/json_pack.go`

## Files

| File | Action | Description |
|------|--------|-------------|
| `pkg/pipeline/ast.go` | **Modify** | Add `NodeTypeExec`, `NodeTypeJSONPack` |
| `pkg/pipeline/handlers/exec.go` | **Create** | `ExecHandler` |
| `pkg/pipeline/handlers/exec_test.go` | **Create** | Unit tests |
| `pkg/pipeline/handlers/json_pack.go` | **Create** | `JSONPackHandler` |
| `pkg/pipeline/handlers/json_pack_test.go` | **Create** | Unit tests |
| `pkg/pipeline/validator.go` | **Modify** | Add required attrs |
| `cmd/attractor/main.go` | **Modify** | Register `exec` and `json_pack` |
| `examples/exec_pack.dot` | **Create** | Example pipeline |

## Definition of Done

### Functional
- [ ] `exec` runs a shell command and captures stdout to `stdout_key`
- [ ] `exec` captures stderr to `stderr_key` when attr set
- [ ] `exec` stores exit code to `exit_code_key` when attr set
- [ ] `exec` fails pipeline by default on non-zero exit (fail_on_error=true)
- [ ] `exec` succeeds on non-zero exit when `fail_on_error="false"`
- [ ] `exec` respects `timeout` attr (command killed on timeout)
- [ ] `exec` sets `last_output` = stdout
- [ ] `exec` template-renders `cmd` against pipeline context
- [ ] `json_pack` packs listed keys into a JSON object string
- [ ] `json_pack` with empty `keys` stores `"{}"` without error
- [ ] `json_pack` missing keys stored as empty string in JSON object
- [ ] `attractor lint` catches missing `exec` attr (`cmd`)
- [ ] `attractor lint` catches missing `json_pack` attrs (`keys`, `output`)
- [ ] `examples/exec_pack.dot` passes `attractor lint`

### Correctness
- [ ] `TestExecBasic` — command stdout captured in default key
- [ ] `TestExecStderr` — stderr captured in named key
- [ ] `TestExecExitCode` — exit code stored as string
- [ ] `TestExecFailOnError` — non-zero exit returns error by default
- [ ] `TestExecNoFailOnError` — fail_on_error=false suppresses error
- [ ] `TestExecTimeout` — command killed after timeout, context error returned
- [ ] `TestExecTemplateCmd` — cmd template rendered against context
- [ ] `TestExecSetsLastOutput` — last_output set to stdout
- [ ] `TestExecMissingCmdAttr` — error for missing cmd
- [ ] `TestJSONPackBasic` — keys packed to JSON object
- [ ] `TestJSONPackEmptyKeys` — empty keys → `"{}"`
- [ ] `TestJSONPackMissingContextKey` — absent key stored as `""`
- [ ] `TestJSONPackMissingAttrs` — validator catches missing keys/output

### Quality
- [ ] `go test -race ./...` — all green
- [ ] `golangci-lint run ./...` — 0 issues

## Dependencies

- Sprint 015 complete
- No new external dependencies
