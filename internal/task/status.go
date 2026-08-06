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

// EntityReference is a bounded reference to an entity affected by a task.
// It is used for identity recovery after asynchronous create operations.
type EntityReference struct {
	ExtID string
	Rel   string
	Name  string
}

// Format prevents entity identifiers and names from entering diagnostics.
func (EntityReference) Format(state fmt.State, _ rune) {
	_, _ = state.Write([]byte("task.EntityReference(redacted)"))
}

// Snapshot is one bounded, vendor-neutral task observation.
type Snapshot struct {
	ExtID              string
	Status             Status
	ProgressPercentage int32
	EntitiesAffected   []EntityReference
	Errors             []Projection
	Warnings           []Projection
	LastUpdatedTime    time.Time
	ServerDelay        time.Duration
}

// AsyncOperation identifies a submitted asynchronous API operation by its
// Prism task identifier. Product clients alias this stable transport-neutral
// value instead of defining parallel operation types.
type AsyncOperation struct {
	TaskID string
}

// Format prevents task identifiers and remote projections from entering diagnostics.
func (Snapshot) Format(state fmt.State, _ rune) {
	_, _ = state.Write([]byte("task.Snapshot(redacted)"))
}
