package agent_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	// Append 15 alternating messages: 0(user),1(asst),…,14(asst).
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

	// Truncate(headN=2, tailN=4):
	//   tailStart = max(1, 15-4) = 11; messages[11] is asst → stop.
	//   combined = [messages[0]] + messages[11..14] = 5 entries.
	sess.Truncate(2, 4)
	msgs := sess.Messages()
	if len(msgs) != 5 {
		t.Fatalf("after Truncate(2,4): expected 5 messages, got %d", len(msgs))
	}

	// First message is the original user instruction.
	if len(msgs[0].Content) == 0 || msgs[0].Content[0].Text != "msg-0" {
		t.Errorf("msgs[0] content = %v, want msg-0", msgs[0].Content)
	}
	// Second message is the first kept tail message (first asst at ≥ position 11).
	if len(msgs[1].Content) == 0 || msgs[1].Content[0].Text != "msg-11" {
		t.Errorf("msgs[1] content = %v, want msg-11", msgs[1].Content)
	}
	// Last message is msg-14.
	if len(msgs[4].Content) == 0 || msgs[4].Content[0].Text != "msg-14" {
		t.Errorf("msgs[4] content = %v, want msg-14", msgs[4].Content)
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
