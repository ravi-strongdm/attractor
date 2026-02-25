# Sprint 010: Routing & Configuration — `switch` node + `env` node

## Overview

Two additions that round out everyday pipeline authoring:

1. **`switch` node** — multi-way branch based on the exact string value of a
   pipeline context key.  Cleaner than writing `key == 'value'` conditions on
   every outgoing edge, and provides an explicit `default` path.

2. **`env` node** — injects an OS environment variable into the pipeline
   context at runtime, with optional `required` enforcement and a `default`
   fallback.  Enables secrets and environment-specific config to flow in
   without hardcoding values in DOT files.

## Use Cases

1. **Multi-status routing**: after an `http` node, `switch [key="api_status"]`
   routes to `ok_handler`, `retry_handler`, or `fail_handler` without three
   template conditions on edges.
2. **Environment-driven model selection**: `env [key="model" from="LLM_MODEL"
   default="anthropic:claude-sonnet-4-6"]` → `codergen [model="{{.model}}"]`.
3. **Secret injection**: `env [key="token" from="GITHUB_TOKEN" required="true"]`
   fails fast with a clear error if the secret is missing.
4. **Feature flags**: `env [key="debug" from="PIPELINE_DEBUG" default="false"]`
   → `switch [key="debug"]` routes to a verbose or quiet path.

## Architecture

### `switch` node

```dot
route [type=switch key="status"]

route -> ok_path   [label="ok"]
route -> warn_path [label="warn"]
route -> fail_path [label="error"]
route -> unknown   [label="_"]    // default / fallback
```

**Handler** (`pkg/pipeline/handlers/switch.go`):
- Validates that `key` attribute is present.
- Validates that `pctx` contains the key (logs a warning if missing; routing
  will fall to the default edge).
- Returns nil — routing is performed by the engine.

**Engine** (`pkg/pipeline/engine.go`, `selectNext`):
- When `node.Type == NodeTypeSwitch`, call `selectNextSwitch` instead of the
  normal condition evaluator.
- `selectNextSwitch` reads `node.Attrs["key"]`, fetches `pctx.Get(key)` as a
  string, then iterates outgoing edges:
  1. First pass: return the first edge whose `Condition` equals the context
     value (exact string match).
  2. Second pass (fallback): return the first edge whose `Condition` is `""`,
     `"_"`, or `"default"`.
  3. If nothing matches: return an error describing the unmatched value.

### `env` node

```dot
load_token [type=env key="gh_token" from="GITHUB_TOKEN" required="true"]
load_model [type=env key="model"    from="LLM_MODEL"    default="anthropic:claude-sonnet-4-6"]
```

| Attribute  | Required | Default | Description |
|------------|----------|---------|-------------|
| `key`      | yes      | —       | Context key to set |
| `from`     | yes      | —       | OS environment variable name |
| `required` | no       | `false` | Error if the env var is unset or empty |
| `default`  | no       | `""`    | Value to use when env var is unset |

Implementation: `pkg/pipeline/handlers/env.go`

1. Look up `os.Getenv(from)`.
2. If empty and `required == "true"`: return error.
3. If empty and `default` is set: use the default value.
4. `pctx.Set(key, value)`.

## Files

| File | Action | Description |
|------|--------|-------------|
| `pkg/pipeline/ast.go` | **Modify** | Add `NodeTypeSwitch`, `NodeTypeEnv` |
| `pkg/pipeline/engine.go` | **Modify** | `selectNextSwitch`; call from `selectNext` |
| `pkg/pipeline/handlers/switch.go` | **Create** | `SwitchHandler` |
| `pkg/pipeline/handlers/switch_test.go` | **Create** | Unit tests |
| `pkg/pipeline/handlers/env.go` | **Create** | `EnvHandler` |
| `pkg/pipeline/handlers/env_test.go` | **Create** | Unit tests |
| `pkg/pipeline/engine_switch_test.go` | **Create** | Engine-level switch routing tests |
| `cmd/attractor/main.go` | **Modify** | Register `switch` and `env` in `buildRegistry` |
| `examples/switch_env.dot` | **Create** | Example pipeline |

## Definition of Done

### Functional
- [ ] `switch` node routes to the edge whose label matches the context value
- [ ] `switch` node falls back to `_` / `default` edge when no label matches
- [ ] `switch` node errors when no edge matches and no default exists
- [ ] `env` node injects the OS env var into the pipeline context
- [ ] `env` node uses `default` when env var is unset
- [ ] `env` node errors on missing required env var
- [ ] Both nodes appear in valid `attractor lint` output
- [ ] `examples/switch_env.dot` passes `attractor lint`

### Correctness
- [ ] `TestSwitchRouteMatch` — routes to correct edge on value match
- [ ] `TestSwitchRouteDefault` — falls to `_` edge when no match
- [ ] `TestSwitchNoDefault` — error when value unmatched and no default
- [ ] `TestSwitchMissingKey` — error when `key` attr absent
- [ ] `TestEnvPresent` — env var set → injected into context
- [ ] `TestEnvDefault` — env var unset, default used
- [ ] `TestEnvRequiredMissing` — required var unset → error
- [ ] `TestEnvRequiredPresent` — required var set → no error
- [ ] `TestEnvMissingAttrs` — missing `key` or `from` → error

### Quality
- [ ] `go test -race ./...` — all green
- [ ] `golangci-lint run ./...` — 0 issues

## Dependencies

- Sprint 009 complete
- No new external dependencies
