# Coding Agent Loop Spec Reference

This file is a stub pointing to the canonical specification.

**Canonical source**: https://github.com/strongdm/attractor

## Summary

The coding agent loop (`pkg/agent`) implements a tool-calling agent that:

1. Sends the user prompt to the LLM
2. If the response contains `tool_use` blocks, executes each tool
3. Sends tool results back as a `role:user` message
4. Repeats until the model stops requesting tools or the turn limit is reached

### Built-in Tools

| Tool           | Description                                      |
|----------------|--------------------------------------------------|
| `read_file`    | Read a file relative to the working directory    |
| `write_file`   | Write or overwrite a file                        |
| `list_dir`     | List directory contents                          |
| `run_command`  | Execute a shell command (with timeout sandbox)   |

### Safety

- `read_file` and `write_file` enforce path traversal protection via `safePath`
- `run_command` enforces a configurable timeout (default 30 s)
- The agent loop has a configurable max-turns limit to prevent runaway loops

### Context Management

The loop tracks token usage and applies a sliding-window truncation strategy
when approaching the model's context limit. A `TRUNCATED` marker is injected
into the history to indicate dropped messages.
