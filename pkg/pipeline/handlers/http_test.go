package handlers_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ravi-parthasarathy/attractor/pkg/pipeline"
	"github.com/ravi-parthasarathy/attractor/pkg/pipeline/handlers"
)

func newHTTPNode(id string, attrs map[string]string) *pipeline.Node {
	return &pipeline.Node{
		ID:    id,
		Type:  pipeline.NodeTypeHTTP,
		Attrs: attrs,
	}
}

func TestHTTPNodeGet(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		_, _ = fmt.Fprint(w, `{"ok":true}`)
	}))
	defer srv.Close()

	pctx := pipeline.NewPipelineContext()
	node := newHTTPNode("fetch", map[string]string{
		"url": srv.URL + "/data",
	})

	h := &handlers.HTTPHandler{}
	if err := h.Handle(t.Context(), node, pctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := pctx.GetString("fetch_body"); got != `{"ok":true}` {
		t.Errorf("response_key: got %q, want %q", got, `{"ok":true}`)
	}
	if got := pctx.GetString("fetch_status"); got != "200" {
		t.Errorf("status_key: got %q, want %q", got, "200")
	}
}

func TestHTTPNodePost(t *testing.T) {
	t.Parallel()
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		buf := make([]byte, 512)
		n, _ := r.Body.Read(buf)
		gotBody = string(buf[:n])
		_, _ = fmt.Fprint(w, "created")
	}))
	defer srv.Close()

	pctx := pipeline.NewPipelineContext()
	pctx.Set("name", "world")
	node := newHTTPNode("post", map[string]string{
		"url":          srv.URL + "/items",
		"method":       "POST",
		"body":         `{"hello":"{{.name}}"}`,
		"response_key": "post_result",
	})

	h := &handlers.HTTPHandler{}
	if err := h.Handle(t.Context(), node, pctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotBody != `{"hello":"world"}` {
		t.Errorf("request body: got %q, want %q", gotBody, `{"hello":"world"}`)
	}
	if got := pctx.GetString("post_result"); got != "created" {
		t.Errorf("response_key: got %q, want %q", got, "created")
	}
}

func TestHTTPNodeHeaders(t *testing.T) {
	t.Parallel()
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_, _ = fmt.Fprint(w, "ok")
	}))
	defer srv.Close()

	pctx := pipeline.NewPipelineContext()
	pctx.Set("token", "secret123")
	node := newHTTPNode("hdr", map[string]string{
		"url":     srv.URL,
		"headers": "Authorization:Bearer {{.token}}",
	})

	h := &handlers.HTTPHandler{}
	if err := h.Handle(t.Context(), node, pctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotAuth != "Bearer secret123" {
		t.Errorf("Authorization header: got %q, want %q", gotAuth, "Bearer secret123")
	}
}

func TestHTTPNodeTimeout(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		_, _ = fmt.Fprint(w, "too late")
	}))
	defer srv.Close()

	pctx := pipeline.NewPipelineContext()
	node := newHTTPNode("slow", map[string]string{
		"url":     srv.URL,
		"timeout": "50ms",
	})

	h := &handlers.HTTPHandler{}
	err := h.Handle(t.Context(), node, pctx)
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
}

func TestHTTPNodeFail2xx(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer srv.Close()

	pctx := pipeline.NewPipelineContext()
	node := newHTTPNode("err", map[string]string{
		"url":         srv.URL,
		"fail_non2xx": "true",
	})

	h := &handlers.HTTPHandler{}
	if err := h.Handle(t.Context(), node, pctx); err == nil {
		t.Fatal("expected error for non-2xx, got nil")
	}
}

func TestHTTPNodeAllow2xx(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer srv.Close()

	pctx := pipeline.NewPipelineContext()
	node := newHTTPNode("allow", map[string]string{
		"url": srv.URL,
		// fail_non2xx defaults to false
	})

	h := &handlers.HTTPHandler{}
	if err := h.Handle(t.Context(), node, pctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := pctx.GetString("allow_status"); got != "404" {
		t.Errorf("status_key: got %q, want %q", got, "404")
	}
}

func TestHTTPNodeMissingURL(t *testing.T) {
	t.Parallel()
	pctx := pipeline.NewPipelineContext()
	node := newHTTPNode("bad", map[string]string{})

	h := &handlers.HTTPHandler{}
	if err := h.Handle(t.Context(), node, pctx); err == nil {
		t.Fatal("expected error for missing url, got nil")
	}
}
