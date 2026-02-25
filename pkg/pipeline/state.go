package pipeline

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

// PipelineContext is a thread-safe key-value store for pipeline state.
type PipelineContext struct {
	mu   sync.RWMutex
	data map[string]any
}

// NewPipelineContext creates an empty PipelineContext.
func NewPipelineContext() *PipelineContext {
	return &PipelineContext{data: make(map[string]any)}
}

// Set stores a value under key.
func (c *PipelineContext) Set(key string, value any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[key] = value
}

// Get retrieves a value by key.
func (c *PipelineContext) Get(key string) (any, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.data[key]
	return v, ok
}

// GetString retrieves a string value, returning "" if not found or not a string.
func (c *PipelineContext) GetString(key string) string {
	v, ok := c.Get(key)
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}

// Snapshot returns a shallow copy of all key-value pairs.
func (c *PipelineContext) Snapshot() map[string]any {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make(map[string]any, len(c.data))
	for k, v := range c.data {
		out[k] = v
	}
	return out
}

// Merge copies all key-value pairs from src into this context (last-write-wins).
func (c *PipelineContext) Merge(src map[string]any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for k, v := range src {
		c.data[k] = v
	}
}

// Copy returns a new PipelineContext initialised from a snapshot of this one.
// The copy is completely independent — mutations to either context do not
// affect the other.
func (c *PipelineContext) Copy() *PipelineContext {
	return &PipelineContext{data: c.Snapshot()}
}

// checkpoint is the JSON-serialisable form of a saved checkpoint.
type checkpoint struct {
	LastNodeID string         `json:"last_node_id"`
	Data       map[string]any `json:"data"`
}

// SaveCheckpoint persists the context + last completed node ID to a JSON file.
func (c *PipelineContext) SaveCheckpoint(path, lastNodeID string) error {
	c.mu.RLock()
	snap := make(map[string]any, len(c.data))
	for k, v := range c.data {
		snap[k] = v
	}
	c.mu.RUnlock()

	cp := checkpoint{LastNodeID: lastNodeID, Data: snap}
	data, err := json.MarshalIndent(cp, "", "  ")
	if err != nil {
		return fmt.Errorf("checkpoint marshal: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("checkpoint write: %w", err)
	}
	return nil
}

// LoadCheckpoint restores a context from a JSON checkpoint file.
// Returns the context and the last completed node ID.
func LoadCheckpoint(path string) (*PipelineContext, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", fmt.Errorf("checkpoint read: %w", err)
	}
	var cp checkpoint
	if err := json.Unmarshal(data, &cp); err != nil {
		return nil, "", fmt.Errorf("checkpoint unmarshal: %w", err)
	}
	ctx := &PipelineContext{data: cp.Data}
	return ctx, cp.LastNodeID, nil
}
