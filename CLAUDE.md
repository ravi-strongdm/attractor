# Attractor — Developer Conventions

## Build & Test

```bash
# Build all packages (requires Go 1.26+)
export PATH="/opt/homebrew/bin:$PATH"
go build ./...

# Run all tests with race detector
go test -race ./...

# Lint (requires golangci-lint v2.x)
golangci-lint run ./...

# Build the binary
mkdir -p bin
go build -o bin/attractor ./cmd/attractor
```

## Adding a New LLM Provider

1. Copy `pkg/llm/providers/anthropic.go` as a starting point.
2. Rename the file and package is still `providers`.
3. Call `llm.RegisterProvider("yourname", factory)` in the package `init()`.
4. Blank-import the providers package in `cmd/attractor/main.go`:
   ```go
   import _ "github.com/ravi-parthasarathy/attractor/pkg/llm/providers"
   ```
5. Users select the provider via `model="yourname:model-id"` in their DOT file.

## Pipeline DOT Syntax

Nodes:
| type       | purpose                                         |
|------------|-------------------------------------------------|
| `start`    | Entry point (exactly one required)              |
| `exit`     | Terminal node (exactly one required)            |
| `set`      | Set a pipeline context variable                 |
| `codergen` | LLM coding agent loop (requires API key)        |
| `wait.human` | Prompt for human input                        |
| `fan_out`  | Fork to parallel branches                       |
| `fan_in`   | Join parallel branches                          |

Edge labels are Go template expressions evaluated against `PipelineContext.Vars`.
Unconditional edges have no label or `label="_"`.

## Project Layout

```
cmd/attractor/       CLI entry point
pkg/llm/             Unified LLM client interface and error types
pkg/llm/providers/   Anthropic and OpenAI adapters (init() registration)
pkg/agent/           Coding agent loop and tool definitions
pkg/pipeline/        DOT parser, AST, validator, engine
pkg/pipeline/handlers/ Node type implementations
docs/sprints/        Sprint planning documents
docs/specs/          Spec reference stubs
examples/            Example DOT pipelines
```

## Sprint Process

Sprint documents live in `docs/sprints/SPRINT-NNN.md`.
Draft artifacts are in `docs/sprints/drafts/`.
Run `/megaplan` to start a new sprint planning session.

Canonical specs: https://github.com/strongdm/attractor
