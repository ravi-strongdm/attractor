package pipeline

import (
	"fmt"
	"strings"

	gographviz "github.com/awalterschulze/gographviz"
)

// ParseDOT parses a Graphviz DOT string into a Pipeline.
func ParseDOT(src string) (*Pipeline, error) {
	graphAst, err := gographviz.ParseString(src)
	if err != nil {
		return nil, fmt.Errorf("dot parse error: %w", err)
	}

	// Use a custom permissive graph collector that accepts any attribute name
	// without the strict validation that gographviz.Graph performs.
	collector := newDOTCollector()
	if err := gographviz.Analyse(graphAst, collector); err != nil {
		return nil, fmt.Errorf("dot analyse error: %w", err)
	}

	p := &Pipeline{
		Name:  collector.name,
		Nodes: make(map[string]*Node),
	}

	// Build nodes
	for id, attrs := range collector.nodes {
		nodeType := NodeType(attrs["type"])
		if nodeType == "" {
			nodeType = NodeTypeCodergen // default for untyped nodes
		}
		// Make a copy of attrs so the pipeline owns the map
		nodeCopy := make(map[string]string, len(attrs))
		for k, v := range attrs {
			nodeCopy[k] = v
		}
		p.Nodes[id] = &Node{
			ID:    id,
			Type:  nodeType,
			Attrs: nodeCopy,
		}
	}

	// Build edges (in definition order)
	for _, e := range collector.edges {
		p.Edges = append(p.Edges, &Edge{
			From:      e.from,
			To:        e.to,
			Condition: e.condition,
		})
	}

	// Extract graph-level stylesheet
	if raw, ok := collector.graphAttrs["model_stylesheet"]; ok {
		p.Stylesheet = parseStylesheet(raw)
	}

	return p, nil
}

// ─── permissive DOT collector ─────────────────────────────────────────────────

type rawEdge struct {
	from, to  string
	condition string
}

// dotCollector implements gographviz.Interface without attribute validation.
type dotCollector struct {
	name       string
	nodes      map[string]map[string]string // id → attrs
	edges      []rawEdge
	graphAttrs map[string]string
	// defaultNodeAttrs holds attrs set at the graph level (node [...]).
	defaultNodeAttrs map[string]string
}

func newDOTCollector() *dotCollector {
	return &dotCollector{
		nodes:            make(map[string]map[string]string),
		graphAttrs:       make(map[string]string),
		defaultNodeAttrs: make(map[string]string),
	}
}

func (c *dotCollector) SetStrict(_ bool) error  { return nil }
func (c *dotCollector) SetDir(_ bool) error     { return nil }
func (c *dotCollector) SetName(n string) error  { c.name = unquote(n); return nil }
func (c *dotCollector) String() string          { return c.name }

func (c *dotCollector) AddNode(_ string, name string, attrs map[string]string) error {
	id := unquote(name)
	if _, ok := c.nodes[id]; !ok {
		// Copy default attrs first
		c.nodes[id] = make(map[string]string, len(c.defaultNodeAttrs))
		for k, v := range c.defaultNodeAttrs {
			c.nodes[id][k] = v
		}
	}
	for k, v := range attrs {
		c.nodes[id][k] = unquote(v)
	}
	return nil
}

func (c *dotCollector) AddEdge(src, dst string, _ bool, attrs map[string]string) error {
	cond := ""
	if lbl, ok := attrs["label"]; ok {
		cond = unquote(lbl)
	}
	c.edges = append(c.edges, rawEdge{from: unquote(src), to: unquote(dst), condition: cond})
	return nil
}

func (c *dotCollector) AddPortEdge(src, _, dst, _ string, directed bool, attrs map[string]string) error {
	return c.AddEdge(src, dst, directed, attrs)
}

func (c *dotCollector) AddAttr(_ string, field, value string) error {
	c.graphAttrs[field] = unquote(value)
	return nil
}

func (c *dotCollector) AddSubGraph(_, _ string, _ map[string]string) error { return nil }

// ─── helpers ─────────────────────────────────────────────────────────────────

// unquote strips surrounding double-quotes from a DOT attribute value.
func unquote(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}

// parseStylesheet parses a simple CSS-like model stylesheet.
// Example: `type[codergen] { model: "anthropic:claude-opus-4-6" }`
func parseStylesheet(src string) *Stylesheet {
	ss := &Stylesheet{}
	src = strings.TrimSpace(src)
	parts := strings.Split(src, "}")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		braceIdx := strings.Index(part, "{")
		if braceIdx < 0 {
			continue
		}
		selector := strings.TrimSpace(part[:braceIdx])
		body := strings.TrimSpace(part[braceIdx+1:])
		rule := StyleRule{Selector: selector}
		for _, line := range strings.Split(body, ";") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			kv := strings.SplitN(line, ":", 2)
			if len(kv) != 2 {
				continue
			}
			k := strings.TrimSpace(kv[0])
			v := strings.Trim(strings.TrimSpace(kv[1]), `"`)
			if k == "model" {
				rule.Model = v
			}
		}
		ss.Rules = append(ss.Rules, rule)
	}
	return ss
}
