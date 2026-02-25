package pipeline

import "strings"

// ApplyStylesheet applies model_stylesheet rules to the pipeline's nodes.
// It mutates node Attrs["model"] for matching nodes.
func ApplyStylesheet(p *Pipeline) {
	if p.Stylesheet == nil {
		return
	}
	for _, rule := range p.Stylesheet.Rules {
		for _, node := range p.Nodes {
			if matchesSelector(rule.Selector, node) && rule.Model != "" {
				if node.Attrs == nil {
					node.Attrs = make(map[string]string)
				}
				node.Attrs["model"] = rule.Model
			}
		}
	}
}

// matchesSelector returns true if the node matches the given selector.
// Supported selectors:
//   - "*"               — all nodes
//   - "type[codergen]"  — nodes with type == codergen
//   - "id[my_node]"     — node with id == my_node
func matchesSelector(selector string, node *Node) bool {
	selector = strings.TrimSpace(selector)
	if selector == "*" {
		return true
	}
	if strings.HasPrefix(selector, "type[") && strings.HasSuffix(selector, "]") {
		want := selector[5 : len(selector)-1]
		return string(node.Type) == want
	}
	if strings.HasPrefix(selector, "id[") && strings.HasSuffix(selector, "]") {
		want := selector[3 : len(selector)-1]
		return node.ID == want
	}
	return false
}
