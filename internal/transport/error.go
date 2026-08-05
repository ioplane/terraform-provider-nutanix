package transport

import (
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"strconv"
)

var (
	// ErrRequestFailed identifies a request attempt that failed for an unexposed cause.
	ErrRequestFailed = errors.New("transport request attempt failed")
	// ErrResponseRead identifies a response-body read failure.
	ErrResponseRead = errors.New("transport response body read failed")
	// ErrResponseTooLarge identifies a response larger than its approved ceiling.
	ErrResponseTooLarge = errors.New("transport response body exceeded its limit")
	// ErrResponseClose identifies a response-body close failure.
	ErrResponseClose = errors.New("transport response body close failed")
)

// TransportFailureKind classifies where an operation failed without exposing
// an underlying URL, request, response, or vendor value.
type TransportFailureKind string

const (
	TransportFailureRequest       TransportFailureKind = "request"
	TransportFailureRedirect      TransportFailureKind = "redirect"
	TransportFailureCanceled      TransportFailureKind = "canceled"
	TransportFailureDeadline      TransportFailureKind = "deadline"
	TransportFailureResponseRead  TransportFailureKind = "response_read"
	TransportFailureResponseLimit TransportFailureKind = "response_limit"
	TransportFailureResponseClose TransportFailureKind = "response_close"
)

// TransportError is a vendor-neutral operation failure.
type TransportError struct {
	operation string
	kind      TransportFailureKind
	cause     error
}

func newTransportError(operation string, kind TransportFailureKind, cause error) *TransportError {
	return &TransportError{
		operation: safeOperation(operation),
		kind:      kind,
		cause:     approvedTransportCause(cause),
	}
}

// Error returns a stable message containing no raw request or response data.
func (e *TransportError) Error() string {
	if e == nil {
		return "transport operation failed"
	}
	return fmt.Sprintf("transport operation %s failed (%s)", e.operation, e.kind)
}

// Format prevents verbose formatting from exposing private fields or causes.
func (e *TransportError) Format(state fmt.State, verb rune) {
	formatSafeText(state, verb, e.Error())
}

// Unwrap exposes only an approved stable cause.
func (e *TransportError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

// Operation returns the locked, non-secret operation name.
func (e *TransportError) Operation() string {
	if e == nil {
		return ""
	}
	return e.operation
}

// Kind returns the stable transport failure class.
func (e *TransportError) Kind() TransportFailureKind {
	if e == nil {
		return ""
	}
	return e.kind
}

// HTTPError is an unexpected HTTP status with a bounded body available only
// through an explicit copy-returning accessor for namespace decoding.
type HTTPError struct {
	operation     string
	statusCode    int
	correlationID string
	body          []byte
}

func newHTTPError(operation string, statusCode int, correlationID string, body []byte) *HTTPError {
	return &HTTPError{
		operation:     safeOperation(operation),
		statusCode:    statusCode,
		correlationID: validCorrelationIDOrEmpty(correlationID),
		body:          slices.Clone(body),
	}
}

// Error returns a stable vendor-neutral status message.
func (e *HTTPError) Error() string {
	if e == nil {
		return "transport operation returned an unexpected HTTP status"
	}
	return fmt.Sprintf("transport operation %s returned HTTP status %d", e.operation, e.statusCode)
}

// Format prevents verbose formatting from exposing the bounded vendor body.
func (e *HTTPError) Format(state fmt.State, verb rune) {
	formatSafeText(state, verb, e.Error())
}

// Operation returns the locked, non-secret operation name.
func (e *HTTPError) Operation() string {
	if e == nil {
		return ""
	}
	return e.operation
}

// StatusCode returns the unexpected HTTP status code.
func (e *HTTPError) StatusCode() int {
	if e == nil {
		return 0
	}
	return e.statusCode
}

// CorrelationID returns a validated UUID correlation identifier when present.
func (e *HTTPError) CorrelationID() string {
	if e == nil {
		return ""
	}
	return e.correlationID
}

// Body returns a copy of the bounded body for the owning namespace decoder.
func (e *HTTPError) Body() []byte {
	if e == nil {
		return nil
	}
	return slices.Clone(e.body)
}

func approvedTransportCause(cause error) error {
	for _, approved := range []error{
		context.Canceled,
		context.DeadlineExceeded,
		ErrRedirectRefused,
		ErrInvalidRequest,
		ErrRequestFailed,
		ErrResponseRead,
		ErrResponseTooLarge,
		ErrResponseClose,
	} {
		if errors.Is(cause, approved) {
			return approved
		}
	}
	return nil
}

func safeOperation(operation string) string {
	if validOperation(operation) {
		return operation
	}
	return "unknown"
}

func validCorrelationIDOrEmpty(value string) string {
	if len(value) != 36 || value[8] != '-' || value[13] != '-' || value[18] != '-' || value[23] != '-' {
		return ""
	}
	for index := 0; index < len(value); index++ {
		if index == 8 || index == 13 || index == 18 || index == 23 {
			continue
		}
		character := value[index]
		if (character < '0' || character > '9') &&
			(character < 'a' || character > 'f') &&
			(character < 'A' || character > 'F') {
			return ""
		}
	}
	return value
}

func formatSafeText(state fmt.State, verb rune, message string) {
	if verb == 'q' {
		_, _ = io.WriteString(state, strconv.Quote(message))
		return
	}
	_, _ = io.WriteString(state, message)
}
