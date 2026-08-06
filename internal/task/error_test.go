package task

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestTaskErrorIsTypedStableAndRedacted(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		kind      FailureKind
		cause     error
		wantIs    error
		wantCodes []string
	}{
		{name: "failed", kind: FailureFailed, cause: ErrTaskFailed, wantIs: ErrTaskFailed, wantCodes: []string{"TASK-1", "TASK-2"}},
		{name: "remote canceled", kind: FailureCanceled, cause: ErrTaskCanceled, wantIs: ErrTaskCanceled},
		{name: "unsupported", kind: FailureUnsupported, cause: ErrTaskUnsupported, wantIs: ErrTaskUnsupported},
		{name: "caller canceled", kind: FailureCanceled, cause: context.Canceled, wantIs: context.Canceled},
		{name: "deadline", kind: FailureDeadline, cause: context.DeadlineExceeded, wantIs: context.DeadlineExceeded},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			codes := append([]string(nil), test.wantCodes...)
			err := newTaskError(test.kind, codes, test.cause)
			if len(codes) > 0 {
				codes[0] = "mutation-secret-canary"
			}

			var taskError *TaskError
			if !errors.As(err, &taskError) || taskError.Kind() != test.kind || !errors.Is(err, test.wantIs) {
				t.Fatalf("TaskError kind/cause = %v/%v", taskError, test.wantIs)
			}
			gotCodes := taskError.Codes()
			if fmt.Sprint(gotCodes) != fmt.Sprint(test.wantCodes) {
				t.Fatalf("Codes() = %v, want %v", gotCodes, test.wantCodes)
			}
			if len(gotCodes) > 0 {
				gotCodes[0] = "returned-slice-secret-canary"
				if fmt.Sprint(taskError.Codes()) != fmt.Sprint(test.wantCodes) {
					t.Fatal("Codes() exposed mutable internal state")
				}
			}
			for _, rendered := range []string{err.Error(), fmt.Sprintf("%v", err), fmt.Sprintf("%+v", err), fmt.Sprintf("%#v", err)} {
				for _, canary := range []string{"mutation-secret-canary", "returned-slice-secret-canary"} {
					if strings.Contains(rendered, canary) {
						t.Fatalf("TaskError leaked %q: %q", canary, rendered)
					}
				}
			}
		})
	}
}

func TestTaskErrorRejectsUnapprovedCause(t *testing.T) {
	t.Parallel()

	err := newTaskError(FailureFailed, []string{"SAFE"}, errors.New("vendor-message-secret-canary"))
	if errors.Unwrap(err) != nil {
		t.Fatalf("Unwrap() = %v, want nil for unapproved cause", errors.Unwrap(err))
	}
	for _, rendered := range []string{err.Error(), fmt.Sprintf("%+v", err), fmt.Sprintf("%#v", err)} {
		if strings.Contains(rendered, "vendor-message-secret-canary") {
			t.Fatalf("TaskError leaked raw cause: %q", rendered)
		}
	}
}
