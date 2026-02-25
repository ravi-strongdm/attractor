package agent_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/ravi-parthasarathy/attractor/pkg/agent"
	"github.com/ravi-parthasarathy/attractor/pkg/agent/tools"
	"github.com/ravi-parthasarathy/attractor/pkg/llm"
)

// ─── Session tests ────────────────────────────────────────────────────────────

func TestSession_AppendAndMessages(t *testing.T) {
	sess := agent.NewSession("You are a helpful assistant.")
	if sess.System() != "You are a helpful assistant." {
		t.Fatalf("unexpected system prompt: %q", sess.System())
	}
	sess.Append(llm.TextMessage(llm.RoleUser, "hello"))
	sess.Append(llm.TextMessage(llm.RoleAssistant, "hi there"))
	if sess.Len() != 2 {
		t.Fatalf("expected 2 messages, got %d", sess.Len())
	}
}

func TestSession_Truncate(t *testing.T) {
	sess := agent.NewSession("")
	// Append 15 messages.
	for i := 0; i < 15; i++ {
		role := llm.RoleUser
		if i%2 == 1 {
			role = llm.RoleAssistant
		}
		sess.Append(llm.TextMessage(role, fmt.Sprintf("msg-%d", i)))
	}
	if sess.Len() != 15 {
		t.Fatalf("expected 15 messages before truncation, got %d", sess.Len())
	}

	// Truncate to head=2, tail=4 → 2 + marker + 4 = 7 entries.
	sess.Truncate(2, 4)
	msgs := sess.Messages()
	if len(msgs) != 7 {
		t.Fatalf("after Truncate(2,4): expected 7 messages, got %d", len(msgs))
	}

	// First two messages unchanged.
	first := msgs[0]
	if len(first.Content) == 0 || first.Content[0].Text != "msg-0" {
		t.Errorf("msgs[0] content = %v, want msg-0", first.Content)
	}

	// Middle entry is the truncation marker.
	marker := msgs[2]
	if len(marker.Content) == 0 {
		t.Fatal("marker message has no content")
	}
	if !strings.Contains(marker.Content[0].Text, "TRUNCATED") {
		t.Errorf("marker text = %q, expected to contain TRUNCATED", marker.Content[0].Text)
	}
	if !strings.Contains(marker.Content[0].Text, "9") {
		t.Errorf("marker text = %q, expected to mention 9 omitted messages", marker.Content[0].Text)
	}

	// Last message is msg-14.
	last := msgs[6]
	if len(last.Content) == 0 || last.Content[0].Text != "msg-14" {
		t.Errorf("msgs[6] content = %v, want msg-14", last.Content)
	}
}

func TestSession_TruncateNoOp(t *testing.T) {
	sess := agent.NewSession("")
	for i := 0; i < 3; i++ {
		sess.Append(llm.TextMessage(llm.RoleUser, fmt.Sprintf("m%d", i)))
	}
	sess.Truncate(2, 4) // head+tail >= len → no truncation
	if sess.Len() != 3 {
		t.Errorf("expected 3 messages, got %d", sess.Len())
	}
}

// ─── LoopDetector tests ───────────────────────────────────────────────────────

func TestLoopDetector_DetectsRepeat(t *testing.T) {
	ld := agent.NewLoopDetector(3)
	input := json.RawMessage(`{"path":"foo.go"}`)
	if ld.Record("read_file", input) {
		t.Fatal("should not detect loop on 1st call")
	}
	if ld.Record("read_file", input) {
		t.Fatal("should not detect loop on 2nd call")
	}
	if !ld.Record("read_file", input) {
		t.Fatal("should detect loop on 3rd identical call (threshold=3)")
	}
}

func TestLoopDetector_DifferentCalls(t *testing.T) {
	ld := agent.NewLoopDetector(3)
	calls := []struct {
		name  string
		input json.RawMessage
	}{
		{"read_file", json.RawMessage(`{"path":"a.go"}`)},
		{"read_file", json.RawMessage(`{"path":"b.go"}`)},
		{"write_file", json.RawMessage(`{"path":"a.go","content":"x"}`)},
	}
	for _, c := range calls {
		if ld.Record(c.name, c.input) {
			t.Errorf("unexpected loop detection for %s %s", c.name, c.input)
		}
	}
}

func TestLoopDetector_SteeringMessage(t *testing.T) {
	msg := agent.SteeringMessage()
	if msg == "" {
		t.Error("steering message should not be empty")
	}
}

func TestLoopDetector_DefaultThreshold(t *testing.T) {
	// Threshold <= 0 uses default (3)
	ld := agent.NewLoopDetector(0)
	input := json.RawMessage(`{}`)
	// Should not trigger on calls 1 or 2
	for i := range 2 {
		if ld.Record("t", input) {
			t.Fatalf("default threshold should not trigger on call %d", i+1)
		}
	}
	// Third call should trigger
	if !ld.Record("t", input) {
		t.Fatal("default threshold should trigger on 3rd call")
	}
}

// ─── CodingAgentLoop max-turns test ──────────────────────────────────────────

// infiniteToolClient always asks the agent to call a tool, forcing the loop
// to run indefinitely until something stops it.
type infiniteToolClient struct{}

func (c *infiniteToolClient) Complete(_ context.Context, _ llm.GenerateRequest) (llm.GenerateResponse, error) {
	return llm.GenerateResponse{
		Content: []llm.ContentBlock{
			{
				Type: llm.ContentTypeToolUse,
				ToolUse: &llm.ToolUse{
					ID:    "call-1",
					Name:  "list_dir",
					Input: json.RawMessage(`{"path":"."}`),
				},
			},
		},
		StopReason: llm.StopReasonToolUse,
	}, nil
}

func (c *infiniteToolClient) Stream(_ context.Context, _ llm.GenerateRequest) (<-chan llm.StreamEvent, error) {
	ch := make(chan llm.StreamEvent)
	close(ch)
	return ch, nil
}

func TestAgentLoop_MaxTurns(t *testing.T) {
	dir := t.TempDir()
	reg := tools.NewRegistry()
	reg.Register(tools.NewListDirTool(dir))

	loop := agent.NewCodingAgentLoop(
		&infiniteToolClient{},
		reg,
		dir,
		agent.WithMaxTurns(3),
	)

	_, err := loop.Run(context.Background(), "loop forever")
	if err == nil {
		t.Fatal("expected MaxTurnsError, got nil")
	}
	var maxErr *agent.MaxTurnsError
	if !errors.As(err, &maxErr) {
		t.Fatalf("expected *agent.MaxTurnsError, got %T: %v", err, err)
	}
	if maxErr.Turns != 3 {
		t.Errorf("MaxTurnsError.Turns = %d, want 3", maxErr.Turns)
	}
}
