package transport

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"testing/synctest"
	"time"

	"github.com/ioplane/terraform-provider-nutanix/internal/auth"
)

func TestClassifyRetryableTransportErrorFailClosed(t *testing.T) {
	t.Parallel()

	temporaryDNS := &net.DNSError{Err: "temporary-dns-secret", Name: "name-secret", IsTemporary: true}
	timeoutDNS := &net.DNSError{Err: "timeout-dns-secret", Name: "name-secret", IsTimeout: true}
	notFoundDNS := &net.DNSError{Err: "nxdomain-secret", Name: "name-secret", IsTemporary: true, IsNotFound: true}
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "EOF", err: io.EOF, want: true},
		{name: "unexpected EOF", err: io.ErrUnexpectedEOF, want: true},
		{name: "connection reset", err: syscall.ECONNRESET, want: true},
		{name: "connection refused", err: syscall.ECONNREFUSED, want: true},
		{name: "broken pipe", err: syscall.EPIPE, want: true},
		{name: "timed out", err: syscall.ETIMEDOUT, want: true},
		{name: "temporary DNS", err: temporaryDNS, want: true},
		{name: "timeout DNS", err: timeoutDNS, want: true},
		{name: "wrapped URL and operation", err: &url.Error{Op: "Get", URL: "https://raw-url-secret", Err: &net.OpError{Op: "dial", Net: "tcp", Err: syscall.ECONNRESET}}, want: true},
		{name: "active context timeout net error", err: classifiedNetError{timeout: true}, want: true},
		{name: "active context temporary net error", err: classifiedNetError{temporary: true}, want: true},
		{name: "caller canceled", err: context.Canceled},
		{name: "caller deadline", err: context.DeadlineExceeded},
		{name: "wrapped caller canceled", err: fmt.Errorf("safe wrapper: %w", context.Canceled)},
		{name: "wrapped caller deadline", err: fmt.Errorf("safe wrapper: %w", context.DeadlineExceeded)},
		{name: "joined caller canceled and reset", err: errors.Join(context.Canceled, syscall.ECONNRESET)},
		{name: "joined caller deadline and reset", err: errors.Join(context.DeadlineExceeded, syscall.ECONNRESET)},
		{name: "NXDOMAIN even temporary", err: notFoundDNS},
		{name: "non temporary DNS", err: &net.DNSError{Err: "permanent", Name: "name-secret"}},
		{name: "x509 unknown authority", err: x509.UnknownAuthorityError{}},
		{name: "x509 hostname", err: x509.HostnameError{}},
		{name: "TLS record header", err: tls.RecordHeaderError{Msg: "tls-secret"}},
		{name: "TLS alert", err: tls.AlertError(40)},
		{name: "malformed request", err: ErrInvalidRequest},
		{name: "redirect", err: ErrRedirectRefused},
		{name: "response read", err: ErrResponseRead},
		{name: "response limit", err: ErrResponseTooLarge},
		{name: "response close", err: ErrResponseClose},
		{name: "unknown", err: errors.New("unknown-secret")},
		{name: "future HTTP2", err: futureHTTP2Error{}},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := classifyRetryableTransportError(context.Background(), test.err); got != test.want {
				t.Fatalf("classifyRetryableTransportError(%T) = %t, want %t", test.err, got, test.want)
			}
		})
	}
}

func TestClassifyRetryableTransportErrorRejectsInactiveCallerContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if classifyRetryableTransportError(ctx, syscall.ECONNRESET) {
		t.Fatal("classifier accepted failure after caller cancellation")
	}
	ctx, cancel = context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()
	<-ctx.Done()
	if classifyRetryableTransportError(ctx, classifiedNetError{timeout: true}) {
		t.Fatal("classifier accepted timeout after caller deadline")
	}
}

func TestClassifyRetryableTransportErrorBoundsCyclicGraphs(t *testing.T) {
	t.Parallel()

	self := &cyclicUnwrapError{}
	self.next = self
	first := &cyclicUnwrapError{}
	second := &cyclicUnwrapError{next: first}
	first.next = second
	urlCycle := &url.Error{Op: "Get", URL: "https://url-secret"}
	urlCycle.Err = urlCycle
	operationCycle := &net.OpError{Op: "read", Net: "tcp"}
	operationCycle.Err = operationCycle
	for _, test := range []struct {
		name string
		err  error
	}{
		{name: "self cycle", err: self},
		{name: "two node cycle", err: first},
		{name: "URL cycle", err: urlCycle},
		{name: "operation cycle", err: operationCycle},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if classifyWithTimeout(t, test.err) {
				t.Fatal("cyclic error graph was classified retryable")
			}
		})
	}
}

