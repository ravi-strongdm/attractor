package handlers

import (
	"fmt"

	"github.com/ravi-parthasarathy/attractor/pkg/pipeline"
)

// Registry maps node types to Handler implementations.
// It implements the pipeline.HandlerRegistry interface.
type Registry struct {
	handlers map[pipeline.NodeType]pipeline.Handler
}

// NewRegistry creates an empty Registry.
func NewRegistry() *Registry {
	return &Registry{handlers: make(map[pipeline.NodeType]pipeline.Handler)}
}

// Register associates a handler with a node type.
func (r *Registry) Register(nodeType pipeline.NodeType, h pipeline.Handler) {
	r.handlers[nodeType] = h
}

// Get returns the handler for a node type, or an error if not registered.
func (r *Registry) Get(nodeType pipeline.NodeType) (pipeline.Handler, error) {
	h, ok := r.handlers[nodeType]
	if !ok {
		return nil, fmt.Errorf("no handler registered for node type %q", nodeType)
	}
	return h, nil
}
