package task

import (
	"context"
	"errors"
	"reflect"
	"time"
	"unicode/utf8"
)

const (
	// DefaultPollInterval is used when a reader supplies no valid delay.
	DefaultPollInterval = 2 * time.Second
)

var (
	// ErrInvalidWaiter identifies an absent reader or sleeper dependency.
	ErrInvalidWaiter = errors.New("task waiter is invalid")
	// ErrInvalidWait identifies an invalid wait context or task identifier.
	ErrInvalidWait = errors.New("task wait input is invalid")
	// ErrTaskWaitInterrupted identifies an unknown sleeper failure.
	ErrTaskWaitInterrupted = errors.New("task wait was interrupted")
)

// Reader returns one vendor-neutral task snapshot.
type Reader interface {
	Read(context.Context, string) (Snapshot, error)
}

type sleeper func(context.Context, time.Duration) error

// Waiter synchronously reads, evaluates, and waits for one task.
type Waiter struct {
	reader  Reader
	sleeper sleeper
}

// NewWaiter returns a synchronous waiter using a context-aware timer.
func NewWaiter(reader Reader) (*Waiter, error) {
	return newWaiter(reader, sleepForPoll)
}

func newWaiter(reader Reader, selectedSleeper sleeper) (*Waiter, error) {
	if nilValue(reader) || selectedSleeper == nil {
		return nil, ErrInvalidWaiter
	}
	return &Waiter{reader: reader, sleeper: selectedSleeper}, nil
}

// Wait polls until success or a typed terminal/caller failure.
func (w *Waiter) Wait(ctx context.Context, extID string) (Snapshot, error) {
	if ctx == nil || !validWaitID(extID) {
		return Snapshot{}, ErrInvalidWait
	}
	if w == nil || nilValue(w.reader) || w.sleeper == nil {
		return Snapshot{}, ErrInvalidWaiter
	}
	for {
		if contextErr := ctx.Err(); contextErr != nil {
			return Snapshot{}, contextFailure(contextErr)
		}
		snapshot, err := w.reader.Read(ctx, extID)
		if err != nil {
			if contextErr := ctx.Err(); contextErr != nil {
				return Snapshot{}, contextFailure(contextErr)
			}
			return Snapshot{}, err
		}
		if contextErr := ctx.Err(); contextErr != nil {
			return Snapshot{}, contextFailure(contextErr)
		}
		switch snapshot.Status.Outcome() {
		case OutcomeSucceeded:
			return snapshot, nil
		case OutcomeFailed:
			return Snapshot{}, newTaskError(FailureFailed, projectionCodes(snapshot.Errors), ErrTaskFailed)
		case OutcomeCanceled:
			return Snapshot{}, newTaskError(FailureCanceled, projectionCodes(snapshot.Errors), ErrTaskCanceled)
		case OutcomeUnsupported:
			return Snapshot{}, newTaskError(FailureUnsupported, nil, ErrTaskUnsupported)
		case OutcomePending:
			delay := snapshot.ServerDelay
			if delay <= 0 {
				delay = DefaultPollInterval
			}
			if err := w.sleeper(ctx, delay); err != nil {
				if contextErr := ctx.Err(); contextErr != nil {
					return Snapshot{}, contextFailure(contextErr)
				}
				return Snapshot{}, ErrTaskWaitInterrupted
			}
		default:
			return Snapshot{}, newTaskError(FailureUnsupported, nil, ErrTaskUnsupported)
		}
	}
}

func projectionCodes(projections []Projection) []string {
	codes := make([]string, 0, min(len(projections), MaximumTaskCodes))
	for _, projection := range projections {
		if len(codes) == MaximumTaskCodes {
			break
		}
		codes = append(codes, projection.Code)
	}
	return codes
}

func contextFailure(cause error) *TaskError {
	if cause == context.DeadlineExceeded {
		return newTaskError(FailureDeadline, nil, context.DeadlineExceeded)
	}
	return newTaskError(FailureCanceled, nil, context.Canceled)
}

func sleepForPoll(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func validWaitID(value string) bool {
	if value == "" || !utf8.ValidString(value) {
		return false
	}
	for index := 0; index < len(value); index++ {
		if value[index] < 0x20 || value[index] == 0x7f {
			return false
		}
	}
	return true
}

func nilValue(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}
