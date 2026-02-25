package pipeline_test

import (
	"context"
	"testing"

	"github.com/ravi-parthasarathy/attractor/pkg/pipeline"
)

// switchPipeline builds a pipeline with a switch node routing to three
// branches (ok, warn, error) plus a default (_) branch, all converging
// on a single exit node.
func switchPipeline() *pipeline.Pipeline {
	return &pipeline.Pipeline{
		Name: "switch_test",
		Nodes: map[string]*pipeline.Node{
			"s":    {ID: "s", Type: pipeline.NodeTypeStart},
			"r":    {ID: "r", Type: pipeline.NodeTypeSwitch, Attrs: map[string]string{"key": "status"}},
			"ok":   {ID: "ok", Type: pipeline.NodeTypeSet, Attrs: map[string]string{"key": "result", "value": "ok_branch"}},
			"warn": {ID: "warn", Type: pipeline.NodeTypeSet, Attrs: map[string]string{"key": "result", "value": "warn_branch"}},
			"err":  {ID: "err", Type: pipeline.NodeTypeSet, Attrs: map[string]string{"key": "result", "value": "err_branch"}},
			"def":  {ID: "def", Type: pipeline.NodeTypeSet, Attrs: map[string]string{"key": "result", "value": "default_branch"}},
			"e":    {ID: "e", Type: pipeline.NodeTypeExit},
		},
		Edges: []*pipeline.Edge{
			{From: "s", To: "r"},
			{From: "r", To: "ok", Condition: "ok"},
			{From: "r", To: "warn", Condition: "warn"},
			{From: "r", To: "err", Condition: "error"},
			{From: "r", To: "def", Condition: "_"},
			{From: "ok", To: "e"},
			{From: "warn", To: "e"},
			{From: "err", To: "e"},
			{From: "def", To: "e"},
		},
	}
}

func runSwitchPipeline(t *testing.T, status string) string {
	t.Helper()
	p := switchPipeline()
	pctx := pipeline.NewPipelineContext()
	pctx.Set("status", status)

	reg := &stubRegistry{handlers: map[pipeline.NodeType]pipeline.Handler{
		pipeline.NodeTypeStart:  &countingHandler{},
		pipeline.NodeTypeSwitch: &noopHandler{},
		pipeline.NodeTypeSet:    &setHandler{},
		pipeline.NodeTypeExit:   &exitHandler{},
	}}

	eng, err := pipeline.NewEngine(p, reg, pctx, "")
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	if err := eng.Execute(context.Background(), ""); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	return pctx.GetString("result")
}

// noopHandler is a handler that does nothing (used for switch nodes in tests).
type noopHandler struct{}

func (h *noopHandler) Handle(_ context.Context, _ *pipeline.Node, _ *pipeline.PipelineContext) error {
	return nil
}

// setHandler executes a set node: reads key/value attrs and sets context.
type setHandler struct{}

func (h *setHandler) Handle(_ context.Context, node *pipeline.Node, pctx *pipeline.PipelineContext) error {
	pctx.Set(node.Attrs["key"], node.Attrs["value"])
	return nil
}

func TestSwitchRouteMatch(t *testing.T) {
	t.Parallel()
	if got := runSwitchPipeline(t, "ok"); got != "ok_branch" {
		t.Errorf("status=ok: got %q, want %q", got, "ok_branch")
	}
	if got := runSwitchPipeline(t, "warn"); got != "warn_branch" {
		t.Errorf("status=warn: got %q, want %q", got, "warn_branch")
	}
	if got := runSwitchPipeline(t, "error"); got != "err_branch" {
		t.Errorf("status=error: got %q, want %q", got, "err_branch")
	}
}

func TestSwitchRouteDefault(t *testing.T) {
	t.Parallel()
	// "unknown" doesn't match any labeled edge → falls to "_" default.
	if got := runSwitchPipeline(t, "unknown"); got != "default_branch" {
		t.Errorf("status=unknown: got %q, want %q", got, "default_branch")
	}
}

func TestSwitchNoDefault(t *testing.T) {
	t.Parallel()
	// Pipeline with no default edge — unmatched value should error.
	p := &pipeline.Pipeline{
		Name: "no_default",
		Nodes: map[string]*pipeline.Node{
			"s":  {ID: "s", Type: pipeline.NodeTypeStart},
			"r":  {ID: "r", Type: pipeline.NodeTypeSwitch, Attrs: map[string]string{"key": "v"}},
			"ok": {ID: "ok", Type: pipeline.NodeTypeSet, Attrs: map[string]string{"key": "r", "value": "ok"}},
			"e":  {ID: "e", Type: pipeline.NodeTypeExit},
		},
		Edges: []*pipeline.Edge{
			{From: "s", To: "r"},
			{From: "r", To: "ok", Condition: "ok"},
			{From: "ok", To: "e"},
		},
	}
	pctx := pipeline.NewPipelineContext()
	pctx.Set("v", "nope") // won't match "ok"

	reg := &stubRegistry{handlers: map[pipeline.NodeType]pipeline.Handler{
		pipeline.NodeTypeStart:  &countingHandler{},
		pipeline.NodeTypeSwitch: &noopHandler{},
		pipeline.NodeTypeSet:    &setHandler{},
		pipeline.NodeTypeExit:   &exitHandler{},
	}}

	eng, err := pipeline.NewEngine(p, reg, pctx, "")
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	if err := eng.Execute(context.Background(), ""); err == nil {
		t.Fatal("expected error for unmatched switch with no default, got nil")
	}
}
