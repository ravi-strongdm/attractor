package main

import (
	"encoding/json"
	"os"
	"path/filepath"
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
