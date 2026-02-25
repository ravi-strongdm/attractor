package handlers

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/ravi-parthasarathy/attractor/pkg/pipeline"
)

const defaultHTTPTimeout = 30 * time.Second

// HTTPHandler makes an HTTP request and stores the response body and status
// code in the pipeline context.
type HTTPHandler struct{}

func (h *HTTPHandler) Handle(ctx context.Context, node *pipeline.Node, pctx *pipeline.PipelineContext) error {
	snap := pctx.Snapshot()

	// Required: url
	urlTpl := node.Attrs["url"]
	if urlTpl == "" {
		return fmt.Errorf("http node %q: missing required 'url' attribute", node.ID)
	}
	urlStr, err := renderTemplate(urlTpl, snap)
	if err != nil {
		return fmt.Errorf("http node %q: url template: %w", node.ID, err)
	}

	method := node.Attrs["method"]
	if method == "" {
		method = "GET"
	}
	method = strings.ToUpper(method)

	// Optional body (template-rendered)
	var bodyReader io.Reader
	if bodyTpl := node.Attrs["body"]; bodyTpl != "" {
		bodyStr, err := renderTemplate(bodyTpl, snap)
		if err != nil {
			return fmt.Errorf("http node %q: body template: %w", node.ID, err)
		}
		bodyReader = strings.NewReader(bodyStr)
	}

	// Timeout
	timeout := defaultHTTPTimeout
	if ts := node.Attrs["timeout"]; ts != "" {
		if d, err := time.ParseDuration(ts); err == nil {
			timeout = d
		}
	}

	// Context with timeout
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, method, urlStr, bodyReader)
	if err != nil {
		return fmt.Errorf("http node %q: build request: %w", node.ID, err)
	}

	// Optional headers: semicolon-separated Key:Value pairs
	if headersTpl := node.Attrs["headers"]; headersTpl != "" {
		headersStr, err := renderTemplate(headersTpl, snap)
		if err != nil {
			return fmt.Errorf("http node %q: headers template: %w", node.ID, err)
		}
		for _, pair := range strings.Split(headersStr, ";") {
			pair = strings.TrimSpace(pair)
			if pair == "" {
				continue
			}
			idx := strings.IndexByte(pair, ':')
			if idx < 0 {
				return fmt.Errorf("http node %q: header %q missing ':' separator", node.ID, pair)
			}
			req.Header.Set(strings.TrimSpace(pair[:idx]), strings.TrimSpace(pair[idx+1:]))
		}
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("http node %q: request failed: %w", node.ID, err)
	}
	defer func() { _ = resp.Body.Close() }()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("http node %q: read response body: %w", node.ID, err)
	}

	// Store results in context
	responseKey := node.Attrs["response_key"]
	if responseKey == "" {
		responseKey = node.ID + "_body"
	}
	statusKey := node.Attrs["status_key"]
	if statusKey == "" {
		statusKey = node.ID + "_status"
	}

	pctx.Set(responseKey, string(bodyBytes))
	pctx.Set(statusKey, fmt.Sprintf("%d", resp.StatusCode))

	// Optionally fail on non-2xx
	if node.Attrs["fail_non2xx"] == "true" && (resp.StatusCode < 200 || resp.StatusCode >= 300) {
		return fmt.Errorf("http node %q: non-2xx status %d", node.ID, resp.StatusCode)
	}

	return nil
}
