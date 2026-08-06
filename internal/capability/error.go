package capability

import (
	"context"
	"errors"
	"fmt"
	"reflect"
)

var (
	// ErrCapabilityUnsupported identifies an explicit documented negative probe.
	ErrCapabilityUnsupported = errors.New("capability is unsupported")
	// ErrCapabilityIndeterminate identifies a probe result that cannot be cached.
	ErrCapabilityIndeterminate = errors.New("capability support is indeterminate")
	// ErrCapabilityAuthentication identifies an authentication probe failure.
	ErrCapabilityAuthentication = errors.New("capability probe authentication failed")
	// ErrCapabilityAuthorization identifies an authorization probe failure.
	ErrCapabilityAuthorization = errors.New("capability probe authorization failed")
	// ErrCapabilityRequestTimeout identifies an HTTP 408-style probe failure.
	ErrCapabilityRequestTimeout = errors.New("capability probe request timed out")
	// ErrCapabilityThrottled identifies a throttled capability probe.
	ErrCapabilityThrottled = errors.New("capability probe was throttled")
	// ErrCapabilityServer identifies an indeterminate server-side probe failure.
	ErrCapabilityServer = errors.New("capability probe server failure")
)

// FailureKind is a stable capability failure class.
type FailureKind string

const (
	FailureUnsupported   FailureKind = "unsupported"
	FailureIndeterminate FailureKind = "indeterminate"
)

// CapabilityError is a stable, secret-safe unsupported or indeterminate result.
type CapabilityError struct {
	capability string
	kind       FailureKind
	cause      error
}

// Error returns a stable message containing only the validated capability name.
func (e *CapabilityError) Error() string {
	if e == nil {
		return "capability check failed"
	}
	if e.capability == "" {
		return fmt.Sprintf("capability check failed (%s)", e.kind)
	}
	return fmt.Sprintf("capability %s check failed (%s)", e.capability, e.kind)
}

// Format prevents private fields and wrapped values from entering diagnostics.
func (e *CapabilityError) Format(state fmt.State, _ rune) {
	_, _ = state.Write([]byte(e.Error()))
}

// Unwrap exposes only an approved stable or caller-context cause.
func (e *CapabilityError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

// Kind returns the stable capability failure class.
func (e *CapabilityError) Kind() FailureKind {
	if e == nil {
		return ""
	}
	return e.kind
}

// Capability returns the validated capability name.
func (e *CapabilityError) Capability() string {
	if e == nil {
		return ""
	}
	return e.capability
}

func newCapabilityError(capability string, kind FailureKind, cause error) *CapabilityError {
	if !validCapabilityName(capability) {
		return &CapabilityError{
			kind:  FailureIndeterminate,
			cause: ErrCapabilityIndeterminate,
		}
	}
	if kind == FailureUnsupported && exactError(cause, ErrCapabilityUnsupported) {
		return &CapabilityError{
			capability: capability,
			kind:       FailureUnsupported,
			cause:      ErrCapabilityUnsupported,
		}
	}
	if kind != FailureIndeterminate || exactError(cause, ErrCapabilityUnsupported) {
		return &CapabilityError{
			capability: capability,
			kind:       FailureIndeterminate,
			cause:      ErrCapabilityIndeterminate,
		}
	}
	return &CapabilityError{
		capability: capability,
		kind:       FailureIndeterminate,
		cause:      approvedCapabilityCause(cause),
	}
}

func approvedCapabilityCause(cause error) error {
	// Probe errors are untrusted: do not invoke Is or Unwrap while classifying them.
	if cause == nil || !reflect.TypeOf(cause).Comparable() {
		return ErrCapabilityIndeterminate
	}
	switch cause {
	case ErrCapabilityIndeterminate,
		ErrCapabilityAuthentication,
		ErrCapabilityAuthorization,
		ErrCapabilityRequestTimeout,
		ErrCapabilityThrottled,
		ErrCapabilityServer,
		context.Canceled,
		context.DeadlineExceeded:
		return cause
	default:
		return ErrCapabilityIndeterminate
	}
}

func exactError(cause error, target error) bool {
	return cause != nil && reflect.TypeOf(cause).Comparable() && cause == target
}
