package handlers

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/ravi-parthasarathy/attractor/pkg/pipeline"
)

// ExecHandler runs a shell command and stores its stdout, stderr, and exit
// code in the pipeline context.
type ExecHandler struct {
	Workdir string
}

func (h *ExecHandler) Handle(ctx context.Context, node *pipeline.Node, pctx *pipeline.PipelineContext) error {
	cmdTpl := node.Attrs["cmd"]
	if cmdTpl == "" {
		return fmt.Errorf("exec node %q: missing 'cmd' attribute", node.ID)
	}

	// Render cmd template.
	snapshot := pctx.Snapshot()
	renderedCmd, err := renderTemplate(cmdTpl, snapshot)
	if err != nil {
		return fmt.Errorf("exec node %q: cmd template error: %w", node.ID, err)
	}

	// Resolve working directory.
	workdir := h.Workdir
	if wdTpl := node.Attrs["workdir"]; wdTpl != "" {
		wd, wdErr := renderTemplate(wdTpl, snapshot)
		if wdErr != nil {
			return fmt.Errorf("exec node %q: workdir template error: %w", node.ID, wdErr)
		}
		workdir = wd
	}

	// Apply per-node timeout if set.
	runCtx := ctx
	if timeoutStr := node.Attrs["timeout"]; timeoutStr != "" {
		d, parseErr := time.ParseDuration(timeoutStr)
		if parseErr != nil {
			return fmt.Errorf("exec node %q: invalid timeout %q: %w", node.ID, timeoutStr, parseErr)
		}
		if d > 0 {
			var cancel context.CancelFunc
			runCtx, cancel = context.WithTimeout(ctx, d)
			defer cancel()
		}
	}

	// Build command.
	cmd := exec.CommandContext(runCtx, "/bin/sh", "-c", renderedCmd)
	if workdir != "" {
		cmd.Dir = workdir
	}
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	runErr := cmd.Run()
	stdout := stdoutBuf.String()
	stderr := stderrBuf.String()

	// Determine exit code.
	exitCode := 0
	if runErr != nil {
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			// Context deadline or signal — non-numeric sentinel.
			exitCode = -1
		}
	}

	// Store stdout.
	stdoutKey := node.Attrs["stdout_key"]
	if stdoutKey == "" {
		stdoutKey = node.ID + "_stdout"
	}
	pctx.Set(stdoutKey, stdout)
	pctx.Set("last_output", stdout)

	// Store stderr if requested.
	if sk := node.Attrs["stderr_key"]; sk != "" {
		pctx.Set(sk, stderr)
	}

	// Store exit code if requested.
	if ek := node.Attrs["exit_code_key"]; ek != "" {
		pctx.Set(ek, strconv.Itoa(exitCode))
	}

	// Fail on non-zero exit unless suppressed.
	if exitCode != 0 && node.Attrs["fail_on_error"] != "false" {
		msg := fmt.Sprintf("exec node %q: command exited with code %d", node.ID, exitCode)
		if firstLine := strings.SplitN(strings.TrimSpace(stderr), "\n", 2)[0]; firstLine != "" {
			msg += ": " + firstLine
		}
		return fmt.Errorf("%s", msg)
	}

	return nil
}
