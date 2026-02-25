package pipeline_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/ravi-parthasarathy/attractor/pkg/pipeline"
	"github.com/ravi-parthasarathy/attractor/pkg/pipeline/handlers"
)

// ─── Parser tests ─────────────────────────────────────────────────────────────

func TestParseDOT_MinimalPipeline(t *testing.T) {
	src := `digraph test {
		start  [type=start]
		finish [type=exit]
		start -> finish
	}`
	p, err := pipeline.ParseDOT(src)
	if err != nil {
		t.Fatalf("ParseDOT: %v", err)
	}
	if len(p.Nodes) != 2 {
		t.Errorf("nodes = %d, want 2", len(p.Nodes))
	}
	if len(p.Edges) != 1 {
		t.Errorf("edges = %d, want 1", len(p.Edges))
	}
}

func TestParseDOT_NodeAttrs(t *testing.T) {
	src := `digraph test {
		start  [type=start]
		s      [type=set, key="greeting", value="hello"]
		finish [type=exit]
		start -> s
		s -> finish
	}`
	p, err := pipeline.ParseDOT(src)
	if err != nil {
		t.Fatalf("ParseDOT: %v", err)
	}
	n := p.Nodes["s"]
	if n == nil {
		t.Fatal("node 's' not found")
	}
	if n.Attrs["key"] != "greeting" {
		t.Errorf("key = %q, want %q", n.Attrs["key"], "greeting")
	}
	if n.Attrs["value"] != "hello" {
		t.Errorf("value = %q, want %q", n.Attrs["value"], "hello")
	}
}

func TestParseDOT_EdgeCondition(t *testing.T) {
	src := `digraph test {
		start  [type=start]
		finish [type=exit]
		start -> finish [label="status == 'ok'"]
	}`
	p, err := pipeline.ParseDOT(src)
	if err != nil {
		t.Fatalf("ParseDOT: %v", err)
	}
	edges := p.OutgoingEdges("start")
	if len(edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(edges))
	}
	if edges[0].Condition != "status == 'ok'" {
		t.Errorf("condition = %q, want %q", edges[0].Condition, "status == 'ok'")
	}
}

// ─── Validator tests ──────────────────────────────────────────────────────────

