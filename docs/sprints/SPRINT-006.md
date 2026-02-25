# Sprint 006: Developer Ergonomics

## Overview

Three quality-of-life improvements that unblock real coding-agent workflows:

1. **`patch_file` tool** — targeted string replacement inside a file without
   rewriting the entire content.  Without this, agents must read a file, modify
   it in-memory, and write the entire thing back — token-expensive and error-prone.
   `patch_file(path, old_string, new_string)` finds the first exact occurrence of
   `old_string` and replaces it, returning an error if the string is not present.

2. **`--var key=value` CLI flag** — inject arbitrary variables into the pipeline
   context at startup.  Currently only `--seed` sets one fixed key.  A repeatable
   `--var` flag lets users parametrise any pipeline without modifying the DOT file.

3. **`attractor version` command** — print the binary version, VCS revision, and
   build timestamp using `runtime/debug.ReadBuildInfo`.

## Use Cases

1. **Targeted edit**: agent reads a file, identifies a function signature to change,
   calls `patch_file` with the old and new signatures — no full-file rewrite needed
2. **Parametric pipeline**: `attractor run pipeline.dot --var env=prod --var region=us-east-1`
3. **CI version check**: `attractor version` in a Dockerfile prints the exact binary provenance

## Architecture

### `patch_file` tool

```go
// pkg/agent/tools/patchfile.go
type PatchFileTool struct { workdir string }

// Input: { "path": "…", "old_string": "…", "new_string": "…" }
// 1. safePath check
// 2. os.ReadFile
// 3. strings.Index(content, old_string) — error if -1
// 4. strings.Replace(content, old_string, new_string, 1)
// 5. os.WriteFile
// Output: "patched <path> (replaced <N> bytes)"
```

Exact-string matching (not regex) — safe, predictable, easy for agents to use.
Returns a clear error if `old_string` is not found, so the agent knows the patch
failed and must re-read the file.

### `--var key=value`

`run` and `resume` subcommands gain a `--var` `StringArray` flag.
Each value is split on the first `=`; key and value are set in `PipelineContext`
before execution begins.  Invalid entries (no `=`) are rejected with an error.

### `attractor version`

Uses `runtime/debug.ReadBuildInfo()` to read the module version and VCS settings
embedded at build time.  Falls back gracefully when not built from a module or
when VCS info is not embedded (e.g. `go run`).

## Files

| File | Action | Description |
|------|--------|-------------|
| `pkg/agent/tools/patchfile.go` | **Create** | `patch_file` tool implementation |
| `pkg/agent/tools/tools_test.go` | **Modify** | Tests for `patch_file` |
| `pkg/pipeline/handlers/codergen.go` | **Modify** | Register `patch_file` tool |
| `cmd/attractor/main.go` | **Modify** | `--var` flag + `version` command |

## Definition of Done

### Functional
- [ ] `patch_file` replaces the first occurrence of `old_string` in a file
- [ ] `patch_file` returns a descriptive error when `old_string` is not found
- [ ] `attractor run pipeline.dot --var greeting=hello --var name=world` injects both vars
- [ ] `attractor version` prints module path, version, and VCS revision

### Correctness
- [ ] `TestPatchFileTool` — successful patch, byte-count reported
- [ ] `TestPatchFileTool_NotFound` — error when old_string absent
- [ ] `TestPatchFileTool_PathTraversal` — rejected
- [ ] `TestPatchFileTool_FirstOnly` — only first occurrence replaced
- [ ] `TestVar_Injection` — vars appear in context before first node

### Quality
- [ ] `go test -race ./...` — all green
- [ ] `golangci-lint run ./...` — 0 issues

## Dependencies

- Sprint 005 complete
- No new external dependencies
