package agent

import "fmt"

// MaxTurnsError is returned when the agent loop exceeds its configured turn limit.
type MaxTurnsError struct {
	Turns int
}

func (e *MaxTurnsError) Error() string {
	return fmt.Sprintf("agent loop exceeded maximum turns (%d)", e.Turns)
}
