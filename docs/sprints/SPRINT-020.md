# Sprint 020: `wait.human` Improvements + README + Release Workflow

## Overview

Three finishing touches that make attractor production-ready:

1. **`wait.human` improvements** — add `key` attr (custom storage key) and
   `options` attr (display a numbered menu and validate the choice).

2. **README.md** — comprehensive project documentation covering installation,
   quick-start, all 24 node types, CLI reference, and examples.

3. **GitHub Actions release workflow** — `.github/workflows/release.yml` that
   builds and publishes multi-platform binaries on every `v*` tag push.

## wait.human Improvements

**Current** behaviour: stores response as `<nodeID>_response`, no options display.

**New** attrs:

| Attribute | Required | Default              | Description |
|-----------|----------|----------------------|-------------|
| `prompt`  | no       | node ID message      | Prompt text shown to user |
| `key`     | no       | `<nodeID>_response`  | Context key where response is stored |
| `options` | no       | `""`                 | Comma-separated list of valid responses |

When `options` is set:
- Display numbered list: `  1) yes\n  2) no\n`
- Accept either the number or the exact option text (case-insensitive).
- On invalid input: re-prompt (loop until valid or EOF).
- Store the **canonical option text** (not the number).

This makes `wait.human` → `switch` routing clean:
```dot
ask    [type=wait.human prompt="Deploy to prod?" options="yes,no" key="confirm"]
branch [type=switch key="confirm"]
branch -> deploy [label=yes]
branch -> abort  [label=no]
```

## README

Sections:
1. **What is Attractor** — one-paragraph pitch
2. **Installation** — `go install` + pre-built binaries link
3. **Quick Start** — 5-step hello-world pipeline
4. **CLI Reference** — `run`, `lint`, `resume`, `graph`, flags table
5. **Node Type Reference** — table for all 24 node types with attributes
6. **Pipeline DOT Syntax** — format, edge conditions, templates
7. **Examples** — annotated list of `examples/*.dot`
8. **Configuration** — `--var`, `--var-file`, stylesheet, log level/format
9. **Contributing** — build, test, lint commands

## Release Workflow

`.github/workflows/release.yml`:
- Trigger: push of tag matching `v*.*.*`
- Matrix: `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`, `windows/amd64`
- Steps: checkout → setup-go → build binary → upload as GitHub Release asset
- Binary naming: `attractor-<os>-<arch>[.exe]`
- Uses `softprops/action-gh-release` to create/update the release

## Files

| File | Action | Description |
|------|--------|-------------|
| `pkg/pipeline/handlers/human.go` | **Modify** | Add `key` and `options` support |
| `pkg/pipeline/handlers/human_test.go` | **Create** | Unit tests for new attrs |
| `README.md` | **Create** | Full project documentation |
| `.github/workflows/release.yml` | **Create** | Multi-platform binary release |

## Definition of Done

### wait.human
- [ ] `key` attr controls where response is stored
- [ ] `options` attr displays numbered menu
- [ ] User can enter number or option text
- [ ] Invalid input re-prompts until valid
- [ ] Stored value is canonical option text
- [ ] Without `options`, behavior unchanged from before

### README
- [ ] All 24 node types documented with attribute tables
- [ ] CLI reference covers all flags
- [ ] Quick-start example works end-to-end
- [ ] Installation instructions for both go install and binaries

### Release
- [ ] Workflow triggers on `v*` tag
- [ ] Builds for all 5 platforms
- [ ] Binaries uploaded to GitHub Release

### Quality
- [ ] `go test -race ./...` — all green
- [ ] `golangci-lint run ./...` — 0 issues

## Dependencies
- Sprint 019 complete
- No new external dependencies
