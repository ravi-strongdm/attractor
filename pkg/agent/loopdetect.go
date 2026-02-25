package agent

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
)

const defaultSteeringThreshold = 3

// callKey is a fingerprint of a tool call.
type callKey struct {
	toolName  string
	inputHash string
}

// LoopDetector tracks tool call history and detects repeated identical calls.
type LoopDetector struct {
	counts    map[callKey]int
	threshold int
}

// NewLoopDetector creates a LoopDetector with the given repeat threshold.
// A threshold <= 0 uses the default (3).
func NewLoopDetector(threshold int) *LoopDetector {
	if threshold <= 0 {
		threshold = defaultSteeringThreshold
	}
	return &LoopDetector{counts: make(map[callKey]int), threshold: threshold}
}

// Record records a tool call and returns true if the loop threshold is reached.
func (d *LoopDetector) Record(toolName string, input json.RawMessage) bool {
	h := sha256.Sum256(input)
	key := callKey{toolName: toolName, inputHash: fmt.Sprintf("%x", h)}
	d.counts[key]++
	return d.counts[key] >= d.threshold
}

// SteeringMessage returns the message injected when a loop is detected.
func SteeringMessage() string {
	return "You appear to be stuck in a loop. Try a fundamentally different approach to complete the task."
}
