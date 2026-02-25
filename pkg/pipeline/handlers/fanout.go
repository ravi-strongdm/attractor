package handlers

import (
	"context"
	"fmt"
	"sync"

	"github.com/ravi-parthasarathy/attractor/pkg/pipeline"
)

// FanOutHandler marks the fan-out point in the context.
// Actual parallel execution is coordinated by the engine.
type FanOutHandler struct{}

func (h *FanOutHandler) Handle(_ context.Context, node *pipeline.Node, pctx *pipeline.PipelineContext) error {
	pctx.Set(node.ID+"_fanout", "started")
	return nil
}

// RunParallel executes handlerFn for each branchID in parallel,
// then merges all returned context snapshots (last-write-wins per key).
func RunParallel(
	ctx context.Context,
	branchIDs []string,
	handlerFn func(ctx context.Context, branchID string) (map[string]any, error),
) (map[string]any, error) {
	type result struct {
		data map[string]any
		err  error
	}
	results := make([]result, len(branchIDs))
	var wg sync.WaitGroup
	for i, id := range branchIDs {
		wg.Add(1)
		go func(idx int, branchID string) {
			defer wg.Done()
			data, err := handlerFn(ctx, branchID)
			results[idx] = result{data: data, err: err}
		}(i, id)
	}
	wg.Wait()

	merged := make(map[string]any)
	var errs []error
	for i, r := range results {
		if r.err != nil {
			errs = append(errs, fmt.Errorf("branch %q: %w", branchIDs[i], r.err))
			continue
		}
		for k, v := range r.data {
			merged[k] = v
		}
	}
	if len(errs) > 0 {
		return merged, fmt.Errorf("parallel branches failed: %v", errs)
	}
	return merged, nil
}
