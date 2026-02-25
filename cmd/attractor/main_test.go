package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ravi-parthasarathy/attractor/pkg/pipeline"
)

// ─── TestWriteOutputContext ───────────────────────────────────────────────────

func TestWriteOutputContext_WritesJSON(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "ctx.json")

	pctx := pipeline.NewPipelineContext()
	pctx.Set("greeting", "hello")
	pctx.Set("count", "42")

	if err := writeOutputContext(out, pctx); err != nil {
		t.Fatalf("writeOutputContext: %v", err)
	}

	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read output file: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got["greeting"] != "hello" {
		t.Errorf("greeting = %v, want hello", got["greeting"])
	}
	if got["count"] != "42" {
		t.Errorf("count = %v, want 42", got["count"])
	}
}

func TestWriteOutputContext_NoOp(t *testing.T) {
	// An empty path must be a no-op with no error.
	pctx := pipeline.NewPipelineContext()
	if err := writeOutputContext("", pctx); err != nil {
		t.Fatalf("expected no error for empty path, got: %v", err)
	}
}

func TestWriteOutputContext_BadPath(t *testing.T) {
	// Writing to a non-existent directory should return an error.
	pctx := pipeline.NewPipelineContext()
	err := writeOutputContext("/nonexistent/dir/ctx.json", pctx)
	if err == nil {
		t.Fatal("expected error writing to bad path")
	}
}

// ─── TestInitLogger ───────────────────────────────────────────────────────────

func TestInitLogger_ValidLevels(t *testing.T) {
	for _, lvl := range []string{"debug", "info", "warn", "error", "DEBUG", "INFO"} {
		if err := initLogger(lvl, "text"); err != nil {
			t.Errorf("initLogger(%q, text): unexpected error: %v", lvl, err)
		}
	}
}

func TestInitLogger_ValidFormats(t *testing.T) {
	for _, fmt := range []string{"text", "json", "TEXT", "JSON"} {
		if err := initLogger("info", fmt); err != nil {
			t.Errorf("initLogger(info, %q): unexpected error: %v", fmt, err)
		}
	}
}

func TestInitLogger_InvalidLevel(t *testing.T) {
	if err := initLogger("verbose", "text"); err == nil {
		t.Fatal("expected error for unknown log level")
	}
}

func TestInitLogger_InvalidFormat(t *testing.T) {
	if err := initLogger("info", "xml"); err == nil {
		t.Fatal("expected error for unknown log format")
	}
}

// ─── TestApplyVarFile ─────────────────────────────────────────────────────────

