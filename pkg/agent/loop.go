package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ravi-parthasarathy/attractor/pkg/agent/tools"
	"github.com/ravi-parthasarathy/attractor/pkg/llm"
)

const (
	defaultModel     = "anthropic:claude-sonnet-4-6"
	defaultMaxTokens = 4096
	defaultMaxTurns  = 50
)

// AgentResult holds the final output of a completed agent loop.
type AgentResult struct {
	Output  string
	Session *Session
}

// CodingAgentLoop runs an LLM + tool loop until the model stops using tools.
type CodingAgentLoop struct {
	client    llm.Client
	registry  *tools.Registry
	workdir   string
	model     string
	maxTokens int
	maxTurns  int
	system    string
	eventCh   chan<- Event
}

// Option configures a CodingAgentLoop.
type Option func(*CodingAgentLoop)

// WithModel sets the model for the agent.
func WithModel(model string) Option {
	return func(a *CodingAgentLoop) { a.model = model }
}

// WithSystem sets the system prompt.
func WithSystem(system string) Option {
	return func(a *CodingAgentLoop) { a.system = system }
}

// WithEvents provides a channel for event emission.
func WithEvents(ch chan<- Event) Option {
	return func(a *CodingAgentLoop) { a.eventCh = ch }
}

// WithMaxTokens sets the per-turn max token budget.
func WithMaxTokens(n int) Option {
	return func(a *CodingAgentLoop) { a.maxTokens = n }
}

// WithMaxTurns sets the maximum number of LLM turns before the loop aborts.
// A value <= 0 uses the default (50).
func WithMaxTurns(n int) Option {
	return func(a *CodingAgentLoop) {
		if n > 0 {
			a.maxTurns = n
		}
	}
}

// NewCodingAgentLoop creates a CodingAgentLoop.
func NewCodingAgentLoop(client llm.Client, registry *tools.Registry, workdir string, opts ...Option) *CodingAgentLoop {
	a := &CodingAgentLoop{
		client:    client,
		registry:  registry,
		workdir:   workdir,
		model:     defaultModel,
		maxTokens: defaultMaxTokens,
		maxTurns:  defaultMaxTurns,
	}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

// Run executes the agent loop for the given instruction.
// Returns when the model produces a response with no tool_use blocks.
func (a *CodingAgentLoop) Run(ctx context.Context, instruction string) (AgentResult, error) {
	session := NewSession(a.system)
	detector := NewLoopDetector(defaultSteeringThreshold)

	// Build tool definitions from registry
	allTools := a.registry.All()
	toolDefs := make([]llm.ToolDefinition, 0, len(allTools))
	for _, t := range allTools {
		toolDefs = append(toolDefs, llm.ToolDefinition{
			Name:        t.Name(),
			Description: t.Description(),
			InputSchema: t.InputSchema(),
		})
	}

	session.Append(llm.TextMessage(llm.RoleUser, instruction))
	a.emit(Event{Type: EventTypeLLMTurn, Content: "starting agent loop"})

	turns := 0
	for {
		turns++
		if turns > a.maxTurns {
			return AgentResult{}, &MaxTurnsError{Turns: a.maxTurns}
		}
		// Truncate if session is getting large
		if session.Len() > defaultTruncationHeadTurns+defaultTruncationTailTurns+5 {
			session.Truncate(defaultTruncationHeadTurns, defaultTruncationTailTurns)
		}

		req := llm.GenerateRequest{
			Model:     a.model,
			Messages:  session.Messages(),
			Tools:     toolDefs,
			System:    session.System(),
			MaxTokens: a.maxTokens,
		}

		resp, err := a.client.Complete(ctx, req)
		if err != nil {
			a.emit(Event{Type: EventTypeError, Content: err.Error(), IsError: true})
			return AgentResult{}, fmt.Errorf("agent loop: LLM call failed: %w", err)
		}

		session.Append(llm.Message{Role: llm.RoleAssistant, Content: resp.Content})
		a.emit(Event{Type: EventTypeLLMTurn, Content: fmt.Sprintf("stop_reason=%s tokens=%d", resp.StopReason, resp.Usage.OutputTokens)})

		// Collect tool calls and text output
		var toolCalls []*llm.ToolUse
		var textOutput string
		for _, b := range resp.Content {
			switch b.Type {
			case llm.ContentTypeToolUse:
				if b.ToolUse != nil {
					toolCalls = append(toolCalls, b.ToolUse)
				}
			case llm.ContentTypeText:
				textOutput += b.Text
			}
		}

		// No tool calls = model is done
		if len(toolCalls) == 0 {
			a.emit(Event{Type: EventTypeComplete, Content: textOutput})
			return AgentResult{Output: textOutput, Session: session}, nil
		}

		// Execute each tool call; build tool_result blocks
		toolResults := make([]llm.ContentBlock, 0, len(toolCalls))
		for _, tc := range toolCalls {
			a.emit(Event{Type: EventTypeToolCall, ToolName: tc.Name, Content: string(tc.Input)})

			// Loop detection: inject steering instead of executing
			if detector.Record(tc.Name, tc.Input) {
				steering := SteeringMessage()
				a.emit(Event{Type: EventTypeSteering, Content: steering})
				toolResults = append(toolResults, llm.ContentBlock{
					Type: llm.ContentTypeToolResult,
					ToolResult: &llm.ToolResult{
						ToolUseID: tc.ID,
						Content:   steering,
						IsError:   true,
					},
				})
				continue
			}

			tool, err := a.registry.Get(tc.Name)
			if err != nil {
				a.emit(Event{Type: EventTypeToolResult, ToolName: tc.Name, Content: "not found", IsError: true})
				toolResults = append(toolResults, llm.ContentBlock{
					Type: llm.ContentTypeToolResult,
					ToolResult: &llm.ToolResult{
						ToolUseID: tc.ID,
						Content:   fmt.Sprintf("tool not found: %s", tc.Name),
						IsError:   true,
					},
				})
				continue
			}

			var inputJSON json.RawMessage = tc.Input
			result, execErr := tool.Execute(ctx, inputJSON)
			if execErr != nil {
				a.emit(Event{Type: EventTypeToolResult, ToolName: tc.Name, Content: execErr.Error(), IsError: true})
				toolResults = append(toolResults, llm.ContentBlock{
					Type: llm.ContentTypeToolResult,
					ToolResult: &llm.ToolResult{
						ToolUseID: tc.ID,
						Content:   execErr.Error(),
						IsError:   true,
					},
				})
			} else {
				a.emit(Event{Type: EventTypeToolResult, ToolName: tc.Name, Content: result})
				toolResults = append(toolResults, llm.ContentBlock{
					Type: llm.ContentTypeToolResult,
					ToolResult: &llm.ToolResult{
						ToolUseID: tc.ID,
						Content:   result,
						IsError:   false,
					},
				})
			}
		}

		session.Append(llm.Message{Role: llm.RoleUser, Content: toolResults})
	}
}

func (a *CodingAgentLoop) emit(e Event) {
	if a.eventCh != nil {
		select {
		case a.eventCh <- e:
		default:
		}
	}
}