func TestClassifyRetryableTransportErrorFindsDeepTLSAndBoundsOverflow(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		err  error
	}{
		{name: "TLS", err: tls.RecordHeaderError{Msg: "deep-tls-secret"}},
		{name: "x509", err: x509.UnknownAuthorityError{}},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			failure := test.err
			for index := 0; index < 40; index++ {
				failure = &singleUnwrapError{next: failure}
			}
			if classifyWithTimeout(t, &timeoutWrappingError{next: failure}) {
				t.Fatalf("deep %s failure hidden by timeout wrapper was classified retryable", test.name)
			}
		})
	}

	overflow := error(io.EOF)
	for index := 0; index < 256; index++ {
		overflow = &timeoutWrappingError{next: overflow}
	}
	if classifyWithTimeout(t, overflow) {
		t.Fatal("error graph beyond the hard traversal ceiling was classified retryable")
	}
}

func TestClassifyRetryableTransportErrorChecksEveryJoinedBranch(t *testing.T) {
	t.Parallel()

	unknown := errors.New("joined-unknown-secret")
	for _, joined := range []error{
		errors.Join(syscall.ECONNRESET, unknown),
		errors.Join(unknown, syscall.ECONNRESET),
	} {
		if classifyWithTimeout(t, joined) {
			t.Fatal("mixed approved and unknown joined graph was classified retryable")
		}
	}
	for _, joined := range []error{
		errors.Join(io.EOF, syscall.ECONNRESET),
		errors.Join(syscall.ECONNRESET, io.EOF),
		errors.Join(io.EOF, io.EOF),
	} {
		if !classifyWithTimeout(t, joined) {
			t.Fatal("joined graph containing only approved failures was rejected")
		}
	}
}

func TestClassifyRetryableTransportErrorRequiresStructuralTimeoutForDeadline(t *testing.T) {
	t.Parallel()

	wrappedDeadline := fmt.Errorf("neutral wrapper: %w", context.DeadlineExceeded)
	for _, failure := range []error{
		&url.Error{Op: "Get", URL: "https://url-secret", Err: wrappedDeadline},
		&net.OpError{Op: "read", Net: "tcp", Err: wrappedDeadline},
	} {
		if classifyWithTimeout(t, failure) {
			t.Fatalf("non-timeout %T wrapper allowed a nested deadline", failure)
		}
	}
	for _, failure := range []error{
		&url.Error{Op: "Get", URL: "https://url-secret", Err: context.DeadlineExceeded},
		&net.OpError{Op: "read", Net: "tcp", Err: context.DeadlineExceeded},
	} {
		if !classifyWithTimeout(t, failure) {
			t.Fatalf("active structural timeout %T was rejected", failure)
		}
	}
}

func TestAnalyzeErrorGraphBoundsWideUnwrap(t *testing.T) {
	t.Parallel()

	children := make([]error, maximumErrorGraphNodes*4)
	for index := range children {
		children[index] = io.EOF
	}
	analysis := analyzeErrorGraph(manyUnwrapError{children: children})
	if !analysis.overflow {
		t.Fatal("wide error graph was not marked overflow")
	}
	if analysis.nodes > maximumErrorGraphNodes {
		t.Fatalf("visited nodes = %d, want at most %d", analysis.nodes, maximumErrorGraphNodes)
	}
	if analysis.permitsRetry() {
		t.Fatal("wide error graph beyond the node ceiling was classified retryable")
	}
}

func TestClassifyRetryableTransportErrorRejectsDynamicallyUncomparableValue(t *testing.T) {
	t.Parallel()

	failure := dynamicallyUncomparableError{payload: []byte("dynamic-payload-secret")}
	if classifyWithTimeout(t, failure) {
		t.Fatal("dynamically uncomparable value error was classified retryable")
	}
}

func TestSameKnownErrorRejectsDynamicallyUncomparableValuesWithoutPanic(t *testing.T) {
	t.Parallel()

	left := dynamicallyUncomparableError{payload: []byte("left-payload-secret")}
	right := dynamicallyUncomparableError{payload: []byte("right-payload-secret")}
	var (
		equal    bool
		panicked bool
	)
	func() {
		defer func() {
			panicked = recover() != nil
		}()
		equal = sameKnownError(left, right)
	}()
	if panicked {
		t.Fatal("sameKnownError() panicked on dynamically uncomparable values")
	}
	if equal {
		t.Fatal("dynamically uncomparable values were reported equal")
	}
}

func TestRetryableHTTPStatusAllowlist(t *testing.T) {
	t.Parallel()

	for status := 100; status <= 599; status++ {
		want := status == http.StatusRequestTimeout || status == http.StatusTooManyRequests ||
			status == http.StatusBadGateway || status == http.StatusServiceUnavailable ||
			status == http.StatusGatewayTimeout
		if got := retryableHTTPStatus(status); got != want {
			t.Fatalf("retryableHTTPStatus(%d) = %t, want %t", status, got, want)
		}
	}
}

