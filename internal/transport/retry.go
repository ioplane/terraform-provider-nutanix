package transport

import (
	"context"
	"io"
	"math"
	rand "math/rand/v2"
	"net"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	maximumAttempts        = 4
	initialRetryBackoff    = 500 * time.Millisecond
	maximumRetryDelay      = 30 * time.Second
	maximumCumulativeDelay = 60 * time.Second
	maximumErrorGraphDepth = 64
	maximumErrorGraphNodes = 128
)

// RetryClass is an immutable namespace-owned operation retry contract.
type RetryClass uint8

const (
	// RetryNone forbids provider-managed replay.
	RetryNone RetryClass = iota
	// RetryRead permits approved read/list/task-poll failures to be replayed.
	RetryRead
	// RetryIdempotentMutation permits replay only with locked request-ID and body evidence.
	RetryIdempotentMutation
)

func (class RetryClass) valid() bool {
	return class == RetryNone || class == RetryRead || class == RetryIdempotentMutation
}

func (r Request) permitsRetry() bool {
	switch r.retryClass {
	case RetryRead:
		return true
	case RetryIdempotentMutation:
		return r.requestIDRequired && r.replayable
	default:
		return false
	}
}

func retryableHTTPStatus(status int) bool {
	switch status {
	case http.StatusRequestTimeout,
		http.StatusTooManyRequests,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

func classifyRetryableTransportError(ctx context.Context, err error) bool {
	if err == nil || ctx == nil || ctx.Err() != nil {
		return false
	}
	return analyzeErrorGraph(err).permitsRetry()
}

type errorIdentity struct {
	errorType reflect.Type
	value     error
}

type errorVisitIdentity struct {
	errorIdentity
	deadlineAllowed bool
}

type errorGraphAnalysis struct {
	nodes             int
	retryableLeaves   int
	leaves            int
	cycle             bool
	overflow          bool
	unwrapFailure     bool
	canonical         error
	canonicalConflict bool
	path              map[errorIdentity]struct{}
	completed         map[errorVisitIdentity]struct{}
}

func analyzeErrorGraph(err error) errorGraphAnalysis {
	analysis := errorGraphAnalysis{
		path:      make(map[errorIdentity]struct{}),
		completed: make(map[errorVisitIdentity]struct{}),
	}
	analysis.visit(err, 0, false)
	return analysis
}

func canonicalErrorCause(err error) error {
	return analyzeErrorGraph(err).canonicalCause()
}

func (analysis errorGraphAnalysis) permitsRetry() bool {
	return analysis.graphSafe() && analysis.leaves > 0 &&
		analysis.retryableLeaves == analysis.leaves
}

func (analysis errorGraphAnalysis) graphSafe() bool {
	return !analysis.cycle && !analysis.overflow && !analysis.unwrapFailure
}

func (analysis errorGraphAnalysis) canonicalCause() error {
	if !analysis.graphSafe() || analysis.leaves == 0 ||
		analysis.canonical == nil || analysis.canonicalConflict {
		return nil
	}
	return analysis.canonical
}

func (analysis *errorGraphAnalysis) visit(err error, depth int, deadlineAllowed bool) {
	if depth >= maximumErrorGraphDepth || analysis.nodes >= maximumErrorGraphNodes {
		analysis.overflow = true
		return
	}
	if err == nil {
		analysis.recordLeaf(false, nil)
		return
	}

	errorType := reflect.TypeOf(err)
	errorValue := reflect.ValueOf(err)
	if isNilErrorValue(errorValue) {
		analysis.recordLeaf(false, nil)
		return
	}

	identity, identifiable := comparableErrorIdentity(err, errorValue)
	if identifiable {
		if _, cyclic := analysis.path[identity]; cyclic {
			analysis.cycle = true
			return
		}
		visitIdentity := errorVisitIdentity{
			errorIdentity:   identity,
			deadlineAllowed: deadlineAllowed,
		}
		if _, complete := analysis.completed[visitIdentity]; complete {
			return
		}
		analysis.path[identity] = struct{}{}
		defer func() {
			delete(analysis.path, identity)
			analysis.completed[visitIdentity] = struct{}{}
		}()
	}

	analysis.nodes++

	baseType := errorType
	for baseType.Kind() == reflect.Pointer {
		baseType = baseType.Elem()
	}
	if packagePath := baseType.PkgPath(); packagePath == "crypto/tls" || packagePath == "crypto/x509" {
		analysis.recordLeaf(false, nil)
		return
	}

	switch {
	case sameKnownError(err, context.Canceled):
		analysis.recordLeaf(false, context.Canceled)
		return
	case sameKnownError(err, context.DeadlineExceeded):
		analysis.recordLeaf(deadlineAllowed, context.DeadlineExceeded)
		return
	case sameKnownError(err, ErrRedirectRefused):
		analysis.recordLeaf(false, ErrRedirectRefused)
		return
	case sameKnownError(err, ErrInvalidRequest):
		analysis.recordLeaf(false, ErrInvalidRequest)
		return
	case sameKnownError(err, ErrRequestFailed):
		analysis.recordLeaf(false, ErrRequestFailed)
		return
	case sameKnownError(err, ErrResponseRead):
		analysis.recordLeaf(false, ErrResponseRead)
		return
	case sameKnownError(err, ErrResponseTooLarge):
		analysis.recordLeaf(false, ErrResponseTooLarge)
		return
	case sameKnownError(err, ErrResponseClose):
		analysis.recordLeaf(false, ErrResponseClose)
		return
	case sameKnownError(err, errResponseWithError),
		sameKnownError(err, ErrInvalidAuthorizationState):
		analysis.recordLeaf(false, ErrRequestFailed)
		return
	case sameKnownError(err, io.EOF), sameKnownError(err, io.ErrUnexpectedEOF):
		analysis.recordLeaf(true, nil)
		return
	}
	if errno, ok := err.(syscall.Errno); ok {
		switch errno {
		case syscall.ECONNRESET, syscall.ECONNREFUSED, syscall.EPIPE, syscall.ETIMEDOUT:
			analysis.recordLeaf(true, nil)
		default:
			analysis.recordLeaf(false, nil)
		}
		return
	}

	switch typed := err.(type) {
	case *url.Error:
		if typed.Err == nil {
			analysis.recordLeaf(false, nil)
			return
		}
		analysis.visit(
			typed.Err,
			depth+1,
			deadlineAllowed || sameKnownError(typed.Err, context.DeadlineExceeded),
		)
		return
	case *net.OpError:
		if typed.Err == nil {
			analysis.recordLeaf(false, nil)
			return
		}
		analysis.visit(
			typed.Err,
			depth+1,
			deadlineAllowed || sameKnownError(typed.Err, context.DeadlineExceeded),
		)
		return
	case *net.DNSError:
		retryable := !typed.IsNotFound && (typed.IsTimeout || typed.IsTemporary)
		analysis.recordLeaf(retryable, nil)
		if typed.UnwrapErr != nil {
			analysis.visit(typed.UnwrapErr, depth+1, deadlineAllowed || typed.IsTimeout)
		}
		return
	case *os.SyscallError:
		if typed.Err == nil {
			analysis.recordLeaf(false, nil)
			return
		}
		analysis.visit(typed.Err, depth+1, deadlineAllowed)
		return
	case interface{ Unwrap() []error }:
		children, ok := safeUnwrapMany(typed)
		if !ok {
			analysis.unwrapFailure = true
			return
		}
		if len(children) == 0 {
			analysis.recordLeaf(false, nil)
			return
		}
		if len(children) > maximumErrorGraphNodes-analysis.nodes {
			analysis.overflow = true
			return
		}
		for _, child := range children {
			analysis.visit(child, depth+1, deadlineAllowed)
			if !analysis.graphSafe() {
				return
			}
		}
		return
	case interface{ Unwrap() error }:
		child, ok := safeUnwrapOne(typed)
		if !ok {
			analysis.unwrapFailure = true
			return
		}
		if child == nil {
			analysis.recordLeaf(false, nil)
			return
		}
		analysis.visit(child, depth+1, deadlineAllowed)
		return
	case net.Error:
		timeout, temporary, ok := safeNetworkErrorFlags(typed)
		if !ok {
			analysis.unwrapFailure = true
			return
		}
		analysis.recordLeaf(timeout || temporary, nil)
		return
	default:
		analysis.recordLeaf(false, nil)
		return
	}
}

func (analysis *errorGraphAnalysis) recordLeaf(retryable bool, canonical error) {
	analysis.leaves++
	if retryable {
		analysis.retryableLeaves++
	}
	if canonical == nil {
		analysis.canonicalConflict = true
		return
	}
	if analysis.canonical == nil {
		analysis.canonical = canonical
		return
	}
	if analysis.canonical != canonical {
		analysis.canonicalConflict = true
	}
}

func comparableErrorIdentity(err error, errorValue reflect.Value) (errorIdentity, bool) {
	if !errorValue.IsValid() || !errorValue.Comparable() {
		return errorIdentity{}, false
	}
	return errorIdentity{errorType: errorValue.Type(), value: err}, true
}

func isNilErrorValue(value reflect.Value) bool {
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

func sameKnownError(err, target error) (same bool) {
	if err == nil || target == nil {
		return err == nil && target == nil
	}
	errValue := reflect.ValueOf(err)
	targetValue := reflect.ValueOf(target)
	if errValue.Type() != targetValue.Type() ||
		!errValue.Comparable() || !targetValue.Comparable() {
		return false
	}
	defer func() {
		if recover() != nil {
			same = false
		}
	}()
	return errValue.Equal(targetValue)
}

func safeUnwrapOne(wrapper interface{ Unwrap() error }) (child error, ok bool) {
	ok = true
	defer func() {
		if recover() != nil {
			child = nil
			ok = false
		}
	}()
	return wrapper.Unwrap(), true
}

func safeUnwrapMany(wrapper interface{ Unwrap() []error }) (children []error, ok bool) {
	ok = true
	defer func() {
		if recover() != nil {
			children = nil
			ok = false
		}
	}()
	return wrapper.Unwrap(), true
}

func safeNetworkErrorFlags(networkError net.Error) (timeout, temporary, ok bool) {
	ok = true
	defer func() {
		if recover() != nil {
			timeout = false
			temporary = false
			ok = false
		}
	}()
	timeout = networkError.Timeout()
	// Temporary is intentionally part of the approved fail-closed classifier.
	temporary = networkError.Temporary() //nolint:staticcheck
	return timeout, temporary, true
}

func parseRetryAfter(value string, now time.Time) (time.Duration, bool) {
	trimmed := strings.Trim(value, " \t")
	if trimmed == "" {
		return 0, false
	}
	if allDecimalDigits(trimmed) {
		seconds, err := strconv.ParseUint(trimmed, 10, 64)
		if err != nil || seconds > uint64(math.MaxInt64/int64(time.Second)) {
			return 0, false
		}
		return time.Duration(seconds) * time.Second, true
	}
	parsed, err := http.ParseTime(trimmed)
	if err != nil || !parsed.After(now) {
		return 0, false
	}
	if parsed.After(now.Add(time.Duration(math.MaxInt64))) {
		return 0, false
	}
	return parsed.Sub(now), true
}

func allDecimalDigits(value string) bool {
	for index := 0; index < len(value); index++ {
		if value[index] < '0' || value[index] > '9' {
			return false
		}
	}
	return value != ""
}

type retryBudget struct {
	cumulativeDelay time.Duration
}

type jitterSource func(time.Duration) time.Duration

type retrySleeper func(context.Context, time.Duration) error

func fullJitter(ceiling time.Duration) time.Duration {
	if ceiling <= 0 {
		return 0
	}
	return time.Duration(rand.Int64N(int64(ceiling)))
}

func sleepForRetry(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func retryBackoffCeiling(completedAttempt int) time.Duration {
	if completedAttempt < 1 {
		return 0
	}
	ceiling := initialRetryBackoff
	for index := 1; index < completedAttempt && ceiling < maximumRetryDelay; index++ {
		if ceiling > maximumRetryDelay/2 {
			return maximumRetryDelay
		}
		ceiling *= 2
	}
	return min(ceiling, maximumRetryDelay)
}

func (budget *retryBudget) nextDelay(
	ctx context.Context,
	completedAttempt int,
	retryAfter string,
	now time.Time,
	jitter jitterSource,
) (time.Duration, bool) {
	if budget == nil || ctx == nil || ctx.Err() != nil || jitter == nil {
		return 0, false
	}
	ceiling := retryBackoffCeiling(completedAttempt)
	delay := jitter(ceiling)
	if ctx.Err() != nil || delay < 0 || delay > ceiling {
		return 0, false
	}
	if serverDelay, ok := parseRetryAfter(retryAfter, now); ok && serverDelay > delay {
		delay = serverDelay
	}
	if delay > maximumRetryDelay || delay > maximumCumulativeDelay-budget.cumulativeDelay {
		return 0, false
	}
	if deadline, ok := ctx.Deadline(); ok {
		remaining := deadline.Sub(now)
		if remaining <= 0 || delay >= remaining {
			return 0, false
		}
	}
	budget.cumulativeDelay += delay
	return delay, true
}
