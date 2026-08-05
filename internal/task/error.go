package task

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"
)

const (
	// MaximumTaskCodes bounds stable code projection into one TaskError.
	MaximumTaskCodes = 100
	// MaximumTaskCodeBytes bounds each projected stable code.
	MaximumTaskCodeBytes = 128
)

var (
	// ErrTaskFailed identifies a remote terminal failure.
	ErrTaskFailed = errors.New("task failed")
	// ErrTaskCanceled identifies a remote terminal cancellation.
	ErrTaskCanceled = errors.New("task was canceled")
	// ErrTaskUnsupported identifies an unknown, redacted, or future task state.
	ErrTaskUnsupported = errors.New("task status is unsupported")
)

// FailureKind is a stable task waiter failure class.
type FailureKind string

const (
	FailureFailed      FailureKind = "failed"
	FailureCanceled    FailureKind = "canceled"
	FailureUnsupported FailureKind = "unsupported"
	FailureDeadline    FailureKind = "deadline"
)

// TaskError is a stable, redacted task waiter failure.
type TaskError struct {
	kind  FailureKind
	codes []string
	cause error
}

// Error returns a stable message without task IDs or vendor text.
func (e *TaskError) Error() string {
	if e == nil {
		return "task wait failed"
	}
	return fmt.Sprintf("task wait failed (%s)", e.kind)
}

// Format prevents private fields and wrapped values from entering diagnostics.
func (e *TaskError) Format(state fmt.State, _ rune) {
	_, _ = state.Write([]byte(e.Error()))
}

// Unwrap exposes only an approved stable or caller-context cause.
func (e *TaskError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

// Kind returns the stable task failure class.
func (e *TaskError) Kind() FailureKind {
	if e == nil {
		return ""
	}
	return e.kind
}

// Codes returns a copy of bounded, validated remote codes.
func (e *TaskError) Codes() []string {
	if e == nil {
		return nil
	}
	return slices.Clone(e.codes)
}

func newTaskError(kind FailureKind, codes []string, cause error) *TaskError {
	return &TaskError{
		kind:  kind,
		codes: safeTaskCodes(codes),
		cause: approvedTaskCause(cause),
	}
}

func safeTaskCodes(codes []string) []string {
	projected := make([]string, 0, min(len(codes), MaximumTaskCodes))
	for _, code := range codes {
		if len(projected) == MaximumTaskCodes {
			break
		}
		if validTaskCode(code) {
			projected = append(projected, code)
		}
	}
	return projected
}

func validTaskCode(code string) bool {
	if code == "" || len(code) > MaximumTaskCodeBytes || !utf8.ValidString(code) {
		return false
	}
	for index := 0; index < len(code); index++ {
		character := code[index]
		if (character >= 'a' && character <= 'z') ||
			(character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') ||
			strings.ContainsRune("._:/-", rune(character)) {
			continue
		}
		return false
	}
	return true
}

func approvedTaskCause(cause error) error {
	switch cause {
	case ErrTaskFailed, ErrTaskCanceled, ErrTaskUnsupported, context.Canceled, context.DeadlineExceeded:
		return cause
	default:
		return nil
	}
}