func TestRetryRequestPolicyValidation(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name              string
		retryClass        RetryClass
		requestIDRequired bool
	}{
		{name: "future retry class", retryClass: RetryClass(255)},
		{name: "read cannot require mutation ID", retryClass: RetryRead, requestIDRequired: true},
		{name: "none cannot require mutation ID", retryClass: RetryNone, requestIDRequired: true},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			request, err := NewRequest(RequestOptions{
				Operation:         "prism.retry_policy",
				Method:            http.MethodGet,
				PathTemplate:      "/api/test",
				ExpectedStatuses:  []int{http.StatusOK},
				RetryClass:        test.retryClass,
				RequestIDRequired: test.requestIDRequired,
			})
			if request.valid || !errors.Is(err, ErrInvalidRequest) {
				t.Fatalf("NewRequest() = %#v, %v; want fail-closed ErrInvalidRequest", request, err)
			}
		})
	}
}

func TestRetryTransportFailureRequiresExplicitPolicy(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name       string
		method     string
		retryClass RetryClass
		failure    error
		wantCalls  int
	}{
		{name: "GET none does not infer retry", method: http.MethodGet, retryClass: RetryNone, failure: syscall.ECONNRESET, wantCalls: 1},
		{name: "POST read policy retries", method: http.MethodPost, retryClass: RetryRead, failure: syscall.ECONNRESET, wantCalls: maximumAttempts},
		{name: "read rejects TLS", method: http.MethodGet, retryClass: RetryRead, failure: tls.RecordHeaderError{Msg: "tls-secret"}, wantCalls: 1},
		{name: "read rejects unknown", method: http.MethodGet, retryClass: RetryRead, failure: futureHTTP2Error{}, wantCalls: 1},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			var calls int
			client := testClientWithRoundTripper(t, auth.NewAPIKey("safe-api-key"), roundTripFunc(func(*http.Request) (*http.Response, error) {
				calls++
				return nil, test.failure
			}))
			client.jitterSource = func(time.Duration) time.Duration { return 0 }
			client.retrySleeper = zeroRetrySleeper(t)
			request, err := NewRequest(RequestOptions{
				Operation:        "prism.transport_retry",
				Method:           test.method,
				PathTemplate:     "/api/test",
				ExpectedStatuses: []int{http.StatusOK},
				RetryClass:       test.retryClass,
			})
			if err != nil {
				t.Fatalf("NewRequest() error = %v", err)
			}
			_, executeErr := client.Execute(context.Background(), request)
			if calls != test.wantCalls {
				t.Fatalf("calls = %d, want %d", calls, test.wantCalls)
			}
			if executeErr == nil {
				t.Fatal("Execute() error = nil, want transport failure")
			}
			assertErrorRenderingsRedacted(t, executeErr, []string{
				"tls-secret",
				"future-http2-secret",
			})
		})
	}
}

func TestRetryUsesFourAttemptsFullJitterAndOneEventPerAttempt(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		sink := &captureEventSink{}
		var calls int
		client := testClientWithSink(t, sink, roundTripFunc(func(request *http.Request) (*http.Response, error) {
			calls++
			status := http.StatusServiceUnavailable
			if calls == maximumAttempts {
				status = http.StatusOK
			}
			return responseWithBody(request, status, "attempt-body-secret-canary"), nil
		}))
		var ceilings []time.Duration
		client.jitterSource = func(ceiling time.Duration) time.Duration {
			ceilings = append(ceilings, ceiling)
			return ceiling
		}
		start := time.Now()
		response, err := client.Execute(context.Background(), mustRetryRequest(t, RetryRead, false, false, nil))
		if err != nil || response.StatusCode() != http.StatusOK {
			t.Fatalf("Execute() = %#v, %v", response, err)
		}
		if got, want := time.Since(start), 3500*time.Millisecond; got != want {
			t.Fatalf("provider delay = %s, want %s", got, want)
		}
		wantCeilings := []time.Duration{500 * time.Millisecond, time.Second, 2 * time.Second}
		if len(ceilings) != len(wantCeilings) {
			t.Fatalf("jitter calls = %v, want %v", ceilings, wantCeilings)
		}
		for index := range wantCeilings {
			if ceilings[index] != wantCeilings[index] {
				t.Fatalf("jitter ceiling %d = %s, want %s", index, ceilings[index], wantCeilings[index])
			}
		}
		events := sink.Events()
		if len(events) != maximumAttempts {
			t.Fatalf("events = %d, want %d", len(events), maximumAttempts)
		}
		for index, event := range events {
			if event.attempt != index+1 || event.operation != "prism.retry_test" {
				t.Fatalf("event %d = %#v", index, event)
			}
			for key, value := range event.fields() {
				if rendered := fmt.Sprint(value); strings.Contains(rendered, "attempt-body-secret-canary") {
					t.Fatalf("event field %s leaked response body: %q", key, rendered)
				}
			}
		}
	})
}

