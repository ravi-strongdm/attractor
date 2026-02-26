package agent

import (
	"github.com/ravi-parthasarathy/attractor/pkg/llm"
)

const (
	defaultTruncationHeadTurns = 2
	defaultTruncationTailTurns = 10
)

// Session manages the conversation history for an agent loop.
type Session struct {
	messages []llm.Message
	system   string
}

// NewSession creates a session with an optional system prompt.
func NewSession(system string) *Session {
	return &Session{system: system}
}

// Append adds a message to the session history.
func (s *Session) Append(msg llm.Message) {
	s.messages = append(s.messages, msg)
}

// Messages returns all messages in the session.
func (s *Session) Messages() []llm.Message {
	return s.messages
}

// System returns the system prompt.
func (s *Session) System() string {
	return s.system
}

// Len returns the number of messages.
func (s *Session) Len() int {
	return len(s.messages)
}

// Truncate shrinks the session when it grows too large by keeping only
// messages[0] (the original user instruction) and the most recent tailN turns.
//
// The tail is adjusted to start at an assistant message so that the resulting
// sequence messages[0](user) → tail[0](asst) → … preserves valid role
// alternation and keeps every tool_use/tool_result pair intact.
//
// headN is accepted for API compatibility but only messages[0] is kept as head.
func (s *Session) Truncate(headN, tailN int) {
	total := len(s.messages)
	if total <= headN+tailN {
		return
	}
	if total == 0 {
		return
	}

	// Find the tail start: first assistant message at or after (total - tailN).
	// Starting on an assistant message ensures messages[0](user) → tail[0](asst)
	// is valid alternation, and any tool_use in tail[0] has its matching
	// tool_results in tail[1] (since consecutive session entries are intact).
	tailStart := total - tailN
	if tailStart < 1 {
		tailStart = 1
	}
	for tailStart < total && s.messages[tailStart].Role == llm.RoleUser {
		tailStart++
	}
	// Nothing meaningful to drop if the tail already starts right after head.
	if tailStart >= total || tailStart <= 1 {
		return
	}

	tail := make([]llm.Message, total-tailStart)
	copy(tail, s.messages[tailStart:])

	combined := make([]llm.Message, 0, 1+len(tail))
	combined = append(combined, s.messages[0]) // always keep original instruction
	combined = append(combined, tail...)
	s.messages = combined
}
