package llm_test

import (
	"testing"

	"github.com/ravi-parthasarathy/attractor/pkg/llm"
)

func TestParseModelID(t *testing.T) {
	tests := []struct {
		input        string
		wantProvider string
		wantModel    string
		wantErr      bool
	}{
		{"anthropic:claude-sonnet-4-6", "anthropic", "claude-sonnet-4-6", false},
		{"openai:gpt-4o", "openai", "gpt-4o", false},
		{"invalid", "", "", true},
		{":", "", "", true},
		{":model", "", "", true},
		{"provider:", "", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			prov, model, err := llm.ParseModelID(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseModelID(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if prov != tt.wantProvider {
				t.Errorf("provider = %q, want %q", prov, tt.wantProvider)
			}
			if model != tt.wantModel {
				t.Errorf("model = %q, want %q", model, tt.wantModel)
			}
		})
	}
}

func TestNewClient_UnknownProvider(t *testing.T) {
	_, err := llm.NewClient("unknown_provider:some-model")
	if err == nil {
		t.Fatal("expected error for unknown provider, got nil")
	}
}

func TestRetryable(t *testing.T) {
	base := func(msg string) llm.LLMError { return llm.LLMError{Message: msg} }
	tests := []struct {
		err      error
		wantTrue bool
	}{
		{&llm.RateLimitError{LLMError: base("rate limit")}, true},
		{&llm.ServerError{LLMError: base("5xx")}, true},
		{&llm.AuthError{LLMError: base("auth")}, false},
		{&llm.ContextLengthError{LLMError: base("ctx")}, false},
		{&llm.ContentFilterError{LLMError: base("filter")}, false},
	}
	for _, tt := range tests {
		got := llm.Retryable(tt.err)
		if got != tt.wantTrue {
			t.Errorf("Retryable(%T) = %v, want %v", tt.err, got, tt.wantTrue)
		}
	}
}