func TestRetryLoggingCountsOnlyReturnedRoundTripperCalls(t *testing.T) {
	t.Parallel()

	sink := &captureEventSink{}
	var calls int
	client := testClientWithSink(t, sink, roundTripFunc(func(request *http.Request) (*http.Response, error) {
		calls++
		return responseWithBody(request, http.StatusOK, "body"), nil
	}))
	_, err := client.Execute(context.Background(), Request{})
	if !errors.Is(err, ErrInvalidRequest) || calls != 0 || len(sink.Events()) != 0 {
		t.Fatalf("invalid request = error %v calls/events %d/%d; want 0 actual attempts", err, calls, len(sink.Events()))
	}
}

func TestRetryAfterDeltaAndHTTPDateAreServerMinimums(t *testing.T) {
	tests := []struct {
		name       string
		header     func(time.Time) string
		clientWait time.Duration
		wantWait   time.Duration
	}{
		{name: "delta greater", header: func(time.Time) string { return "2" }, clientWait: 500 * time.Millisecond, wantWait: 2 * time.Second},
		{name: "date greater", header: func(now time.Time) string { return now.Add(3 * time.Second).Format(http.TimeFormat) }, clientWait: 500 * time.Millisecond, wantWait: 3 * time.Second},
		{name: "client greater", header: func(time.Time) string { return "0" }, clientWait: 500 * time.Millisecond, wantWait: 500 * time.Millisecond},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				var calls int
				client := testClientWithRoundTripper(t, auth.NewAPIKey("safe-api-key"), roundTripFunc(func(request *http.Request) (*http.Response, error) {
					calls++
					status := http.StatusServiceUnavailable
					if calls == 2 {
						status = http.StatusOK
					}
					response := responseWithBody(request, status, "body")
					if status != http.StatusOK {
						response.Header.Set("Retry-After", test.header(time.Now()))
					}
					return response, nil
				}))
				client.jitterSource = func(time.Duration) time.Duration { return test.clientWait }
				start := time.Now()
				_, err := client.Execute(context.Background(), mustRetryRequest(t, RetryRead, false, false, nil))
				if err != nil {
					t.Fatalf("Execute() error = %v", err)
				}
				if got := time.Since(start); got != test.wantWait {
					t.Fatalf("delay = %s, want %s", got, test.wantWait)
				}
			})
		})
	}
}

func TestRetryAfterAmbiguousOrMalformedValueFallsBackToJitter(t *testing.T) {
	for _, test := range []struct {
		name   string
		header http.Header
	}{
		{name: "duplicate canonical", header: http.Header{"Retry-After": {"10", "20"}}},
		{name: "duplicate case variant", header: http.Header{"Retry-After": {"10"}, "retry-after": {"20"}}},
		{name: "malformed", header: http.Header{"Retry-After": {"1.5"}}},
		{name: "overflow", header: http.Header{"Retry-After": {"9223372036854775808"}}},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				var calls int
				client := testClientWithRoundTripper(t, auth.NewAPIKey("safe-api-key"), roundTripFunc(func(request *http.Request) (*http.Response, error) {
					calls++
					status := http.StatusServiceUnavailable
					if calls == 2 {
						status = http.StatusOK
					}
					response := responseWithBody(request, status, "body")
					response.Header = test.header.Clone()
					return response, nil
				}))
				client.jitterSource = func(time.Duration) time.Duration { return 250 * time.Millisecond }
				start := time.Now()
				_, err := client.Execute(context.Background(), mustRetryRequest(t, RetryRead, false, false, nil))
				if err != nil {
					t.Fatalf("Execute() error = %v", err)
				}
				if got := time.Since(start); got != 250*time.Millisecond {
					t.Fatalf("fallback delay = %s, want 250ms", got)
				}
			})
		})
	}
}

func TestParseRetryAfterValidAndFallbackInputs(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.August, 5, 12, 0, 0, 0, time.UTC)
	future := now.Add(17 * time.Second).Format(http.TimeFormat)
	past := now.Add(-time.Second).Format(http.TimeFormat)
	tests := []struct {
		name  string
		value string
		want  time.Duration
		ok    bool
	}{
		{name: "zero", value: "0", want: 0, ok: true},
		{name: "delta", value: "17", want: 17 * time.Second, ok: true},
		{name: "outer whitespace", value: " \t17\t ", want: 17 * time.Second, ok: true},
		{name: "future HTTP date", value: future, want: 17 * time.Second, ok: true},
		{name: "empty"},
		{name: "whitespace only", value: " \t "},
		{name: "non HTTP whitespace", value: "\u00a017\u00a0"},
		{name: "signed positive", value: "+17"},
		{name: "signed negative", value: "-17"},
		{name: "fraction", value: "1.5"},
		{name: "embedded whitespace", value: "1 7"},
		{name: "past date", value: past},
		{name: "overflow delta", value: "9223372036854775808"},
		{name: "duration overflow delta", value: "9223372037"},
		{name: "duration overflow date", value: "Fri, 31 Dec 9999 23:59:59 GMT"},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			got, ok := parseRetryAfter(test.value, now)
			if got != test.want || ok != test.ok {
				t.Fatalf("parseRetryAfter(%q) = %s, %t; want %s, %t", test.value, got, ok, test.want, test.ok)
			}
		})
	}
}

