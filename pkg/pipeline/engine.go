package pipeline

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
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
	return e.run(ctx, startID, e.pctx, "")
}

// run is the inner sequential execution loop.  It stops when:
//   - an exit node is reached (returns nil)
//   - a node whose type equals stopAtType is encountered (returns nil, caller
//     takes over from that node)
//   - an error occurs
//
// stopAtType == "" means run until exit.
func (e *Engine) run(ctx context.Context, startID string, pctx *PipelineContext, stopAtType NodeType) error {
	visits := make(map[string]int)
	currentID := startID

	for {
		// Respect context cancellation between nodes.
		select {
		case <-ctx.Done():
			return fmt.Errorf("pipeline cancelled at node %q: %w", currentID, ctx.Err())
		default:
		}

		// Cycle detection.
		visits[currentID]++
		if visits[currentID] > maxNodeVisits {
			return fmt.Errorf("cycle detected: node %q visited more than %d times", currentID, maxNodeVisits)
		}

		node, ok := e.pipeline.Nodes[currentID]
		if !ok {
			return fmt.Errorf("node %q not found in pipeline", currentID)
		}

		// Stop-at boundary: caller will handle this node type.
		if stopAtType != "" && node.Type == stopAtType {
			return nil
		}

		// ── Fan-out: run all branches in parallel then skip to fan_in ──────
		if node.Type == NodeTypeFanOut {
			if err := e.executeFanOut(ctx, node, pctx); err != nil {
				return err
			}
			// After fan-out completes, find and continue from fan_in.
			fanInID, err := e.findFanIn(node.ID)
			if err != nil {
				return fmt.Errorf("fan_out node %q: %w", node.ID, err)
			}
			currentID = fanInID
			continue
		}

		handler, err := e.handlerReg.Get(node.Type)
		if err != nil {
			return fmt.Errorf("node %q (type=%q): %w", currentID, node.Type, err)
		}

		slog.Info("executing node", "node", node.ID, "type", node.Type)

		if execErr := handler.Handle(ctx, node, pctx); execErr != nil {
			// Check for the exit sentinel.
			var exitSig ExitSignal
			if errors.As(execErr, &exitSig) {
				slog.Info("pipeline complete", "node", node.ID)
				pctx.Set("last_node", node.ID)
				if e.checkpointPath != "" {
					_ = pctx.SaveCheckpoint(e.checkpointPath, node.ID)
				}
				return nil
			}
			return fmt.Errorf("node %q: %w", node.ID, execErr)
		}

		// Checkpoint after every successful node execution.
		if e.checkpointPath != "" {
			if cpErr := pctx.SaveCheckpoint(e.checkpointPath, node.ID); cpErr != nil {
				return fmt.Errorf("node %q: save checkpoint: %w", node.ID, cpErr)
			}
		}

		// Determine next node.
		nextID, err := e.selectNext(node.ID, pctx)
		if err != nil {
			return fmt.Errorf("node %q: select next: %w", node.ID, err)
		}
		if nextID == "" {
			// No outgoing edges and not an exit node — treat as implicit exit.
			slog.Info("pipeline ended", "node", node.ID, "reason", "no outgoing edges")
			return nil
		}

		currentID = nextID
	}
}

// executeFanOut runs all outgoing branches of a fan_out node in parallel,
// using goroutines. Each branch receives an independent copy of pctx and
// runs until it reaches a fan_in node (exclusive). After all branches
// complete, their results are merged into pctx (last-write-wins).
func (e *Engine) executeFanOut(ctx context.Context, fanOutNode *Node, pctx *PipelineContext) error {
	outEdges := e.pipeline.OutgoingEdges(fanOutNode.ID)
	if len(outEdges) == 0 {
		return fmt.Errorf("fan_out node %q has no outgoing edges", fanOutNode.ID)
	}

	type branchResult struct {
		snap map[string]any
		err  error
	}
	results := make([]branchResult, len(outEdges))

	var wg sync.WaitGroup
	for i, edge := range outEdges {
		branchStart := edge.To
		idx := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			branchCtx := pctx.Copy()
			subEng := &Engine{
				pipeline:   e.pipeline,
				handlerReg: e.handlerReg,
				pctx:       branchCtx,
				// no checkpointing inside branches
			}
			slog.Debug("fan_out branch starting", "branch", branchStart)
			err := subEng.run(ctx, branchStart, branchCtx, NodeTypeFanIn)
			if err != nil {
				results[idx] = branchResult{err: fmt.Errorf("branch %q: %w", branchStart, err)}
				return
			}
			slog.Debug("fan_out branch complete", "branch", branchStart)
			results[idx] = branchResult{snap: branchCtx.Snapshot()}
		}()
	}
	wg.Wait()

	// Collect errors and merge results.
	var errs []error
	for _, r := range results {
		if r.err != nil {
			errs = append(errs, r.err)
			continue
		}
		pctx.Merge(r.snap)
	}
	if len(errs) > 0 {
		return fmt.Errorf("parallel branches failed: %v", errs)
	}
	return nil
}

// findFanIn performs a BFS from fanOutID to locate the first downstream node
// of type fan_in. Returns an error if none is reachable.
func (e *Engine) findFanIn(fanOutID string) (string, error) {
	visited := make(map[string]bool)
	queue := []string{fanOutID}
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		if visited[id] {
			continue
		}
		visited[id] = true
		for _, edge := range e.pipeline.OutgoingEdges(id) {
			next := edge.To
			if n, ok := e.pipeline.Nodes[next]; ok && n.Type == NodeTypeFanIn {
				return next, nil
			}
			if !visited[next] {
				queue = append(queue, next)
			}
		}
	}
	return "", fmt.Errorf("no fan_in node reachable from fan_out %q", fanOutID)
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
func (e *Engine) selectNext(nodeID string, pctx *PipelineContext) (string, error) {
	edges := e.pipeline.OutgoingEdges(nodeID)
	if len(edges) == 0 {
		return "", nil
	}

	snap := pctx.Snapshot()

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