func TestVarFileBasic(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	f := filepath.Join(dir, "vars.json")
	if err := os.WriteFile(f, []byte(`{"model":"gpt-4","limit":"10"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	pctx := pipeline.NewPipelineContext()
	if err := applyVarFile(pctx, f); err != nil {
		t.Fatalf("applyVarFile: %v", err)
	}
	if got := pctx.GetString("model"); got != "gpt-4" {
		t.Errorf("model = %q, want %q", got, "gpt-4")
	}
	if got := pctx.GetString("limit"); got != "10" {
		t.Errorf("limit = %q, want %q", got, "10")
	}
}

func TestVarFileOverriddenByVar(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	f := filepath.Join(dir, "vars.json")
	if err := os.WriteFile(f, []byte(`{"model":"gpt-4","debug":"false"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	pctx := pipeline.NewPipelineContext()
	if err := applyVarFile(pctx, f); err != nil {
		t.Fatalf("applyVarFile: %v", err)
	}
	// --var override wins.
	if err := applyVars(pctx, []string{"model=claude-sonnet"}); err != nil {
		t.Fatalf("applyVars: %v", err)
	}
	if got := pctx.GetString("model"); got != "claude-sonnet" {
		t.Errorf("model = %q, want %q", got, "claude-sonnet")
	}
	// Non-overridden key still present.
	if got := pctx.GetString("debug"); got != "false" {
		t.Errorf("debug = %q, want %q", got, "false")
	}
}

func TestVarFileMissing(t *testing.T) {
	t.Parallel()
	pctx := pipeline.NewPipelineContext()
	err := applyVarFile(pctx, "/nonexistent/path/vars.json")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if !strings.Contains(err.Error(), "--var-file") {
		t.Errorf("error should mention --var-file: %v", err)
	}
}

func TestVarFileNonObject(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	f := filepath.Join(dir, "array.json")
	if err := os.WriteFile(f, []byte(`["a","b","c"]`), 0o644); err != nil {
		t.Fatal(err)
	}
	pctx := pipeline.NewPipelineContext()
	err := applyVarFile(pctx, f)
	if err == nil {
		t.Fatal("expected error for JSON array at top level")
	}
	if !strings.Contains(err.Error(), "JSON object") {
		t.Errorf("error should mention JSON object: %v", err)
	}
}

func TestVarFileNoOp(t *testing.T) {
	t.Parallel()
	pctx := pipeline.NewPipelineContext()
	if err := applyVarFile(pctx, ""); err != nil {
		t.Fatalf("expected no-op for empty path, got: %v", err)
	}
}

// ─── TestGraph ────────────────────────────────────────────────────────────────

const batchDOT = `digraph batch {
    start [type=start]
    load  [type=env key=topics_raw from=TOPICS required=true]
    done  [type=exit]
    start -> load
    load  -> done
}`

func TestGraphTextOutput(t *testing.T) {
	t.Parallel()
	p, err := pipeline.ParseDOT(batchDOT)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	out := renderText(p)

	// Should contain pipeline name.
	if !strings.Contains(out, "batch") {
		t.Errorf("output missing pipeline name 'batch': %s", out)
	}
	// Should contain node IDs.
	for _, id := range []string{"start", "load", "done"} {
		if !strings.Contains(out, id) {
			t.Errorf("output missing node %q: %s", id, out)
		}
	}
	// Should contain node types.
	for _, typ := range []string{"start", "env", "exit"} {
		if !strings.Contains(out, typ) {
			t.Errorf("output missing type %q: %s", typ, out)
		}
	}
	// Should have Nodes and Edges sections.
	if !strings.Contains(out, "Nodes:") {
		t.Error("output missing 'Nodes:' header")
	}
	if !strings.Contains(out, "Edges:") {
		t.Error("output missing 'Edges:' header")
	}
}

func TestGraphDOTRoundtrip(t *testing.T) {
	t.Parallel()
	p, err := pipeline.ParseDOT(batchDOT)
	if err != nil {
		t.Fatalf("parse original: %v", err)
	}
	dotOut := renderDOT(p)

	// Re-parse the emitted DOT.
	p2, err := pipeline.ParseDOT(dotOut)
	if err != nil {
		t.Fatalf("re-parse DOT output: %v\nDOT:\n%s", err, dotOut)
	}

	// Validate re-parsed pipeline — should have no lint errors.
	if lintErr := pipeline.ValidateErr(p2); lintErr != nil {
		t.Errorf("re-parsed pipeline has lint errors: %v\nDOT:\n%s", lintErr, dotOut)
	}

	// Node count must match.
	if len(p2.Nodes) != len(p.Nodes) {
		t.Errorf("node count: got %d, want %d", len(p2.Nodes), len(p.Nodes))
	}
	// Edge count must match.
	if len(p2.Edges) != len(p.Edges) {
		t.Errorf("edge count: got %d, want %d", len(p2.Edges), len(p.Edges))
	}
}

func TestGraphTextTruncation(t *testing.T) {
	t.Parallel()
	longVal := strings.Repeat("x", 80)
	dot := `digraph trunc {
    start [type=start]
    n     [type=set key=foo value=` + longVal + `]
    done  [type=exit]
    start -> n
    n -> done
}`
	p, err := pipeline.ParseDOT(dot)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	out := renderText(p)
	// Truncated value should appear (first 60 chars + ellipsis).
	truncated := longVal[:60] + "…"
	if !strings.Contains(out, truncated) {
		t.Errorf("expected truncated value %q in output:\n%s", truncated, out)
	}
}

func TestGraphDOTConditionEdges(t *testing.T) {
	t.Parallel()
	dot := `digraph sw {
    start  [type=start]
    branch [type=switch key=mode]
    a      [type=set key=r value=a]
    b      [type=set key=r value=b]
    done   [type=exit]
    start  -> branch
    branch -> a [label=fast]
    branch -> b [label=slow]
    a -> done
    b -> done
}`
	p, err := pipeline.ParseDOT(dot)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	out := renderDOT(p)

	// Conditions should appear as label attributes in DOT output.
	if !strings.Contains(out, "label=fast") && !strings.Contains(out, `label="fast"`) {
		t.Errorf("DOT output missing label=fast:\n%s", out)
	}
}