func TestRetryBudgetCeilingsFailClosed(t *testing.T) {
	tests := []struct {
		name       string
		retryAfter func(int) string
		wantCalls  int
		wantDelay  time.Duration
	}{
		{name: "per delay over 30 seconds", retryAfter: func(int) string { return "31" }, wantCalls: 1},
		{name: "cumulative over 60 seconds", retryAfter: func(attempt int) string {
			if attempt < 3 {
				return "30"
			}
			return "1"
		}, wantCalls: 3, wantDelay: 60 * time.Second},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				var calls int
				client := testClientWithRoundTripper(t, auth.NewAPIKey("safe-api-key"), roundTripFunc(func(request *http.Request) (*http.Response, error) {
					calls++
					response := responseWithBody(request, http.StatusServiceUnavailable, "body")
					response.Header.Set("Retry-After", test.retryAfter(calls))
					return response, nil
				}))
				client.jitterSource = func(time.Duration) time.Duration { return 0 }
				start := time.Now()
				_, err := client.Execute(context.Background(), mustRetryRequest(t, RetryRead, false, false, nil))
				var httpError *HTTPError
				if !errors.As(err, &httpError) {
					t.Fatalf("Execute() error = %v, want HTTPError", err)
				}
				if calls != test.wantCalls || time.Since(start) != test.wantDelay {
					t.Fatalf("calls/delay = %d/%s, want %d/%s", calls, time.Since(start), test.wantCalls, test.wantDelay)
				}
			})
		})
	}
}

func TestRetryRejectsInvalidInjectedJitterWithoutSleeping(t *testing.T) {
	t.Parallel()

	for _, invalid := range []time.Duration{-time.Nanosecond, initialRetryBackoff + time.Nanosecond} {
		invalid := invalid
		t.Run(invalid.String(), func(t *testing.T) {
			t.Parallel()
			var calls, sleeps int
			client := testClientWithRoundTripper(t, auth.NewAPIKey("safe-api-key"), roundTripFunc(func(request *http.Request) (*http.Response, error) {
				calls++
				return responseWithBody(request, http.StatusServiceUnavailable, "body"), nil
			}))
			client.jitterSource = func(time.Duration) time.Duration { return invalid }
			client.retrySleeper = func(context.Context, time.Duration) error {
				sleeps++
				return nil
			}
			_, _ = client.Execute(context.Background(), mustRetryRequest(t, RetryRead, false, false, nil))
			if calls != 1 || sleeps != 0 {
				t.Fatalf("calls/sleeps = %d/%d, want 1/0", calls, sleeps)
			}
		})
	}
}

func TestRetryDoesNotStartAtOrBeyondCallerDeadline(t *testing.T) {
	tests := []struct {
		name     string
		deadline time.Duration
	}{
		{name: "exact boundary", deadline: 500 * time.Millisecond},
		{name: "beyond boundary", deadline: 499 * time.Millisecond},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				var calls int
				client := testClientWithRoundTripper(t, auth.NewAPIKey("safe-api-key"), roundTripFunc(func(request *http.Request) (*http.Response, error) {
					calls++
					return responseWithBody(request, http.StatusServiceUnavailable, "body"), nil
				}))
				client.jitterSource = func(time.Duration) time.Duration { return 500 * time.Millisecond }
				ctx, cancel := context.WithTimeout(context.Background(), test.deadline)
				defer cancel()
				_, err := client.Execute(ctx, mustRetryRequest(t, RetryRead, false, false, nil))
				var httpError *HTTPError
				if !errors.As(err, &httpError) {
					t.Fatalf("Execute() error = %v, want current HTTPError", err)
				}
				if calls != 1 {
					t.Fatalf("calls = %d, want 1", calls)
				}
			})
		})
	}
}

func TestRetryCancellationDuringDelayReturnsCallerCause(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var calls int
		client := testClientWithRoundTripper(t, auth.NewAPIKey("safe-api-key"), roundTripFunc(func(request *http.Request) (*http.Response, error) {
			calls++
			return responseWithBody(request, http.StatusServiceUnavailable, "body"), nil
		}))
		client.jitterSource = func(time.Duration) time.Duration { return 500 * time.Millisecond }
		ctx, cancel := context.WithCancel(context.Background())
		result := make(chan error, 1)
		go func() {
			_, err := client.Execute(ctx, mustRetryRequest(t, RetryRead, false, false, nil))
			result <- err
		}()
		time.Sleep(100 * time.Millisecond)
		cancel()
		err := <-result
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Execute() error = %v, want context.Canceled", err)
		}
		if calls != 1 {
			t.Fatalf("calls = %d, want 1", calls)
		}
	})
}