func TestValidate_Valid(t *testing.T) {
	src := `digraph ok {
		s [type=start]
		e [type=exit]
		s -> e
	}`
	p, _ := pipeline.ParseDOT(src)
	if err := pipeline.ValidateErr(p); err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestValidate_NoStart(t *testing.T) {
	src := `digraph bad {
		a [type=set, key="x", value="y"]
		e [type=exit]
		a -> e
	}`
	p, _ := pipeline.ParseDOT(src)
	if err := pipeline.ValidateErr(p); err == nil {
		t.Error("expected error for missing start node")
	}
}

func TestValidate_NoExit(t *testing.T) {
	src := `digraph bad {
		s [type=start]
		a [type=set, key="x", value="y"]
		s -> a
	}`
	p, _ := pipeline.ParseDOT(src)
	if err := pipeline.ValidateErr(p); err == nil {
		t.Error("expected error for missing exit node")
	}
}

func TestValidate_UnreachableNode(t *testing.T) {
	src := `digraph bad {
		s    [type=start]
		orphan [type=set, key="x", value="y"]
		e    [type=exit]
		s -> e
	}`
	p, _ := pipeline.ParseDOT(src)
	if err := pipeline.ValidateErr(p); err == nil {
		t.Error("expected error for unreachable node")
	}
}

func TestValidate_FanOutNoFanIn(t *testing.T) {
	// fan_out with no reachable fan_in — should fail validation.
	src := `digraph bad {
		s    [type=start]
		fork [type=fan_out]
		a    [type=set, key="x", value="1"]
		b    [type=set, key="y", value="2"]
		e    [type=exit]
		s    -> fork
		fork -> a
		fork -> b
		a    -> e
		b    -> e
	}`
	p, err := pipeline.ParseDOT(src)
	if err != nil {
		t.Fatalf("ParseDOT: %v", err)
	}
	if err := pipeline.ValidateErr(p); err == nil {
		t.Error("expected lint error for fan_out with no reachable fan_in")
	}
}

func TestValidate_FanOutWithFanIn(t *testing.T) {
	// Properly paired fan_out/fan_in — should pass.
	src := `digraph ok {
		s    [type=start]
		fork [type=fan_out]
		a    [type=set, key="x", value="1"]
		b    [type=set, key="y", value="2"]
		join [type=fan_in]
		e    [type=exit]
		s    -> fork
		fork -> a
		fork -> b
		a    -> join
		b    -> join
		join -> e
	}`
	p, err := pipeline.ParseDOT(src)
	if err != nil {
		t.Fatalf("ParseDOT: %v", err)
	}
	if err := pipeline.ValidateErr(p); err != nil {
		t.Errorf("expected valid pipeline, got: %v", err)
	}
}

// ─── Condition evaluator tests ────────────────────────────────────────────────

func TestEvalCondition(t *testing.T) {
	snap := map[string]any{
		"status": "ok",
		"count":  "3",
		"flag":   "true",
	}
	tests := []struct {
		cond string
		want bool
	}{
		{"status == 'ok'", true},
		{"status == 'fail'", false},
		{"status != 'fail'", true},
		{"flag", true},
		{"!flag", false},
		{"missing", false},
		{"!missing", true},
		{"status == 'ok' && flag", true},
		{"status == 'fail' || flag", true},
		{"status == 'fail' || missing", false},
		{"(status == 'ok')", true},
	}
	for _, tt := range tests {
		t.Run(tt.cond, func(t *testing.T) {
			got, err := pipeline.EvalCondition(tt.cond, snap)
			if err != nil {
				t.Fatalf("EvalCondition(%q): %v", tt.cond, err)
			}
			if got != tt.want {
				t.Errorf("EvalCondition(%q) = %v, want %v", tt.cond, got, tt.want)
			}
		})
	}
}

func TestEvalCondition_ParseError(t *testing.T) {
	_, err := pipeline.EvalCondition("(unclosed", map[string]any{})
	if err == nil {
		t.Error("expected parse error for unclosed parenthesis")
	}
}

// ─── PipelineContext / checkpoint tests ───────────────────────────────────────

func TestPipelineContext_SetGet(t *testing.T) {
	pctx := pipeline.NewPipelineContext()
	pctx.Set("key", "value")
	if got := pctx.GetString("key"); got != "value" {
		t.Errorf("GetString = %q, want %q", got, "value")
	}
}

func TestPipelineContext_Snapshot(t *testing.T) {
	pctx := pipeline.NewPipelineContext()
	pctx.Set("a", 1)
	pctx.Set("b", "hello")
	snap := pctx.Snapshot()
	if snap["a"] != 1 {
		t.Error("snapshot missing 'a'")
	}
	if snap["b"] != "hello" {
		t.Error("snapshot missing 'b'")
	}
}

func TestPipelineContext_Checkpoint(t *testing.T) {
	dir := t.TempDir()
	cpPath := filepath.Join(dir, "checkpoint.json")

	pctx := pipeline.NewPipelineContext()
	pctx.Set("x", "42")
	pctx.Set("y", true)

	if err := pctx.SaveCheckpoint(cpPath, "node-3"); err != nil {
		t.Fatalf("SaveCheckpoint: %v", err)
	}

	pctx2, lastNode, err := pipeline.LoadCheckpoint(cpPath)
	if err != nil {
		t.Fatalf("LoadCheckpoint: %v", err)
	}
	if lastNode != "node-3" {
		t.Errorf("lastNode = %q, want %q", lastNode, "node-3")
	}
	if pctx2.GetString("x") != "42" {
		t.Errorf("x = %q, want 42", pctx2.GetString("x"))
	}
}

// ─── Engine tests (with stub handlers) ───────────────────────────────────────

type recordHandler struct {
	visited []string
}

func (h *recordHandler) Handle(_ context.Context, node *pipeline.Node, _ *pipeline.PipelineContext) error {
	h.visited = append(h.visited, node.ID)
	return nil
}

func TestEngine_SimplePath(t *testing.T) {
	src := `digraph simple {
		s [type=start]
		a [type=set, key="x", value="1"]
		e [type=exit]
		s -> a
		a -> e
	}`
	p, err := pipeline.ParseDOT(src)
	if err != nil {
		t.Fatalf("ParseDOT: %v", err)
	}

	// Use a temp checkpoint path.
	dir := t.TempDir()
	cpPath := filepath.Join(dir, "cp.json")

	rec := &recordHandler{}
	reg := handlers.NewRegistry()
	reg.Register("start", rec)
	reg.Register("set", rec)
	reg.Register("exit", &handlers.ExitHandler{})

	pctx := pipeline.NewPipelineContext()
	eng, err := pipeline.NewEngine(p, reg, pctx, cpPath)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	if err := eng.Execute(context.Background(), ""); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	// s and a should have been visited.
	if len(rec.visited) != 2 {
		t.Errorf("visited %v, want [s a]", rec.visited)
	}

	// Checkpoint file should exist.
	if _, err := os.Stat(cpPath); err != nil {
		t.Errorf("checkpoint not written: %v", err)
	}
}

func TestEngine_ConditionalEdge(t *testing.T) {
	// Two branches that both lead to a single exit node.
	src := `digraph cond {
		s    [type=start]
		good [type=set, key="result", value="good"]
		bad  [type=set, key="result", value="bad"]
		done [type=exit]
		s    -> good [label="status == 'ok'"]
		s    -> bad  [label="status != 'ok'"]
		good -> done
		bad  -> done
	}`
	p, err := pipeline.ParseDOT(src)
	if err != nil {
		t.Fatalf("ParseDOT: %v", err)
	}

	rec := &recordHandler{}
	reg := handlers.NewRegistry()
	reg.Register("start", rec)
	reg.Register("set", rec)
	reg.Register("exit", &handlers.ExitHandler{})

	pctx := pipeline.NewPipelineContext()
	pctx.Set("status", "ok")

	eng, err := pipeline.NewEngine(p, reg, pctx, "")
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	if err := eng.Execute(context.Background(), ""); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	// "ok" branch taken → good node visited
	found := false
	for _, id := range rec.visited {
		if id == "good" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'good' branch to be taken; visited: %v", rec.visited)
	}
}

func TestEngine_InvalidPipeline(t *testing.T) {
	// No start node.
	src := `digraph bad {
		a [type=set, key="x", value="y"]
		e [type=exit]
		a -> e
	}`
	p, _ := pipeline.ParseDOT(src)
	reg := handlers.NewRegistry()
	pctx := pipeline.NewPipelineContext()
	_, err := pipeline.NewEngine(p, reg, pctx, "")
	if err == nil {
		t.Error("expected error for invalid pipeline")
	}
}

func TestEngine_NilInputs(t *testing.T) {
	reg := handlers.NewRegistry()
	pctx := pipeline.NewPipelineContext()

	_, err := pipeline.NewEngine(nil, reg, pctx, "")
	if err == nil {
		t.Error("expected error for nil pipeline")
	}
	_, err = pipeline.NewEngine(&pipeline.Pipeline{Nodes: map[string]*pipeline.Node{}, Edges: []*pipeline.Edge{}}, nil, pctx, "")
	if err == nil {
		t.Error("expected error for nil registry")
	}
	_, err = pipeline.NewEngine(&pipeline.Pipeline{Nodes: map[string]*pipeline.Node{}, Edges: []*pipeline.Edge{}}, reg, nil, "")
	if err == nil {
		t.Error("expected error for nil context")
	}
}

// ─── Parallel fan-out tests ───────────────────────────────────────────────────

func TestEngine_ParallelFanOut(t *testing.T) {
	// Topology: start → fork → [analyze, summarize] → join → report → exit
	// Both branches must run and their keys must appear in the merged context.
	src := `digraph parallel {
		start     [type=start]
		fork      [type=fan_out]
		analyze   [type=set, key="analysis",  value="analysis complete"]
		summarize [type=set, key="summary",   value="summary complete"]
		join      [type=fan_in]
		report    [type=set, key="report",    value="done"]
		done      [type=exit]

		start     -> fork
		fork      -> analyze
		fork      -> summarize
		analyze   -> join
		summarize -> join
		join      -> report
		report    -> done
	}`
	p, err := pipeline.ParseDOT(src)
	if err != nil {
		t.Fatalf("ParseDOT: %v", err)
	}

	reg := handlers.NewRegistry()
	reg.Register("start", &handlers.StartHandler{})
	reg.Register("fan_out", &handlers.FanOutHandler{})
	reg.Register("set", &handlers.SetHandler{})
	reg.Register("fan_in", &handlers.FanInHandler{})
	reg.Register("exit", &handlers.ExitHandler{})

	pctx := pipeline.NewPipelineContext()
	eng, err := pipeline.NewEngine(p, reg, pctx, "")
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	if err := eng.Execute(context.Background(), ""); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	// Both branch outputs must be present in the merged context.
	if got := pctx.GetString("analysis"); got != "analysis complete" {
		t.Errorf("analysis = %q, want %q", got, "analysis complete")
	}
	if got := pctx.GetString("summary"); got != "summary complete" {
		t.Errorf("summary = %q, want %q", got, "summary complete")
	}
	// Post-fan_in node must also have run.
	if got := pctx.GetString("report"); got != "done" {
		t.Errorf("report = %q, want %q", got, "done")
	}
}

func TestPipelineContext_Copy(t *testing.T) {
	orig := pipeline.NewPipelineContext()
	orig.Set("x", "original")

	cp := orig.Copy()

	// Copy has the same values.
	if got := cp.GetString("x"); got != "original" {
		t.Errorf("copy x = %q, want %q", got, "original")
	}

	// Mutating the copy does not affect the original.
	cp.Set("x", "modified")
	if got := orig.GetString("x"); got != "original" {
		t.Errorf("original x after copy mutation = %q, want %q", got, "original")
	}

	// Mutating the original does not affect the copy.
	orig.Set("y", "new")
	if _, ok := cp.Snapshot()["y"]; ok {
		t.Error("copy should not see keys added to original after Copy()")
	}
}
