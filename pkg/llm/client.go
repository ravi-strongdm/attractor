package llm

import (
	"context"
	"fmt"
	"sync"
)

// Client is the provider-agnostic LLM interface.
type Client interface {
	// Complete performs a blocking generation and returns the full response.
	Complete(ctx context.Context, req GenerateRequest) (GenerateResponse, error)
	// Stream starts streaming generation; events are sent on the returned channel.
	// The channel is closed when generation completes or an error occurs.
	Stream(ctx context.Context, req GenerateRequest) (<-chan StreamEvent, error)
}

// ProviderFactory creates a Client for a given model name within a provider.
type ProviderFactory func(modelName string) (Client, error)

var (
	registryMu sync.RWMutex
	registry   = map[string]ProviderFactory{}
)

// RegisterProvider registers a factory function for a named provider.
// Call this from init() in provider packages.
func RegisterProvider(name string, factory ProviderFactory) {
	registryMu.Lock()
	defer registryMu.Unlock()
	registry[name] = factory
}

// NewClient constructs a Client for the given model ID.
// Model IDs use the form "provider:model-name". If no provider prefix is given,
// "anthropic" is assumed.
func NewClient(modelID string) (Client, error) {
	provider, modelName, err := ParseModelID(modelID)
	if err != nil {
		return nil, fmt.Errorf("NewClient: %w", err)
	}
	registryMu.RLock()
	factory, ok := registry[provider]
	registryMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("no provider registered for %q (model ID %q) — did you import the provider package?", provider, modelID)
	}
	return factory(modelName)
}
