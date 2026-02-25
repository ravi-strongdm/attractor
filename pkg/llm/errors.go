package llm

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"time"
)

// LLMError is the base error type for all LLM client errors.
type LLMError struct {
	Code    int
	Message string
	Cause   error
}

func (e *LLMError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("llm error %d: %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("llm error %d: %s", e.Code, e.Message)
}

func (e *LLMError) Unwrap() error { return e.Cause }

// RateLimitError is returned when the provider rate-limits the request.
type RateLimitError struct{ LLMError }

// ServerError is returned on 5xx responses from the provider.
type ServerError struct{ LLMError }

// AuthError is returned on authentication/authorization failures.
type AuthError struct{ LLMError }

// ContextLengthError is returned when the request exceeds the model's context window.
type ContextLengthError struct{ LLMError }

// ContentFilterError is returned when the request is blocked by the provider's safety filter.
type ContentFilterError struct{ LLMError }

// Retryable returns true if the error is transient and the request may be retried.
func Retryable(err error) bool {
	var rl *RateLimitError
	var se *ServerError
	return errors.As(err, &rl) || errors.As(err, &se)
}

// WithRetry retries fn up to maxAttempts using exponential backoff with jitter.
// It respects context cancellation.
func WithRetry(ctx context.Context, maxAttempts int, fn func() error) error {
	var lastErr error
	for i := range maxAttempts {
		lastErr = fn()
		if lastErr == nil {
			return nil
		}
		if !Retryable(lastErr) {
			return lastErr
		}
		if i == maxAttempts-1 {
			break
		}
		// Exponential backoff: base 1s, max 30s, ±25% jitter
		base := time.Duration(1<<uint(i)) * time.Second
		if base > 30*time.Second {
			base = 30 * time.Second
		}
		jitter := time.Duration(rand.Float64() * 0.5 * float64(base))
		wait := base/4*3 + jitter
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(wait):
		}
	}
	return fmt.Errorf("max retries (%d) exceeded: %w", maxAttempts, lastErr)
}
