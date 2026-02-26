package pipeline

// NodeType identifies the kind of work a node performs.
type NodeType string

const (
	NodeTypeStart    NodeType = "start"
	NodeTypeExit     NodeType = "exit"
	NodeTypeCodergen NodeType = "codergen"
	NodeTypeHuman    NodeType = "wait.human"
	NodeTypeSet      NodeType = "set"
	NodeTypeFanOut   NodeType = "fan_out"
	NodeTypeFanIn    NodeType = "fan_in"
	NodeTypeHTTP     NodeType = "http"
	NodeTypeAssert   NodeType = "assert"
	NodeTypeSleep    NodeType = "sleep"
	NodeTypeSwitch    NodeType = "switch"
	NodeTypeEnv       NodeType = "env"
	NodeTypeReadFile    NodeType = "read_file"
	NodeTypeWriteFile   NodeType = "write_file"
	NodeTypeJSONExtract NodeType = "json_extract"
	NodeTypeSplit       NodeType = "split"
	NodeTypeMap         NodeType = "map"
	NodeTypePrompt      NodeType = "prompt"
	NodeTypeJSONDecode  NodeType = "json_decode"
	NodeTypeExec            NodeType = "exec"
	NodeTypeJSONPack        NodeType = "json_pack"
	NodeTypeRegex           NodeType = "regex"
	NodeTypeStringTransform NodeType = "string_transform"
	NodeTypeForEach         NodeType = "for_each"
)

// Node represents a single vertex in the pipeline graph.
type Node struct {
	ID    string
	Type  NodeType
	Attrs map[string]string // all DOT attributes
}

// Edge is a directed connection between two nodes.
type Edge struct {
	From      string
	To        string
	Condition string // empty means unconditional
}

// Pipeline is the parsed representation of a .dot pipeline file.
type Pipeline struct {
	Name       string
	Nodes      map[string]*Node
	Edges      []*Edge
	Stylesheet *Stylesheet
}

// OutgoingEdges returns all edges leaving nodeID, in definition order.
func (p *Pipeline) OutgoingEdges(nodeID string) []*Edge {
	var out []*Edge
	for _, e := range p.Edges {
		if e.From == nodeID {
			out = append(out, e)
		}
	}
	return out
}

// IncomingEdges returns all edges arriving at nodeID.
func (p *Pipeline) IncomingEdges(nodeID string) []*Edge {
	var out []*Edge
	for _, e := range p.Edges {
		if e.To == nodeID {
			out = append(out, e)
		}
	}
	return out
}

// Stylesheet holds CSS-like model configuration rules.
type Stylesheet struct {
	Rules []StyleRule
}

// StyleRule applies model settings to nodes matching a selector.
type StyleRule struct {
	Selector string // e.g. "type[codergen]" or "*"
	Model    string
}
