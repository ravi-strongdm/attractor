package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/ravi-parthasarathy/attractor/pkg/pipeline"
)

// ForEachHandler iterates sequentially over a JSON array, running a shell
// command template once per element and collecting stdout into a results array.
type ForEachHandler struct {
	Workdir string
}

func (h *ForEachHandler) Handle(ctx context.Context, node *pipeline.Node, pctx *pipeline.PipelineContext) error {
	itemsKey := node.Attrs["items"]
	if itemsKey == "" {
		return fmt.Errorf("for_each node %q: missing 'items' attribute", node.ID)
	}
	itemKey := node.Attrs["item_key"]
	if itemKey == "" {
		return fmt.Errorf("for_each node %q: missing 'item_key' attribute", node.ID)
	}
	cmdTpl := node.Attrs["cmd"]
	if cmdTpl == "" {
		return fmt.Errorf("for_each node %q: missing 'cmd' attribute", node.ID)
	}

	resultsKey := node.Attrs["results_key"]
	if resultsKey == "" {
		resultsKey = node.ID + "_results"
	}

	// Parse the items array.
	raw := pctx.GetString(itemsKey)
	if raw == "" {
		pctx.Set(resultsKey, "[]")
		pctx.Set("last_output", "[]")
		return nil
	}
	var items []any
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return fmt.Errorf("for_each node %q: invalid JSON in items key %q: %w", node.ID, itemsKey, err)
	}
	if len(items) == 0 {
		pctx.Set(resultsKey, "[]")
		pctx.Set("last_output", "[]")
		return nil
	}

	// Resolve working directory.
	workdir := h.Workdir
	if wdTpl := node.Attrs["workdir"]; wdTpl != "" {
		wd, wdErr := renderTemplate(wdTpl, pctx.Snapshot())
		if wdErr != nil {
			return fmt.Errorf("for_each node %q: workdir template error: %w", node.ID, wdErr)
		}
		workdir = wd
	}

	// Parse per-item timeout.
	var itemTimeout time.Duration
	if ts := node.Attrs["timeout"]; ts != "" {
		d, parseErr := time.ParseDuration(ts)
		if parseErr != nil {
			return fmt.Errorf("for_each node %q: invalid timeout %q: %w", node.ID, ts, parseErr)
		}
		itemTimeout = d
	}

	results := make([]string, len(items))

	for i, item := range items {
		// Branch context: copy parent and set item_key.
		branch := pctx.Copy()
		branch.Set(itemKey, fmt.Sprintf("%v", item))

		// Render command template.
		renderedCmd, err := renderTemplate(cmdTpl, branch.Snapshot())
		if err != nil {
			return fmt.Errorf("for_each node %q: item %d cmd template error: %w", node.ID, i, err)
		}

		// Build command.
		runCtx := ctx
		var cancel context.CancelFunc
		if itemTimeout > 0 {
			runCtx, cancel = context.WithTimeout(ctx, itemTimeout)
		}

		cmd := exec.CommandContext(runCtx, "/bin/sh", "-c", renderedCmd)
		if workdir != "" {
			cmd.Dir = workdir
		}
		var stdoutBuf, stderrBuf bytes.Buffer
		cmd.Stdout = &stdoutBuf
		cmd.Stderr = &stderrBuf

		runErr := cmd.Run()
		if cancel != nil {
			cancel()
		}

		stdout := stdoutBuf.String()
		results[i] = stdout

		if runErr != nil {
			if node.Attrs["fail_on_error"] == "false" {
				continue
			}
			exitCode := -1
			if exitErr, ok := runErr.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			}
			msg := fmt.Sprintf("for_each node %q: item %d exited with code %d", node.ID, i, exitCode)
			if firstLine := strings.SplitN(strings.TrimSpace(stderrBuf.String()), "\n", 2)[0]; firstLine != "" {
				msg += ": " + firstLine
			}
			return fmt.Errorf("%s", msg)
		}
	}

	// Marshal results array.
	data, err := json.Marshal(results)
	if err != nil {
		return fmt.Errorf("for_each node %q: marshal results: %w", node.ID, err)
	}
	resultsJSON := string(data)
	pctx.Set(resultsKey, resultsJSON)
	pctx.Set("last_output", resultsJSON)
	// Store count for convenience.
	pctx.Set(node.ID+"_count", strconv.Itoa(len(results)))
	return nil
}
