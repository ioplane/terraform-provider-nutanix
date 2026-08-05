// Package task implements vendor-neutral task snapshots and synchronous waiting.
package task

import (
	"fmt"
	"time"
)

// Status is a vendor-neutral asynchronous task state.
type Status string

const (
	StatusQueued    Status = "queued"
	StatusRunning   Status = "running"
	StatusCanceling Status = "canceling"
	StatusSuspended Status = "suspended"
	StatusSucceeded Status = "succeeded"
	StatusFailed    Status = "failed"
	StatusCanceled  Status = "canceled"
	StatusUnknown   Status = "unknown"
	StatusRedacted  Status = "redacted"
)

// Outcome is the stable control-flow result of a task status.
type Outcome string

const (
	OutcomePending     Outcome = "pending"
	OutcomeSucceeded   Outcome = "succeeded"
	OutcomeFailed      Outcome = "failed"
	OutcomeCanceled    Outcome = "canceled"
	OutcomeUnsupported Outcome = "unsupported"
)

// Outcome maps every status fail closed to a waiter result.
func (s Status) Outcome() Outcome {
	switch s {
	case StatusQueued, StatusRunning, StatusCanceling, StatusSuspended:
		return OutcomePending
	case StatusSucceeded:
		return OutcomeSucceeded
	case StatusFailed:
		return OutcomeFailed
	case StatusCanceled:
		return OutcomeCanceled
	default:
		return OutcomeUnsupported
	}
}

// Projection contains only bounded, character-validated remote error metadata.
type Projection struct {
	Code          string
	ErrorGroup    string
	Severity      string
	AttributePath string
}

// Format prevents projected identifiers from entering default diagnostics.
func (Projection) Format(state fmt.State, _ rune) {
	_, _ = state.Write([]byte("task.Projection(redacted)"))
}

// Snapshot is one bounded, vendor-neutral task observation.
type Snapshot struct {
	ExtID              string
	Status             Status
	ProgressPercentage int32
	Errors             []Projection
	Warnings           []Projection
	LastUpdatedTime    time.Time
	ServerDelay        time.Duration
}

// Format prevents task identifiers and remote projections from entering diagnostics.
func (Snapshot) Format(state fmt.State, _ rune) {
	_, _ = state.Write([]byte("task.Snapshot(redacted)"))
}