func TestRetryCancellationDuringDelaySelectionReturnsCallerCause(t *testing.T) {
	t.Parallel()

	var calls, sleeps int
	client := testClientWithRoundTripper(t, auth.NewAPIKey("safe-api-key"), roundTripFunc(func(request *http.Request) (*http.Response, error) {
		calls++
		return responseWithBody(request, http.StatusServiceUnavailable, "body"), nil
	}))
	ctx, cancel := context.WithCancel(context.Background())
	client.jitterSource = func(time.Duration) time.Duration {
		cancel()
		return 0
	}
	client.retrySleeper = func(context.Context, time.Duration) error {
		sleeps++
		return nil
	}
	_, err := client.Execute(ctx, mustRetryRequest(t, RetryRead, false, false, nil))
	if !errors.Is(err, context.Canceled) || calls != 1 || sleeps != 0 {
		t.Fatalf("Execute() = error %v calls/sleeps %d/%d; want cancellation and 1/0", err, calls, sleeps)
	}
}

func TestRetryNoDeadlineIsBoundedByAttemptTimeoutAndCeiling(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var calls int
		client := testClientWithRoundTripper(t, auth.NewAPIKey("safe-api-key"), roundTripFunc(func(request *http.Request) (*http.Response, error) {
			calls++
			<-request.Context().Done()
			return nil, request.Context().Err()
		}))
		client.httpClient.Timeout = time.Second
		client.jitterSource = func(time.Duration) time.Duration { return 0 }
		start := time.Now()
		_, err := client.Execute(context.Background(), mustRetryRequest(t, RetryRead, false, false, nil))
		if calls != maximumAttempts {
			t.Fatalf("calls = %d, want %d", calls, maximumAttempts)
		}
		if got := time.Since(start); got != time.Duration(maximumAttempts)*time.Second {
			t.Fatalf("network bound = %s, want %s", got, time.Duration(maximumAttempts)*time.Second)
		}
		if !errors.Is(err, ErrRequestFailed) {
			t.Fatalf("Execute() error = %v, want safe ErrRequestFailed", err)
		}
	})
}

func TestRetryClosesEachResponseBeforeDelayAndNeverRetriesBodyFailures(t *testing.T) {
	t.Parallel()

	var (
		calls  int
		bodies []*trackedSizedBody
	)
	client := testClientWithRoundTripper(t, auth.NewAPIKey("safe-api-key"), roundTripFunc(func(request *http.Request) (*http.Response, error) {
		calls++
		body := &trackedSizedBody{remaining: 4}
		bodies = append(bodies, body)
		return &http.Response{StatusCode: http.StatusServiceUnavailable, Header: make(http.Header), Body: body, Request: request}, nil
	}))
	client.jitterSource = func(time.Duration) time.Duration {
		if !bodies[len(bodies)-1].closed.Load() {
			t.Fatal("retry delay selected before response body close")
		}
		return 0
	}
	client.retrySleeper = zeroRetrySleeper(t)
	_, err := client.Execute(context.Background(), mustRetryRequest(t, RetryRead, false, false, nil))
	if calls != maximumAttempts {
		t.Fatalf("calls = %d, want %d", calls, maximumAttempts)
	}
	var httpError *HTTPError
	if !errors.As(err, &httpError) {
		t.Fatalf("Execute() error = %v, want HTTPError", err)
	}
	for index, body := range bodies {
		if !body.closed.Load() {
			t.Fatalf("attempt %d body not closed", index+1)
		}
	}

	failingBody := &failingTrackedBody{readErr: syscall.ECONNRESET}
	client = testClientWithRoundTripper(t, auth.NewAPIKey("safe-api-key"), roundTripFunc(func(request *http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: http.StatusServiceUnavailable, Header: make(http.Header), Body: failingBody, Request: request}, nil
	}))
	calls = 0
	_, err = client.Execute(context.Background(), mustRetryRequest(t, RetryRead, false, false, nil))
	if calls != 1 || !errors.Is(err, ErrResponseRead) || !failingBody.closed.Load() {
		t.Fatalf("body failure = calls %d closed %t error %v; want no retry", calls, failingBody.closed.Load(), err)
	}
}

