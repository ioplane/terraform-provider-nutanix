package task

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"testing/synctest"
	"time"
)

func TestWaiterHandlesEveryKnownAndFutureState(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		status   Status
		wantKind FailureKind
		wantIs   error
	}{
		{name: "queued", status: StatusQueued},
		{name: "running", status: StatusRunning},
		{name: "canceling", status: StatusCanceling},
		{name: "suspended", status: StatusSuspended},
		{name: "succeeded", status: StatusSucceeded},
		{name: "failed", status: StatusFailed, wantKind: FailureFailed, wantIs: ErrTaskFailed},
		{name: "canceled", status: StatusCanceled, wantKind: FailureCanceled, wantIs: ErrTaskCanceled},
		{name: "unknown", status: StatusUnknown, wantKind: FailureUnsupported, wantIs: ErrTaskUnsupported},
		{name: "redacted", status: StatusRedacted, wantKind: FailureUnsupported, wantIs: ErrTaskUnsupported},
		{name: "future", status: Status("future-status-secret-canary"), wantKind: FailureUnsupported, wantIs: ErrTaskUnsupported},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			calls := 0
			reader := readerFunc(func(context.Context, string) (Snapshot, error) {
				calls++
				if test.status.Outcome() == OutcomePending && calls > 1 {
					return Snapshot{Status: StatusSucceeded}, nil
				}
				return Snapshot{
					Status: test.status,
					Errors: []Projection{{Code: "TASK.SAFE-1"}},
				}, nil
			})
			waiter, err := newWaiter(reader, func(context.Context, time.Duration) error { return nil })
			if err != nil {
				t.Fatalf("newWaiter() error = %v", err)
			}
			snapshot, err := waiter.Wait(context.Background(), "task-id-secret-canary")
			if test.wantKind == "" {
				if err != nil || snapshot.Status != StatusSucceeded {
					t.Fatalf("Wait() = %#v, %v", snapshot, err)
				}
				wantCalls := 1
				if test.status.Outcome() == OutcomePending {
					wantCalls = 2
				}
				if calls != wantCalls {
					t.Fatalf("reader calls = %d, want %d", calls, wantCalls)
				}
				return
			}
			var taskError *TaskError
			if !errors.As(err, &taskError) || taskError.Kind() != test.wantKind || !errors.Is(err, test.wantIs) {
				t.Fatalf("Wait() error = %v, want kind=%q cause=%v", err, test.wantKind, test.wantIs)
			}
			if test.status == StatusFailed && fmt.Sprint(taskError.Codes()) != "[TASK.SAFE-1]" {
				t.Fatalf("failure codes = %v", taskError.Codes())
			}
			if strings.Contains(fmt.Sprintf("%+v", err), "future-status-secret-canary") ||
				strings.Contains(fmt.Sprintf("%+v", err), "task-id-secret-canary") {
				t.Fatalf("Wait() leaked status or ID: %+v", err)
			}
		})
	}
}

func TestWaiterUsesDefaultAndValidServerDelayWithFakeTime(t *testing.T) {
	tests := []struct {
		name        string
		serverDelay time.Duration
		wantElapsed time.Duration
	}{
		{name: "default", wantElapsed: DefaultPollInterval},
		{name: "server", serverDelay: 7 * time.Second, wantElapsed: 7 * time.Second},
		{name: "negative falls back", serverDelay: -time.Second, wantElapsed: DefaultPollInterval},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				calls := 0
				reader := readerFunc(func(context.Context, string) (Snapshot, error) {
					calls++
					if calls == 1 {
						return Snapshot{Status: StatusRunning, ServerDelay: test.serverDelay}, nil
					}
					return Snapshot{Status: StatusSucceeded}, nil
				})
				waiter, err := NewWaiter(reader)
				if err != nil {
					t.Fatalf("NewWaiter() error = %v", err)
				}
				started := time.Now()
				if _, err := waiter.Wait(context.Background(), "task-id"); err != nil {
					t.Fatalf("Wait() error = %v", err)
				}
				if elapsed := time.Since(started); elapsed != test.wantElapsed {
					t.Fatalf("fake elapsed = %s, want %s", elapsed, test.wantElapsed)
				}
			})
		})
	}
}

func TestWaiterHonorsCallerCancellationAndDeadline(t *testing.T) {
	t.Run("canceled after read", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		reader := readerFunc(func(context.Context, string) (Snapshot, error) {
			cancel()
			return Snapshot{Status: StatusRunning}, nil
		})
		waiter, err := NewWaiter(reader)
		if err != nil {
			t.Fatalf("NewWaiter() error = %v", err)
		}
		_, err = waiter.Wait(ctx, "task-id")
		var taskError *TaskError
		if !errors.As(err, &taskError) || taskError.Kind() != FailureCanceled || !errors.Is(err, context.Canceled) {
			t.Fatalf("Wait() error = %v, want caller cancellation", err)
		}
	})

	t.Run("deadline during wait", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			reader := readerFunc(func(context.Context, string) (Snapshot, error) {
				return Snapshot{Status: StatusRunning}, nil
			})
			waiter, err := NewWaiter(reader)
			if err != nil {
				t.Fatalf("NewWaiter() error = %v", err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			started := time.Now()
			_, err = waiter.Wait(ctx, "task-id")
			var taskError *TaskError
			if !errors.As(err, &taskError) || taskError.Kind() != FailureDeadline || !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("Wait() error = %v, want deadline", err)
			}
			if elapsed := time.Since(started); elapsed != time.Second {
				t.Fatalf("fake elapsed = %s, want 1s", elapsed)
			}
		})
	})
}

