package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"
)

const defaultCommandTimeout = 30 * time.Second

// RunCommandTool executes a shell command in the working directory.
type RunCommandTool struct {
	workdir string
	timeout time.Duration
}

// NewRunCommandTool creates a RunCommandTool sandboxed to workdir with a 30s timeout.
func NewRunCommandTool(workdir string) *RunCommandTool {
	return &RunCommandTool{workdir: workdir, timeout: defaultCommandTimeout}
}

func (t *RunCommandTool) Name() string        { return "run_command" }
func (t *RunCommandTool) Description() string { return "Run a shell command and return its output." }
func (t *RunCommandTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"command":{"type":"string","description":"Shell command to execute"}},"required":["command"]}`)
}

func (t *RunCommandTool) Execute(ctx context.Context, input json.RawMessage) (string, error) {
	var params struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("run_command: invalid input: %w", err)
	}
	if params.Command == "" {
		return "", fmt.Errorf("run_command: command must not be empty")
	}

	tctx, cancel := context.WithTimeout(ctx, t.timeout)
	defer cancel()

	// /bin/sh -c to run the command string; no direct arg interpolation.
	cmd := exec.CommandContext(tctx, "/bin/sh", "-c", params.Command)
	cmd.Dir = t.workdir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	out := stdout.String()
	if stderr.Len() > 0 {
		out += "\nSTDERR:\n" + stderr.String()
	}
	if runErr != nil {
		return out, fmt.Errorf("run_command: %w", runErr)
	}
	return out, nil
}