func TestRetryRejectsForbiddenStatusesAndRedirectResponseError(t *testing.T) {
	t.Parallel()

	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound, http.StatusConflict, http.StatusPreconditionFailed, http.StatusPreconditionRequired} {
		status := status
		t.Run(http.StatusText(status), func(t *testing.T) {
			t.Parallel()
			var calls int
			client := testClientWithRoundTripper(t, auth.NewAPIKey("safe-api-key"), roundTripFunc(func(request *http.Request) (*http.Response, error) {
				calls++
				return responseWithBody(request, status, "forbidden-status-secret"), nil
			}))
			_, err := client.Execute(context.Background(), mustRetryRequest(t, RetryRead, false, false, nil))
			if calls != 1 {
				t.Fatalf("calls = %d, want 1", calls)
			}
			var httpError *HTTPError
			if !errors.As(err, &httpError) || httpError.StatusCode() != status {
				t.Fatalf("Execute() error = %v, want HTTP %d", err, status)
			}
		})
	}

	body := &countingCloseBody{}
	var calls int
	client := testClientWithRoundTripper(t, auth.NewAPIKey("safe-api-key"), roundTripFunc(func(request *http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: http.StatusFound, Header: http.Header{"Location": {"/redirect"}}, Body: body, Request: request}, ErrRedirectRefused
	}))
	_, err := client.Execute(context.Background(), mustRetryRequest(t, RetryRead, false, false, nil))
	if calls != 1 || body.closes.Load() != 1 || !errors.Is(err, ErrRedirectRefused) {
		t.Fatalf("redirect = calls %d closes %d error %v", calls, body.closes.Load(), err)
	}
}

func TestRetryRejectsTransportErrorAfterResponseWasReturned(t *testing.T) {
	t.Parallel()

	body := &countingCloseBody{}
	var calls int
	client := testClientWithRoundTripper(t, auth.NewAPIKey("safe-api-key"), roundTripFunc(func(request *http.Request) (*http.Response, error) {
		calls++
		return &http.Response{
			StatusCode: http.StatusServiceUnavailable,
			Header:     make(http.Header),
			Body:       body,
			Request:    request,
		}, syscall.ECONNRESET
	}))
	client.jitterSource = func(time.Duration) time.Duration { return 0 }
	client.retrySleeper = zeroRetrySleeper(t)
	_, err := client.Execute(context.Background(), mustRetryRequest(t, RetryRead, false, false, nil))
	if calls != 1 || body.closes.Load() != 1 || !errors.Is(err, ErrRequestFailed) {
		t.Fatalf("response+error = calls %d closes %d error %v; want fail closed", calls, body.closes.Load(), err)
	}
}

func TestRetryRejectsCyclicRoundTripperErrorPromptly(t *testing.T) {
	t.Parallel()

	cycle := &cyclicUnwrapError{}
	cycle.next = cycle
	var calls atomic.Int32
	sink := &captureEventSink{}
	client := testClientWithSink(t, sink, roundTripFunc(func(*http.Request) (*http.Response, error) {
		calls.Add(1)
		return nil, cycle
	}))

	_, err := executeWithTimeout(t, client, mustRetryRequest(t, RetryRead, false, false, nil))
	if !errors.Is(err, ErrRequestFailed) {
		t.Fatalf("Execute() error = %v, want ErrRequestFailed", err)
	}
	if calls.Load() != 1 || len(sink.Events()) != 1 {
		t.Fatalf("calls/events = %d/%d, want 1/1", calls.Load(), len(sink.Events()))
	}
}

func TestRetryRejectsCyclicResponseReadErrorPromptly(t *testing.T) {
	t.Parallel()

	cycle := &cyclicUnwrapError{}
	cycle.next = cycle
	body := &cyclicReadBody{readErr: cycle}
	var calls atomic.Int32
	sink := &captureEventSink{}
	client := testClientWithSink(t, sink, roundTripFunc(func(request *http.Request) (*http.Response, error) {
		calls.Add(1)
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       body,
			Request:    request,
		}, nil
	}))

	_, err := executeWithTimeout(t, client, mustRetryRequest(t, RetryRead, false, false, nil))
	if !errors.Is(err, ErrResponseRead) {
		t.Fatalf("Execute() error = %v, want ErrResponseRead", err)
	}
	if calls.Load() != 1 || len(sink.Events()) != 1 || body.closes.Load() != 1 {
		t.Fatalf(
			"calls/events/closes = %d/%d/%d, want 1/1/1",
			calls.Load(),
			len(sink.Events()),
			body.closes.Load(),
		)
	}
}

func TestRetryRejectsDynamicallyUncomparableRoundTripperErrorPromptly(t *testing.T) {
	t.Parallel()

	failure := dynamicallyUncomparableError{payload: []byte("round-trip-payload-secret")}
	var calls atomic.Int32
	sink := &captureEventSink{}
	client := testClientWithSink(t, sink, roundTripFunc(func(*http.Request) (*http.Response, error) {
		calls.Add(1)
		return nil, failure
	}))

	_, err := executeWithTimeout(t, client, mustRetryRequest(t, RetryRead, false, false, nil))
	if !errors.Is(err, ErrRequestFailed) {
		t.Fatalf("Execute() error = %v, want ErrRequestFailed", err)
	}
	if calls.Load() != 1 || len(sink.Events()) != 1 {
		t.Fatalf("calls/events = %d/%d, want 1/1", calls.Load(), len(sink.Events()))
	}
	assertErrorRenderingsRedacted(t, err, []string{"round-trip-payload-secret"})
}

