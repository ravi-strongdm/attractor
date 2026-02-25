package agent

import (
	"fmt"

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

// Truncate keeps the first headN and last tailN messages, inserting a
// [TRUNCATED] marker between them when messages are dropped.
// Per spec: the first turn (seed instruction) is always preserved.
func (s *Session) Truncate(headN, tailN int) {
	total := len(s.messages)
	if total <= headN+tailN {
		return
	}
	omitted := total - headN - tailN
	marker := llm.TextMessage(llm.RoleUser,
		fmt.Sprintf("[TRUNCATED — %d messages omitted]", omitted))
	head := make([]llm.Message, headN)
	copy(head, s.messages[:headN])
	tail := make([]llm.Message, tailN)
	copy(tail, s.messages[total-tailN:])

	combined := make([]llm.Message, 0, headN+1+tailN)
	combined = append(combined, head...)
	combined = append(combined, marker)
	combined = append(combined, tail...)
	s.messages = combined
}
