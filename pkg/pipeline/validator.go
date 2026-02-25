package pipeline

import (
	"fmt"
	"strings"
)

// LintError describes a structural problem in a pipeline.
type LintError struct {
	NodeID  string
	Message string
}

func (e LintError) Error() string {
	if e.NodeID != "" {
		return fmt.Sprintf("node %q: %s", e.NodeID, e.Message)
	}
	return e.Message
}

// nodeRequiredAttrs maps each node type to the list of attribute names that
// must be present (non-empty) in the DOT file.  The linter reports all
// missing attributes across all nodes before aborting.
var nodeRequiredAttrs = map[NodeType][]string{
	NodeTypeSet:         {"key"},
	NodeTypeHTTP:        {"url"},
	NodeTypeAssert:      {"expr"},
	NodeTypeSleep:       {"duration"},
	NodeTypeSwitch:      {"key"},
	NodeTypeEnv:         {"key", "from"},
	NodeTypeReadFile:    {"key", "path"},
	NodeTypeWriteFile:   {"path", "content"},
	NodeTypeJSONExtract: {"source", "path", "key"},
}

// Validate checks a pipeline for structural correctness.
// Returns all discovered errors (not just the first).
func Validate(p *Pipeline) []LintError {
	var errs []LintError

	// Exactly one start node
	var startNodes []string
	for id, n := range p.Nodes {
		if n.Type == NodeTypeStart {
			startNodes = append(startNodes, id)
		}
	}
	switch len(startNodes) {
	case 0:
		errs = append(errs, LintError{Message: "pipeline must have exactly one start node"})
	case 1:
		// good
	default:
		errs = append(errs, LintError{Message: fmt.Sprintf("pipeline has %d start nodes; exactly one required", len(startNodes))})
	}

	// Exactly one exit node
	var exitNodes []string
	for id, n := range p.Nodes {
		if n.Type == NodeTypeExit {
			exitNodes = append(exitNodes, id)
		}
	}
	switch len(exitNodes) {
	case 0:
		errs = append(errs, LintError{Message: "pipeline must have exactly one exit node"})
	case 1:
		// good
	default:
		errs = append(errs, LintError{Message: fmt.Sprintf("pipeline has %d exit nodes; exactly one required", len(exitNodes))})
	}

	// All edge endpoints must reference existing nodes
	for _, e := range p.Edges {
		if _, ok := p.Nodes[e.From]; !ok {
			errs = append(errs, LintError{Message: fmt.Sprintf("edge references unknown source node %q", e.From)})
		}
		if _, ok := p.Nodes[e.To]; !ok {
			errs = append(errs, LintError{Message: fmt.Sprintf("edge references unknown target node %q", e.To)})
		}
	}

	// All non-start nodes must be reachable from start
	if len(startNodes) == 1 {
		reachable := reachableFrom(p, startNodes[0])
		for id := range p.Nodes {
			if id == startNodes[0] {
				continue
			}
			if !reachable[id] {
				errs = append(errs, LintError{NodeID: id, Message: "node is not reachable from start"})
			}
		}
	}

	// Every fan_out node must have a reachable fan_in node downstream.
	for id, n := range p.Nodes {
		if n.Type != NodeTypeFanOut {
			continue
		}
		if !hasFanInReachable(p, id) {
			errs = append(errs, LintError{NodeID: id, Message: "fan_out node has no reachable fan_in node"})
		}
	}

	// Required attribute checks for known node types.
	for id, n := range p.Nodes {
		required, ok := nodeRequiredAttrs[n.Type]
		if !ok {
			continue
		}
		for _, attr := range required {
			if n.Attrs[attr] == "" {
				errs = append(errs, LintError{
					NodeID:  id,
					Message: fmt.Sprintf("missing required attribute %q for node type %q", attr, n.Type),
				})
			}
		}
	}

	return errs
}

// hasFanInReachable returns true if a fan_in node is reachable from startID via BFS.
func hasFanInReachable(p *Pipeline, startID string) bool {
	visited := map[string]bool{}
	queue := []string{startID}
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		if visited[id] {
			continue
		}
		visited[id] = true
		for _, e := range p.OutgoingEdges(id) {
			if n, ok := p.Nodes[e.To]; ok && n.Type == NodeTypeFanIn {
				return true
			}
			if !visited[e.To] {
				queue = append(queue, e.To)
			}
		}
	}
	return false
}

// ValidateNode checks a single node's required attributes and returns any
// lint errors.  This is a convenience helper used in tests and by Validate.
func ValidateNode(n *Node) []LintError {
	var errs []LintError
	required, ok := nodeRequiredAttrs[n.Type]
	if !ok {
		return nil
	}
	for _, attr := range required {
		if n.Attrs[attr] == "" {
			errs = append(errs, LintError{
				NodeID:  n.ID,
				Message: fmt.Sprintf("missing required attribute %q for node type %q", attr, n.Type),
			})
		}
	}
	return errs
}

// ValidateErr calls Validate and returns nil if there are no errors, or a
// combined error message listing all lint errors.
func ValidateErr(p *Pipeline) error {
	errs := Validate(p)
	if len(errs) == 0 {
		return nil
	}
	msgs := make([]string, len(errs))
	for i, e := range errs {
		msgs[i] = e.Error()
	}
	return fmt.Errorf("pipeline validation failed:\n  %s", strings.Join(msgs, "\n  "))
}

// reachableFrom returns the set of node IDs reachable from start via directed edges.
func reachableFrom(p *Pipeline, start string) map[string]bool {
	visited := map[string]bool{}
	queue := []string{start}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if visited[cur] {
			continue
		}
		visited[cur] = true
		for _, e := range p.OutgoingEdges(cur) {
			queue = append(queue, e.To)
		}
	}
	return visited
}