func TestRetryRejectsDynamicallyUncomparableResponseReadErrorPromptly(t *testing.T) {
	t.Parallel()

	failure := dynamicallyUncomparableError{payload: []byte("body-read-payload-secret")}
	body := &cyclicReadBody{readErr: failure}
	var calls atomic.Int32
	sink := &captureEventSink{}
	client := testClientWithSink(t, sink, roundTripFunc(func(request *http.Request) (*http.Response, error) {
		calls.Add(1)
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       body,
			Request:    request,
		}, nil
	}))

	_, err := executeWithTimeout(t, client, mustRetryRequest(t, RetryRead, false, false, nil))
	if !errors.Is(err, ErrResponseRead) {
		t.Fatalf("Execute() error = %v, want ErrResponseRead", err)
	}
	if calls.Load() != 1 || len(sink.Events()) != 1 || body.closes.Load() != 1 {
		t.Fatalf(
			"calls/events/closes = %d/%d/%d, want 1/1/1",
			calls.Load(),
			len(sink.Events()),
			body.closes.Load(),
		)
	}
	assertErrorRenderingsRedacted(t, err, []string{"body-read-payload-secret"})
}

type classifiedNetError struct {
	timeout   bool
	temporary bool
}

func (e classifiedNetError) Error() string   { return "classified-network-secret" }
func (e classifiedNetError) Timeout() bool   { return e.timeout }
func (e classifiedNetError) Temporary() bool { return e.temporary }

type futureHTTP2Error struct{}

func (futureHTTP2Error) Error() string   { return "future-http2-secret" }
func (futureHTTP2Error) Timeout() bool   { return false }
func (futureHTTP2Error) Temporary() bool { return false }

type cyclicUnwrapError struct {
	next error
}

func (*cyclicUnwrapError) Error() string { return "cyclic-error-secret" }

func (e *cyclicUnwrapError) Unwrap() error { return e.next }

type singleUnwrapError struct {
	next error
}

func (*singleUnwrapError) Error() string { return "single-wrapper-secret" }

func (e *singleUnwrapError) Unwrap() error { return e.next }

type timeoutWrappingError struct {
	next error
}

func (*timeoutWrappingError) Error() string   { return "timeout-wrapper-secret" }
func (*timeoutWrappingError) Timeout() bool   { return true }
func (*timeoutWrappingError) Temporary() bool { return false }

func (e *timeoutWrappingError) Unwrap() error { return e.next }

type manyUnwrapError struct {
	children []error
}

func (manyUnwrapError) Error() string { return "many-wrapper-secret" }

func (e manyUnwrapError) Unwrap() []error { return e.children }

type dynamicallyUncomparableError struct {
	payload any
}

func (dynamicallyUncomparableError) Error() string { return "dynamically-uncomparable-secret" }

func classifyWithTimeout(t *testing.T, err error) bool {
	t.Helper()
	type classificationResult struct {
		retryable bool
		panic     any
	}
	results := make(chan classificationResult, 1)
	go func() {
		completed := classificationResult{}
		defer func() {
			completed.panic = recover()
			results <- completed
		}()
		completed.retryable = classifyRetryableTransportError(context.Background(), err)
	}()
	select {
	case completed := <-results:
		if completed.panic != nil {
			t.Fatalf("classifier panicked: %v", completed.panic)
		}
		return completed.retryable
	case <-time.After(500 * time.Millisecond):
		t.Fatal("classifier did not return within bounded time")
		return false
	}
}

func executeWithTimeout(t *testing.T, client *Client, request Request) (Response, error) {
	t.Helper()
	type result struct {
		response Response
		err      error
		panic    any
	}
	results := make(chan result, 1)
	go func() {
		completed := result{}
		defer func() {
			completed.panic = recover()
			results <- completed
		}()
		completed.response, completed.err = client.Execute(context.Background(), request)
	}()
	select {
	case completed := <-results:
		if completed.panic != nil {
			t.Fatalf("Execute() panicked: %v", completed.panic)
		}
		return completed.response, completed.err
	case <-time.After(500 * time.Millisecond):
		t.Fatal("Execute() did not return within bounded time")
		return Response{}, nil
	}
}

type cyclicReadBody struct {
	readErr error
	closes  atomic.Int32
}

func (body *cyclicReadBody) Read([]byte) (int, error) { return 0, body.readErr }

func (body *cyclicReadBody) Close() error {
	body.closes.Add(1)
	return nil
}

type countingCloseBody struct {
	closes atomic.Int32
}

func (*countingCloseBody) Read([]byte) (int, error) { return 0, io.EOF }

func (b *countingCloseBody) Close() error {
	b.closes.Add(1)
	return nil
}

func zeroRetrySleeper(t *testing.T) retrySleeper {
	t.Helper()
	return func(ctx context.Context, delay time.Duration) error {
		if delay != 0 {
			t.Fatalf("unexpected real retry delay %s", delay)
		}
		return ctx.Err()
	}
}
