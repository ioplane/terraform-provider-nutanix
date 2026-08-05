package task

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestTaskStatusOutcomeFailsClosed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		status Status
		want   Outcome
	}{
		{StatusQueued, OutcomePending},
		{StatusRunning, OutcomePending},
		{StatusCanceling, OutcomePending},
		{StatusSuspended, OutcomePending},
		{StatusSucceeded, OutcomeSucceeded},
		{StatusFailed, OutcomeFailed},
		{StatusCanceled, OutcomeCanceled},
		{StatusUnknown, OutcomeUnsupported},
		{StatusRedacted, OutcomeUnsupported},
		{Status("FUTURE_STATUS"), OutcomeUnsupported},
		{Status(""), OutcomeUnsupported},
	}
	for _, test := range tests {
		test := test
		t.Run(string(test.status), func(t *testing.T) {
			t.Parallel()
			if got := test.status.Outcome(); got != test.want {
				t.Fatalf("Outcome() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestTaskSnapshotAndProjectionFormattingAreRedacted(t *testing.T) {
	t.Parallel()

	snapshot := Snapshot{
		ExtID:              "task-ext-id-secret-canary",
		Status:             StatusFailed,
		ProgressPercentage: 50,
		Errors: []Projection{{
			Code:          "safe-code",
			ErrorGroup:    "safe-group",
			Severity:      "ERROR",
			AttributePath: "attribute-path-secret-canary",
		}},
		LastUpdatedTime: time.Unix(1, 0),
	}
	for _, value := range []any{snapshot, snapshot.Errors[0]} {
		for _, rendered := range []string{
			fmt.Sprintf("%v", value),
			fmt.Sprintf("%+v", value),
			fmt.Sprintf("%#v", value),
		} {
			for _, canary := range []string{"task-ext-id-secret-canary", "attribute-path-secret-canary"} {
				if strings.Contains(rendered, canary) {
					t.Fatalf("formatted value leaked %q: %q", canary, rendered)
				}
			}
		}
	}
}
