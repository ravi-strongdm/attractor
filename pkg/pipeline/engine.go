package pipeline

import (
	"context"
	"errors"
	"fmt"
)

const maxNodeVisits = 50

// Engine executes a Pipeline graph using a HandlerRegistry.
type Engine struct {
	pipeline       *Pipeline
	handlerReg     HandlerRegistry
	pctx           *PipelineContext
	checkpointPath string
}

// NewEngine creates an Engine after validating the pipeline.
func NewEngine(
	p *Pipeline,
	reg HandlerRegistry,
	pctx *PipelineContext,
	checkpointPath string,
) (*Engine, error) {
	if p == nil {
		return nil, fmt.Errorf("pipeline must not be nil")
	}
	if reg == nil {
		return nil, fmt.Errorf("handler registry must not be nil")
	}
	if pctx == nil {
		return nil, fmt.Errorf("pipeline context must not be nil")
	}
	if err := ValidateErr(p); err != nil {
		return nil, err
	}
	return &Engine{
		pipeline:       p,
		handlerReg:     reg,
		pctx:           pctx,
		checkpointPath: checkpointPath,
	}, nil
}

// Execute runs the pipeline starting from the start node, or from
// resumeFromNodeID if non-empty (for checkpoint resume).
func (e *Engine) Execute(ctx context.Context, resumeFromNodeID string) error {
	startID := resumeFromNodeID
	if startID == "" {
		startID = e.startNode()
	}
	if startID == "" {
		return fmt.Errorf("no start node found in pipeline")
	}

	// Visit counter for cycle detection.
	visits := make(map[string]int)

	currentID := startID
	for {
		// Cycle detection.
		visits[currentID]++
		if visits[currentID] > maxNodeVisits {
			return fmt.Errorf("cycle detected: node %q visited more than %d times", currentID, maxNodeVisits)
		}

		node, ok := e.pipeline.Nodes[currentID]
		if !ok {
			return fmt.Errorf("node %q not found in pipeline", currentID)
		}

		handler, err := e.handlerReg.Get(node.Type)
		if err != nil {
			return fmt.Errorf("node %q (type=%q): %w", currentID, node.Type, err)
		}

		fmt.Printf("[attractor] executing node %q (type=%s)\n", node.ID, node.Type)

		if execErr := handler.Handle(ctx, node, e.pctx); execErr != nil {
			// Check for the exit sentinel.
			var exitSig ExitSignal
			if errors.As(execErr, &exitSig) {
				fmt.Printf("[attractor] pipeline complete at exit node %q\n", node.ID)
				e.pctx.Set("last_node", node.ID)
				if e.checkpointPath != "" {
					_ = e.pctx.SaveCheckpoint(e.checkpointPath, node.ID)
				}
				return nil
			}
			return fmt.Errorf("node %q: %w", node.ID, execErr)
		}

		// Checkpoint after every successful node execution.
		if e.checkpointPath != "" {
			if cpErr := e.pctx.SaveCheckpoint(e.checkpointPath, node.ID); cpErr != nil {
				return fmt.Errorf("node %q: save checkpoint: %w", node.ID, cpErr)
			}
		}

		// Determine next node.
		nextID, err := e.selectNext(node.ID)
		if err != nil {
			return fmt.Errorf("node %q: select next: %w", node.ID, err)
		}
		if nextID == "" {
			// No outgoing edges and not an exit node — treat as implicit exit.
			fmt.Printf("[attractor] pipeline ended at node %q (no outgoing edges)\n", node.ID)
			return nil
		}

		currentID = nextID

		// Respect context cancellation between nodes.
		select {
		case <-ctx.Done():
			return fmt.Errorf("pipeline cancelled at node %q: %w", currentID, ctx.Err())
		default:
		}
	}
}

// startNode returns the ID of the first node with type NodeTypeStart.
func (e *Engine) startNode() string {
	for _, n := range e.pipeline.Nodes {
		if n.Type == NodeTypeStart {
			return n.ID
		}
	}
	return ""
}

// selectNext evaluates outgoing edges from nodeID in order and returns the
// first edge whose condition evaluates to true.  An empty label (or
// underscore "_") is treated as an unconditional edge.
func (e *Engine) selectNext(nodeID string) (string, error) {
	edges := e.pipeline.OutgoingEdges(nodeID)
	if len(edges) == 0 {
		return "", nil
	}

	snap := e.pctx.Snapshot()

	for _, edge := range edges {
		cond := edge.Condition
		// Unconditional edges.
		if cond == "" || cond == "_" {
			return edge.To, nil
		}
		ok, err := EvalCondition(cond, snap)
		if err != nil {
			return "", fmt.Errorf("edge %q→%q: condition %q: %w", edge.From, edge.To, cond, err)
		}
		if ok {
			return edge.To, nil
		}
	}

	// No condition matched — this is a pipeline stall.
	return "", fmt.Errorf("no outgoing edge condition matched for node %q", nodeID)
}
