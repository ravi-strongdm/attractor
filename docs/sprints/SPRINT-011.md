# Sprint 011: File I/O — `read_file` node + `write_file` node

## Overview

Two file-system nodes that bridge the gap between the pipeline context and
the local file system:

1. **`read_file` node** — reads a file from disk into a pipeline context
   variable.  The path is template-rendered, so it can reference prior
   context values (e.g., a filename produced by a `codergen` step).

2. **`write_file` node** — writes a pipeline context value (or any
   template-rendered string) to a file.  Supports append mode and
   automatically creates parent directories.

## Use Cases

1. **Prompt from file**: `read_file [key="spec" path="docs/spec.md"]` →
   `codergen [prompt="Implement this spec:\n{{.spec}}"]`.
2. **Save agent output**: `codergen` → `write_file [path="out/result.go"
   content="{{.last_output}}"]`.
3. **Dynamic file paths**: `write_file [path="reports/{{.repo_name}}.md"
   content="{{.analysis}}"]` — path computed from context.
4. **Log aggregation**: `write_file [path="run.log" content="{{.summary}}\n"
   append="true"]` accumulates entries across pipeline nodes.
5. **Config injection**: `read_file [key="config_json" path="config.json"]`
   → `set [key="timeout" value="..."]` (parse timeout from config text).

## Architecture

### `read_file` node

```dot
load_spec [type=read_file
           key="spec"
           path="docs/{{.component}}.md"
           required="true"]
```

| Attribute  | Required | Default  | Description |
|------------|----------|----------|-------------|
| `key`      | yes      | —        | Context key to store file contents |
| `path`     | yes      | —        | File path; template-rendered against context |
| `required` | no       | `"true"` | If `"true"`, error when file not found; if `"false"`, set key to `""` |

Implementation: `pkg/pipeline/handlers/read_file.go`

- Render `path` as a Go template against `pctx.Snapshot()`.
- Read file with `os.ReadFile`.
- On `os.IsNotExist`: error if `required != "false"`, otherwise set key to `""`.
- Store file contents as a string in `pctx`.

### `write_file` node

```dot
save [type=write_file
      path="output/{{.task_id}}.go"
      content="{{.last_output}}"
      mode="0644"
      append="false"]
```

| Attribute | Required | Default  | Description |
|-----------|----------|----------|-------------|
| `path`    | yes      | —        | Destination path; template-rendered |
| `content` | yes      | —        | File contents; template-rendered |
| `mode`    | no       | `"0644"` | Unix file mode (octal string) |
| `append`  | no       | `"false"`| If `"true"`, append to existing file |

Implementation: `pkg/pipeline/handlers/write_file.go`

- Render `path` and `content` as Go templates.
- Parse `mode` as octal (default `0644`); use `fs.FileMode`.
- Create parent directories with `os.MkdirAll`.
- If `append == "true"`: open with `os.O_APPEND|os.O_CREATE|os.O_WRONLY`.
- Otherwise: use `os.WriteFile`.

## Files

| File | Action | Description |
|------|--------|-------------|
| `pkg/pipeline/ast.go` | **Modify** | Add `NodeTypeReadFile`, `NodeTypeWriteFile` |
| `pkg/pipeline/handlers/read_file.go` | **Create** | `ReadFileHandler` |
| `pkg/pipeline/handlers/read_file_test.go` | **Create** | Unit tests |
| `pkg/pipeline/handlers/write_file.go` | **Create** | `WriteFileHandler` |
| `pkg/pipeline/handlers/write_file_test.go` | **Create** | Unit tests |
| `cmd/attractor/main.go` | **Modify** | Register `read_file`, `write_file` |
| `examples/file_io.dot` | **Create** | Example pipeline |

## Definition of Done

### Functional
- [ ] `read_file` reads a file and stores its contents in context
- [ ] `read_file` renders the path as a template
- [ ] `read_file` errors when required file is missing
- [ ] `read_file` stores `""` when file missing and `required="false"`
- [ ] `write_file` writes content to disk, creating parent directories
- [ ] `write_file` renders both path and content as templates
- [ ] `write_file` appends when `append="true"`
- [ ] `write_file` respects the `mode` attribute
- [ ] `examples/file_io.dot` passes `attractor lint`

### Correctness
- [ ] `TestReadFileOK` — reads existing file into context
- [ ] `TestReadFileTemplatePath` — path resolved via template
- [ ] `TestReadFileMissingRequired` — error on missing required file
- [ ] `TestReadFileMissingOptional` — empty string stored for optional missing
- [ ] `TestWriteFileCreatesFile` — file created with expected content
- [ ] `TestWriteFileTemplatePath` — path/content rendered via template
- [ ] `TestWriteFileCreatesParentDirs` — nested dirs created automatically
- [ ] `TestWriteFileAppend` — append mode accumulates content
- [ ] `TestWriteFileMode` — file written with requested permissions
- [ ] `TestReadWriteRoundtrip` — write then read back matches

### Quality
- [ ] `go test -race ./...` — all green
- [ ] `golangci-lint run ./...` — 0 issues

## Dependencies

- Sprint 010 complete
- No new external dependencies
