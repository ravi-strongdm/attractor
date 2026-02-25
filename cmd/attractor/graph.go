package main

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ravi-parthasarathy/attractor/pkg/pipeline"
)

func graphCmd() *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:   "graph <pipeline.dot>",
		Short: "Print a human-readable summary of a pipeline",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			dotFile := args[0]
			src, err := os.ReadFile(dotFile)
			if err != nil {
				return fmt.Errorf("read file: %w", err)
			}
			p, err := pipeline.ParseDOT(string(src))
			if err != nil {
				return fmt.Errorf("parse: %w", err)
			}

			switch strings.ToLower(format) {
			case "dot":
				fmt.Print(renderDOT(p))
			case "text", "":
				fmt.Print(renderText(p))
			default:
				return fmt.Errorf("unknown format %q: use text or dot", format)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&format, "format", "text", "output format: text or dot")
	return cmd
}

// topoOrder returns node IDs in BFS order from the start node; unreachable
// nodes are appended in sorted order at the end.
func topoOrder(p *pipeline.Pipeline) []string {
	// Find start node.
	var startID string
	for id, n := range p.Nodes {
		if n.Type == pipeline.NodeTypeStart {
			startID = id
			break
		}
	}

	visited := map[string]bool{}
	var order []string

	if startID != "" {
		queue := []string{startID}
		for len(queue) > 0 {
			cur := queue[0]
			queue = queue[1:]
			if visited[cur] {
				continue
			}
			visited[cur] = true
			order = append(order, cur)
			for _, e := range p.OutgoingEdges(cur) {
				if !visited[e.To] {
					queue = append(queue, e.To)
				}
			}
		}
	}

	// Append unreachable nodes in deterministic order.
	var rest []string
	for id := range p.Nodes {
		if !visited[id] {
			rest = append(rest, id)
		}
	}
	sort.Strings(rest)
	return append(order, rest...)
}

// truncate shortens s to maxLen chars, appending "…" if needed.
func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "…"
}

// renderText produces the human-readable text summary.
func renderText(p *pipeline.Pipeline) string {
	var sb strings.Builder

	order := topoOrder(p)
	fmt.Fprintf(&sb, "Pipeline: %s  (%d nodes, %d edges)\n", p.Name, len(p.Nodes), len(p.Edges))

	// Calculate column widths.
	maxIDLen := 4 // minimum "node"
	for id := range p.Nodes {
		if len(id) > maxIDLen {
			maxIDLen = len(id)
		}
	}

	fmt.Fprintf(&sb, "\nNodes:\n")
	for _, id := range order {
		n := p.Nodes[id]
		// Build attrs string (skip "type" since it's already the second column).
		var attrParts []string
		// Sort attr keys for determinism.
		keys := make([]string, 0, len(n.Attrs))
		for k := range n.Attrs {
			if k != "type" {
				keys = append(keys, k)
			}
		}
		sort.Strings(keys)
		for _, k := range keys {
			v := truncate(n.Attrs[k], 60)
			attrParts = append(attrParts, k+"="+v)
		}
		attrsStr := strings.Join(attrParts, " ")
		fmt.Fprintf(&sb, "  %-*s  %-12s  %s\n", maxIDLen, id, string(n.Type), attrsStr)
	}

	fmt.Fprintf(&sb, "\nEdges:\n")
	// Compute max From width for alignment.
	maxFromLen := 4
	for _, e := range p.Edges {
		if len(e.From) > maxFromLen {
			maxFromLen = len(e.From)
		}
	}
	for _, e := range p.Edges {
		if e.Condition != "" {
			fmt.Fprintf(&sb, "  %-*s  →  %s  [%s]\n", maxFromLen, e.From, e.To, e.Condition)
		} else {
			fmt.Fprintf(&sb, "  %-*s  →  %s\n", maxFromLen, e.From, e.To)
		}
	}

	return sb.String()
}

// dotQuote returns the value as a DOT-safe string, quoting if necessary.
func dotQuote(s string) string {
	// Quote if the value contains spaces, backslashes, or special DOT chars.
	needsQuote := s == "" ||
		strings.ContainsAny(s, " \t\n\\\"{}[]<>=;,")
	if needsQuote {
		escaped := strings.ReplaceAll(s, `\`, `\\`)
		escaped = strings.ReplaceAll(escaped, `"`, `\"`)
		return `"` + escaped + `"`
	}
	return s
}

// renderDOT produces a canonical DOT digraph string.
func renderDOT(p *pipeline.Pipeline) string {
	var sb strings.Builder

	name := p.Name
	if name == "" {
		name = "pipeline"
	}
	fmt.Fprintf(&sb, "digraph %s {\n", dotQuote(name))

	order := topoOrder(p)
	for _, id := range order {
		n := p.Nodes[id]
		// Build attr list: type first, then sorted rest.
		var parts []string
		parts = append(parts, "type="+dotQuote(string(n.Type)))

		keys := make([]string, 0, len(n.Attrs))
		for k := range n.Attrs {
			if k != "type" {
				keys = append(keys, k)
			}
		}
		sort.Strings(keys)
		for _, k := range keys {
			parts = append(parts, k+"="+dotQuote(n.Attrs[k]))
		}
		fmt.Fprintf(&sb, "    %s [%s]\n", dotQuote(id), strings.Join(parts, " "))
	}

	for _, e := range p.Edges {
		if e.Condition != "" {
			fmt.Fprintf(&sb, "    %s -> %s [label=%s]\n",
				dotQuote(e.From), dotQuote(e.To), dotQuote(e.Condition))
		} else {
			fmt.Fprintf(&sb, "    %s -> %s\n", dotQuote(e.From), dotQuote(e.To))
		}
	}

	fmt.Fprintf(&sb, "}\n")
	return sb.String()
}