func TestWaiterBoundsAndValidatesProjectedCodes(t *testing.T) {
	t.Parallel()

	projections := make([]Projection, 0, MaximumTaskCodes+4)
	for index := 0; index < MaximumTaskCodes+2; index++ {
		projections = append(projections, Projection{Code: fmt.Sprintf("TASK-%03d", index)})
	}
	projections = append(projections,
		Projection{Code: "code with spaces secret-canary"},
		Projection{Code: strings.Repeat("x", MaximumTaskCodeBytes+1)},
	)
	reader := readerFunc(func(context.Context, string) (Snapshot, error) {
		return Snapshot{Status: StatusFailed, Errors: projections}, nil
	})
	waiter, err := NewWaiter(reader)
	if err != nil {
		t.Fatalf("NewWaiter() error = %v", err)
	}
	_, err = waiter.Wait(context.Background(), "task-id")
	var taskError *TaskError
	if !errors.As(err, &taskError) || len(taskError.Codes()) != MaximumTaskCodes {
		t.Fatalf("TaskError codes = %d, want %d", len(taskError.Codes()), MaximumTaskCodes)
	}
	for _, rendered := range []string{err.Error(), fmt.Sprintf("%+v", err), fmt.Sprintf("%#v", err)} {
		if strings.Contains(rendered, "secret-canary") {
			t.Fatalf("TaskError leaked invalid code: %q", rendered)
		}
	}
}

func TestWaiterBoundsSourceProjectionInspection(t *testing.T) {
	t.Parallel()

	projections := make([]Projection, MaximumTaskCodes+1)
	for index := 0; index < MaximumTaskCodes; index++ {
		projections[index] = Projection{Code: "invalid code with spaces"}
	}
	projections[MaximumTaskCodes] = Projection{Code: "MUST-NOT-BE-INSPECTED"}
	reader := readerFunc(func(context.Context, string) (Snapshot, error) {
		return Snapshot{Status: StatusFailed, Errors: projections}, nil
	})
	waiter, err := NewWaiter(reader)
	if err != nil {
		t.Fatalf("NewWaiter() error = %v", err)
	}
	_, err = waiter.Wait(context.Background(), "task-id")
	var taskError *TaskError
	if !errors.As(err, &taskError) || len(taskError.Codes()) != 0 {
		t.Fatalf("TaskError codes = %v, want bounded empty projection", taskError.Codes())
	}
}

func TestWaiterValidatesDependenciesAndInputs(t *testing.T) {
	t.Parallel()

	if _, err := NewWaiter(nil); !errors.Is(err, ErrInvalidWaiter) {
		t.Fatalf("NewWaiter(nil) error = %v", err)
	}
	var typedNil *nilTaskReader
	if _, err := NewWaiter(typedNil); !errors.Is(err, ErrInvalidWaiter) {
		t.Fatalf("NewWaiter(typed nil) error = %v", err)
	}
	readerError := errors.New("safe-reader-error")
	reader := readerFunc(func(context.Context, string) (Snapshot, error) {
		return Snapshot{}, readerError
	})
	waiter, err := NewWaiter(reader)
	if err != nil {
		t.Fatalf("NewWaiter() error = %v", err)
	}
	if _, err := waiter.Wait(nil, "task-id"); !errors.Is(err, ErrInvalidWait) { //nolint:staticcheck // nil-context rejection is the behavior under test.
		t.Fatalf("Wait(nil) error = %v", err)
	}
	if _, err := waiter.Wait(context.Background(), ""); !errors.Is(err, ErrInvalidWait) {
		t.Fatalf("Wait(empty ID) error = %v", err)
	}
	if _, err := waiter.Wait(context.Background(), "task-id"); !errors.Is(err, readerError) {
		t.Fatalf("Wait(reader error) = %v", err)
	}
}

func TestWaiterUnknownSleeperFailureIsRedacted(t *testing.T) {
	t.Parallel()

	reader := readerFunc(func(context.Context, string) (Snapshot, error) {
		return Snapshot{Status: StatusRunning}, nil
	})
	waiter, err := newWaiter(reader, func(context.Context, time.Duration) error {
		return errors.New("sleeper-error-secret-canary")
	})
	if err != nil {
		t.Fatalf("newWaiter() error = %v", err)
	}
	_, err = waiter.Wait(context.Background(), "task-id")
	if !errors.Is(err, ErrTaskWaitInterrupted) || strings.Contains(fmt.Sprintf("%+v", err), "sleeper-error-secret-canary") {
		t.Fatalf("Wait() error = %v, want redacted ErrTaskWaitInterrupted", err)
	}
}

type readerFunc func(context.Context, string) (Snapshot, error)

func (function readerFunc) Read(ctx context.Context, extID string) (Snapshot, error) {
	return function(ctx, extID)
}

type nilTaskReader struct{}

func (*nilTaskReader) Read(context.Context, string) (Snapshot, error) {
	return Snapshot{}, nil
}
