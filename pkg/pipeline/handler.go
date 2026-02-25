package pipeline

import "context"

// Handler executes a pipeline node.
// Implementations live in the handlers sub-package; this interface is defined
// here so that Engine can use it without creating an import cycle.
type Handler interface {
	// Handle executes the node and may mutate pctx.
	// Return an ExitSignal error to terminate the pipeline normally.
	Handle(ctx context.Context, node *Node, pctx *PipelineContext) error
}

// HandlerRegistry looks up Handler implementations by node type.
type HandlerRegistry interface {
	Get(nodeType NodeType) (Handler, error)
}

// ExitSignal is returned by exit handlers to signal normal pipeline completion.
// Placing it in this package avoids a handlers→pipeline→handlers import cycle.
type ExitSignal struct{}

func (ExitSignal) Error() string { return "pipeline exit" }
