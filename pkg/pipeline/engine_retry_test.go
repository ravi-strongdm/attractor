package pipeline_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/ravi-parthasarathy/attractor/pkg/pipeline"
)

// countingHandler fails for the first `failCount` calls, then succeeds.
type countingHandler struct {
	calls     int
	failCount int
	failErr   error
}

func (h *countingHandler) Handle(_ context.Context, _ *pipeline.Node, _ *pipeline.PipelineContext) error {
	h.calls++
	if h.calls <= h.failCount {
		return h.failErr
	}
	return nil
}

// exitHandler always returns an ExitSignal.
type exitHandler struct{}

func (h *exitHandler) Handle(_ context.Context, _ *pipeline.Node, _ *pipeline.PipelineContext) error {
	return pipeline.ExitSignal{}
}

// alwaysFailHandler always returns an error.
type alwaysFailHandler struct{}

func (h *alwaysFailHandler) Handle(_ context.Context, _ *pipeline.Node, _ *pipeline.PipelineContext) error {
	return errors.New("always fails")
}

// stubRegistry implements pipeline.HandlerRegistry for a single node type.
type stubRegistry struct {
	handlers map[pipeline.NodeType]pipeline.Handler
}

func (r *stubRegistry) Get(t pipeline.NodeType) (pipeline.Handler, error) {
	h, ok := r.handlers[t]
	if !ok {
		return nil, fmt.Errorf("no handler for %q", t)
	}
	return h, nil
}

func minimalPipeline(nodeType pipeline.NodeType, attrs map[string]string) *pipeline.Pipeline {
	p := &pipeline.Pipeline{
		Name: "test",
		Nodes: map[string]*pipeline.Node{
			"s": {ID: "s", Type: pipeline.NodeTypeStart},
			"n": {ID: "n", Type: nodeType, Attrs: attrs},
			"e": {ID: "e", Type: pipeline.NodeTypeExit},
		},
		Edges: []*pipeline.Edge{
			{From: "s", To: "n"},
			{From: "n", To: "e"},
		},
	}
	return p
}

func TestRetrySucceedsOnSecondAttempt(t *testing.T) {
	t.Parallel()
	transientErr := errors.New("transient")
	ch := &countingHandler{failCount: 1, failErr: transientErr}

	reg := &stubRegistry{handlers: map[pipeline.NodeType]pipeline.Handler{
		pipeline.NodeTypeStart: &countingHandler{},
		"work":                 ch,
		pipeline.NodeTypeExit:  &exitHandler{},
	}}

	p := minimalPipeline("work", map[string]string{
		"retry_max":   "2",
		"retry_delay": "0s",
	})
	pctx := pipeline.NewPipelineContext()
	eng, err := pipeline.NewEngine(p, reg, pctx, "")
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	if err := eng.Execute(context.Background(), ""); err != nil {
		t.Fatalf("expected success on second attempt, got: %v", err)
	}
	if ch.calls != 2 {
		t.Errorf("expected 2 handler calls, got %d", ch.calls)
	}
}

func TestRetryExhausted(t *testing.T) {
	t.Parallel()
	ch := &alwaysFailHandler{}

	reg := &stubRegistry{handlers: map[pipeline.NodeType]pipeline.Handler{
		pipeline.NodeTypeStart: &countingHandler{},
		"work":                 ch,
		pipeline.NodeTypeExit:  &exitHandler{},
	}}

	p := minimalPipeline("work", map[string]string{
		"retry_max":   "2",
		"retry_delay": "0s",
	})
	pctx := pipeline.NewPipelineContext()
	eng, _ := pipeline.NewEngine(p, reg, pctx, "")
	err := eng.Execute(context.Background(), "")
	if err == nil {
		t.Fatal("expected error after retries exhausted")
	}
	if !strings.Contains(err.Error(), "attempt") {
		t.Errorf("error should mention attempt count: %v", err)
	}
}

func TestRetryNoRetryByDefault(t *testing.T) {
	t.Parallel()
	ch := &countingHandler{failCount: 1, failErr: errors.New("fail once")}

	reg := &stubRegistry{handlers: map[pipeline.NodeType]pipeline.Handler{
		pipeline.NodeTypeStart: &countingHandler{},
		"work":                 ch,
		pipeline.NodeTypeExit:  &exitHandler{},
	}}

	// No retry_max attribute → fails immediately on first error.
	p := minimalPipeline("work", map[string]string{})
	pctx := pipeline.NewPipelineContext()
	eng, _ := pipeline.NewEngine(p, reg, pctx, "")
	if err := eng.Execute(context.Background(), ""); err == nil {
		t.Fatal("expected error without retry, got nil")
	}
	if ch.calls != 1 {
		t.Errorf("expected exactly 1 call without retry, got %d", ch.calls)
	}
}

func TestRetryExitNotRetried(t *testing.T) {
	t.Parallel()
	eh := &exitHandler{}

	reg := &stubRegistry{handlers: map[pipeline.NodeType]pipeline.Handler{
		pipeline.NodeTypeStart: &countingHandler{},
		"work":                 eh,
		pipeline.NodeTypeExit:  &exitHandler{},
	}}

	// Even with retry_max set, ExitSignal should not be retried.
	// The "w" node returns ExitSignal so the pipeline ends before reaching "e".
	// The exit node must still exist to satisfy the validator.
	p := &pipeline.Pipeline{
		Name: "test",
		Nodes: map[string]*pipeline.Node{
			"s": {ID: "s", Type: pipeline.NodeTypeStart},
			"w": {ID: "w", Type: "work", Attrs: map[string]string{
				"retry_max":   "5",
				"retry_delay": "0s",
			}},
			"e": {ID: "e", Type: pipeline.NodeTypeExit},
		},
		Edges: []*pipeline.Edge{
			{From: "s", To: "w"},
			{From: "w", To: "e"},
		},
	}
	pctx := pipeline.NewPipelineContext()
	eng, err := pipeline.NewEngine(p, reg, pctx, "")
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	// ExitSignal → pipeline completes normally (nil error), no retries.
	if err := eng.Execute(context.Background(), ""); err != nil {
		t.Fatalf("expected nil (normal exit), got: %v", err)
	}
}
