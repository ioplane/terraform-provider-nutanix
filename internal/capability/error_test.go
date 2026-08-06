package capability

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestCapabilityErrorIsTypedStableAndRedacted(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		kind   FailureKind
		cause  error
		wantIs error
	}{
		{name: "unsupported", kind: FailureUnsupported, cause: ErrCapabilityUnsupported, wantIs: ErrCapabilityUnsupported},
		{name: "indeterminate", kind: FailureIndeterminate, cause: ErrCapabilityIndeterminate, wantIs: ErrCapabilityIndeterminate},
		{name: "authentication", kind: FailureIndeterminate, cause: ErrCapabilityAuthentication, wantIs: ErrCapabilityAuthentication},
		{name: "authorization", kind: FailureIndeterminate, cause: ErrCapabilityAuthorization, wantIs: ErrCapabilityAuthorization},
		{name: "request timeout", kind: FailureIndeterminate, cause: ErrCapabilityRequestTimeout, wantIs: ErrCapabilityRequestTimeout},
		{name: "throttled", kind: FailureIndeterminate, cause: ErrCapabilityThrottled, wantIs: ErrCapabilityThrottled},
		{name: "server", kind: FailureIndeterminate, cause: ErrCapabilityServer, wantIs: ErrCapabilityServer},
		{name: "canceled", kind: FailureIndeterminate, cause: context.Canceled, wantIs: context.Canceled},
		{name: "deadline", kind: FailureIndeterminate, cause: context.DeadlineExceeded, wantIs: context.DeadlineExceeded},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			err := newCapabilityError("pc_2024_3", test.kind, test.cause)
			var capabilityError *CapabilityError
			if !errors.As(err, &capabilityError) || capabilityError.Kind() != test.kind ||
				capabilityError.Capability() != "pc_2024_3" || !errors.Is(err, test.wantIs) {
				t.Fatalf("CapabilityError = %v, want kind=%q cause=%v", err, test.kind, test.wantIs)
			}
			for _, rendered := range []string{err.Error(), fmt.Sprintf("%v", err), fmt.Sprintf("%+v", err), fmt.Sprintf("%#v", err)} {
				if strings.Contains(rendered, "capability-secret-canary") {
					t.Fatalf("CapabilityError leaked canary: %q", rendered)
				}
			}
		})
	}
}

func TestCapabilityErrorRejectsUnapprovedCause(t *testing.T) {
	t.Parallel()

	err := newCapabilityError("safe", FailureIndeterminate, errors.New("capability-secret-canary"))
	if !errors.Is(err, ErrCapabilityIndeterminate) || errors.Unwrap(err) == nil ||
		strings.Contains(fmt.Sprintf("%+v", err), "capability-secret-canary") {
		t.Fatalf("CapabilityError cause/redaction = %+v", err)
	}
}

func TestCapabilityErrorRejectsWrappedCauseWithoutTraversal(t *testing.T) {
	t.Parallel()

	const canary = "capability-wrapped-secret-canary"
	err := newCapabilityError(
		"safe",
		FailureIndeterminate,
		fmt.Errorf("%s: %w", canary, ErrCapabilityAuthentication),
	)
	if !errors.Is(err, ErrCapabilityIndeterminate) || errors.Is(err, ErrCapabilityAuthentication) {
		t.Fatalf("CapabilityError cause = %v, want generic indeterminate", errors.Unwrap(err))
	}
	if strings.Contains(fmt.Sprintf("%+v", err), canary) ||
		strings.Contains(fmt.Sprintf("%#v", err), canary) {
		t.Fatalf("CapabilityError leaked wrapped cause: %+v", err)
	}
}

func TestCapabilityErrorDoesNotInvokeHostileCauseMethods(t *testing.T) {
	t.Parallel()

	const canary = "capability-hostile-secret-canary"
	err := newCapabilityError("safe", FailureIndeterminate, &hostileCapabilityCause{canary: canary})
	if !errors.Is(err, ErrCapabilityIndeterminate) {
		t.Fatalf("CapabilityError cause = %v, want generic indeterminate", errors.Unwrap(err))
	}
	if strings.Contains(fmt.Sprintf("%+v", err), canary) {
		t.Fatalf("CapabilityError leaked hostile cause: %+v", err)
	}

	nonComparable := nonComparableCapabilityCause{canary}
	err = newCapabilityError("safe", FailureIndeterminate, nonComparable)
	if !errors.Is(err, ErrCapabilityIndeterminate) {
		t.Fatalf("non-comparable CapabilityError cause = %v, want generic indeterminate", errors.Unwrap(err))
	}
}

func TestCapabilityErrorInvalidStateFailsClosed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		kind  FailureKind
		cause error
	}{
		{name: "unknown kind", kind: FailureKind("unknown"), cause: ErrCapabilityUnsupported},
		{name: "unsupported with non-unsupported cause", kind: FailureUnsupported, cause: ErrCapabilityAuthorization},
		{name: "indeterminate with unsupported cause", kind: FailureIndeterminate, cause: ErrCapabilityUnsupported},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			err := newCapabilityError("safe", test.kind, test.cause)
			if err.Kind() != FailureIndeterminate || !errors.Is(err, ErrCapabilityIndeterminate) {
				t.Fatalf("CapabilityError kind/cause = %q/%v, want indeterminate", err.Kind(), errors.Unwrap(err))
			}
			if errors.Is(err, ErrCapabilityUnsupported) {
				t.Fatal("invalid CapabilityError state was reported as explicit unsupported")
			}
		})
	}
}

func TestCapabilityErrorInvalidCapabilityFailsClosedAndRedacts(t *testing.T) {
	t.Parallel()

	const canary = "BAD capability-secret-canary"
	err := newCapabilityError(canary, FailureUnsupported, ErrCapabilityUnsupported)
	if err.Capability() != "" || err.Kind() != FailureIndeterminate ||
		!errors.Is(err, ErrCapabilityIndeterminate) {
		t.Fatalf("invalid CapabilityError fields = %q/%q/%v", err.Capability(), err.Kind(), errors.Unwrap(err))
	}
	for _, rendered := range []string{err.Error(), fmt.Sprintf("%v", err), fmt.Sprintf("%+v", err), fmt.Sprintf("%#v", err)} {
		if strings.Contains(rendered, canary) {
			t.Fatalf("invalid CapabilityError leaked canary: %q", rendered)
		}
	}
}

type hostileCapabilityCause struct {
	canary string
}

func (cause *hostileCapabilityCause) Error() string { return cause.canary }

func (*hostileCapabilityCause) Is(error) bool { panic("hostile Is invoked") }

func (*hostileCapabilityCause) Unwrap() error { panic("hostile Unwrap invoked") }

type nonComparableCapabilityCause []string

func (cause nonComparableCapabilityCause) Error() string { return strings.Join(cause, "") }
