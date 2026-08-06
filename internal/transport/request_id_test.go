package transport

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/ioplane/terraform-provider-nutanix/internal/auth"
)

func TestRequestIDIsStableAcrossApprovedMutationAttempts(t *testing.T) {
	t.Parallel()

	fixedID, err := uuid.Parse("123e4567-e89b-42d3-a456-426614174000")
	if err != nil {
		t.Fatalf("parse fixed UUID: %v", err)
	}
	var (
		mu          sync.Mutex
		gotIDs      []string
		gotBodies   []string
		sourceCalls int
	)
	sink := &captureEventSink{}
	client := testClientWithSink(t, sink, roundTripFunc(func(request *http.Request) (*http.Response, error) {
		body, readErr := io.ReadAll(request.Body)
		if readErr != nil {
			t.Fatalf("read replayed body: %v", readErr)
		}
		mu.Lock()
		gotIDs = append(gotIDs, request.Header.Get(requestIDHeader))
		gotBodies = append(gotBodies, string(body))
		attempt := len(gotIDs)
		mu.Unlock()
		status := http.StatusServiceUnavailable
		if attempt == maximumAttempts {
			status = http.StatusOK
		}
		return responseWithBody(request, status, "response"), nil
	}))
	client.requestIDSource = func() (uuid.UUID, error) {
		sourceCalls++
		return fixedID, nil
	}
	client.jitterSource = func(time.Duration) time.Duration { return 0 }
	client.retrySleeper = zeroRetrySleeper(t)

	request := mustRetryRequest(t, RetryIdempotentMutation, true, true, []byte(`{"name":"stable"}`))
	response, err := client.Execute(context.Background(), request)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if response.StatusCode() != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.StatusCode())
	}
	if len(gotIDs) != maximumAttempts {
		t.Fatalf("attempts = %d, want %d", len(gotIDs), maximumAttempts)
	}
	if sourceCalls != 1 {
		t.Fatalf("request ID source calls = %d, want 1 per logical operation", sourceCalls)
	}
	events := sink.Events()
	if len(events) != maximumAttempts {
		t.Fatalf("attempt events = %d, want %d", len(events), maximumAttempts)
	}
	for index, event := range events {
		if event.attempt != index+1 || event.correlationID != fixedID.String() {
			t.Fatalf("attempt event %d = %#v, want stable request correlation ID", index+1, event)
		}
	}
	for index := range gotIDs {
		if gotIDs[index] != fixedID.String() {
			t.Fatalf("attempt %d request ID = %q, want %q", index+1, gotIDs[index], fixedID.String())
		}
		if gotBodies[index] != `{"name":"stable"}` {
			t.Fatalf("attempt %d body = %q, want recreated immutable JSON", index+1, gotBodies[index])
		}
	}
}

func TestRequestIDSourceUsesCanonicalRFC4122RandomUUID(t *testing.T) {
	t.Parallel()

	value, err := newRandomRequestID()
	if err != nil {
		t.Fatalf("newRandomRequestID() error = %v", err)
	}
	if value == uuid.Nil || value.Version() != uuid.Version(4) || value.Variant() != uuid.RFC4122 {
		t.Fatalf("generated UUID = %s, version %d, variant %d", value.String(), value.Version(), value.Variant())
	}
	canonical := value.String()
	parsed, err := uuid.Parse(canonical)
	if err != nil || parsed.String() != canonical {
		t.Fatalf("canonical UUID round trip = %s, %v", parsed.String(), err)
	}
}

func TestRequestIDsRemainIsolatedAcrossConcurrentLogicalOperations(t *testing.T) {
	t.Parallel()

	firstID, err := uuid.Parse("123e4567-e89b-42d3-a456-426614174000")
	if err != nil {
		t.Fatalf("parse first UUID: %v", err)
	}
	secondID, err := uuid.Parse("123e4567-e89b-42d3-a456-426614174001")
	if err != nil {
		t.Fatalf("parse second UUID: %v", err)
	}
	var (
		mu          sync.Mutex
		sourceIndex int
		attempts    = map[string][]string{}
	)
	client := testClientWithRoundTripper(t, auth.NewAPIKey("safe-api-key"), roundTripFunc(func(request *http.Request) (*http.Response, error) {
		logical := request.URL.Query().Get("logical")
		mu.Lock()
		attempts[logical] = append(attempts[logical], request.Header.Get(requestIDHeader))
		attempt := len(attempts[logical])
		mu.Unlock()
		status := http.StatusServiceUnavailable
		if attempt == 2 {
			status = http.StatusOK
		}
		return responseWithBody(request, status, "body"), nil
	}))
	client.requestIDSource = func() (uuid.UUID, error) {
		mu.Lock()
		defer mu.Unlock()
		values := []uuid.UUID{firstID, secondID}
		if sourceIndex >= len(values) {
			return uuid.Nil, errors.New("request ID source called more than once per logical operation")
		}
		value := values[sourceIndex]
		sourceIndex++
		return value, nil
	}
	client.jitterSource = func(time.Duration) time.Duration { return 0 }
	client.retrySleeper = func(ctx context.Context, delay time.Duration) error {
		if delay != 0 {
			return errors.New("unexpected nonzero test delay")
		}
		return ctx.Err()
	}

	requests := make([]Request, 0, 2)
	for _, logical := range []string{"first", "second"} {
		request, requestErr := NewRequest(RequestOptions{
			Operation:         "prism.concurrent_mutation",
			Method:            http.MethodPost,
			PathTemplate:      "/api/test",
			Query:             url.Values{"logical": {logical}},
			JSONBody:          []byte(`{}`),
			ExpectedStatuses:  []int{http.StatusOK},
			RetryClass:        RetryIdempotentMutation,
			RequestIDRequired: true,
			Replayable:        true,
		})
		if requestErr != nil {
			t.Fatalf("NewRequest(%s) error = %v", logical, requestErr)
		}
		requests = append(requests, request)
	}
	results := make(chan error, len(requests))
	for _, request := range requests {
		request := request
		go func() {
			_, executeErr := client.Execute(context.Background(), request)
			results <- executeErr
		}()
	}
	for range requests {
		if executeErr := <-results; executeErr != nil {
			t.Fatalf("concurrent Execute() error = %v", executeErr)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if sourceIndex != 2 {
		t.Fatalf("request ID source calls = %d, want 2", sourceIndex)
	}
	if len(attempts["first"]) != 2 || len(attempts["second"]) != 2 {
		t.Fatalf("logical attempts = %#v, want two each", attempts)
	}
	if attempts["first"][0] != attempts["first"][1] ||
		attempts["second"][0] != attempts["second"][1] ||
		attempts["first"][0] == attempts["second"][0] {
		t.Fatalf("logical request IDs were not stable and isolated: %#v", attempts)
	}
}

func TestRequestIDGenerationFailureStartsNoAttemptAndRedactsCause(t *testing.T) {
	t.Parallel()

	var calls int
	sink := &captureEventSink{}
	client := testClientWithSink(t, sink, roundTripFunc(func(request *http.Request) (*http.Response, error) {
		calls++
		return responseWithBody(request, http.StatusOK, "ok"), nil
	}))
	client.requestIDSource = func() (uuid.UUID, error) {
		return uuid.Nil, errors.New("uuid-generator-secret-canary")
	}
	request := mustRetryRequest(t, RetryIdempotentMutation, true, true, []byte(`{}`))
	_, err := client.Execute(context.Background(), request)
	if calls != 0 {
		t.Fatalf("RoundTrip calls = %d, want 0", calls)
	}
	if events := sink.Events(); len(events) != 0 {
		t.Fatalf("attempt events = %d, want 0 before RoundTripper", len(events))
	}
	var transportError *TransportError
	if !errors.As(err, &transportError) || !errors.Is(err, ErrRequestFailed) {
		t.Fatalf("Execute() error = %v, want safe TransportError", err)
	}
	assertErrorRenderingsRedacted(t, err, []string{"uuid-generator-secret-canary"})
}

func TestRequestIDRejectsInvalidGeneratorResultWithoutAttempt(t *testing.T) {
	t.Parallel()

	nonRandom, err := uuid.Parse("123e4567-e89b-12d3-a456-426614174000")
	if err != nil {
		t.Fatalf("parse non-random UUID: %v", err)
	}
	for _, test := range []struct {
		name  string
		value uuid.UUID
	}{
		{name: "nil UUID", value: uuid.Nil},
		{name: "non-random UUID", value: nonRandom},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			var calls int
			client := testClientWithRoundTripper(t, auth.NewAPIKey("safe-api-key"), roundTripFunc(func(request *http.Request) (*http.Response, error) {
				calls++
				return responseWithBody(request, http.StatusOK, "ok"), nil
			}))
			client.requestIDSource = func() (uuid.UUID, error) { return test.value, nil }
			_, executeErr := client.Execute(context.Background(), mustRetryRequest(t, RetryIdempotentMutation, true, true, nil))
			if calls != 0 || !errors.Is(executeErr, ErrRequestFailed) {
				t.Fatalf("Execute() = calls %d, error %v; want no attempt and ErrRequestFailed", calls, executeErr)
			}
		})
	}
}

func TestRetryPolicyNeverAddsRequestIDWithoutAllMutationEvidence(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		retryClass        RetryClass
		requestIDRequired bool
		replayable        bool
		wantCalls         int
	}{
		{name: "none", retryClass: RetryNone, requestIDRequired: false, replayable: true, wantCalls: 1},
		{name: "read", retryClass: RetryRead, requestIDRequired: false, replayable: true, wantCalls: maximumAttempts},
		{name: "mutation missing request ID evidence", retryClass: RetryIdempotentMutation, requestIDRequired: false, replayable: true, wantCalls: 1},
		{name: "mutation body not replayable", retryClass: RetryIdempotentMutation, requestIDRequired: true, replayable: false, wantCalls: 1},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			var calls int
			client := testClientWithRoundTripper(t, requestIDInjectingAuthorizer{}, roundTripFunc(func(request *http.Request) (*http.Response, error) {
				calls++
				if values := headerValuesFold(request.Header, requestIDHeader); len(values) != 0 {
					t.Fatalf("attempt request ID values = %v, want absent", values)
				}
				return responseWithBody(request, http.StatusServiceUnavailable, "retryable"), nil
			}))
			client.jitterSource = func(time.Duration) time.Duration { return 0 }
			client.retrySleeper = zeroRetrySleeper(t)
			request := mustRetryRequest(t, test.retryClass, test.requestIDRequired, test.replayable, []byte(`{}`))
			_, executeErr := client.Execute(context.Background(), request)
			var httpError *HTTPError
			if !errors.As(executeErr, &httpError) || httpError.StatusCode() != http.StatusServiceUnavailable {
				t.Fatalf("Execute() error = %v, want HTTP 503", executeErr)
			}
			if calls != test.wantCalls {
				t.Fatalf("RoundTrip calls = %d, want %d", calls, test.wantCalls)
			}
		})
	}
}

func TestRequestIDScrubsCaseVariantInjectionBeforeKernelValue(t *testing.T) {
	t.Parallel()

	fixedID, err := uuid.Parse("123e4567-e89b-42d3-a456-426614174000")
	if err != nil {
		t.Fatalf("parse fixed UUID: %v", err)
	}
	client := testClientWithRoundTripper(t, requestIDInjectingAuthorizer{}, roundTripFunc(func(request *http.Request) (*http.Response, error) {
		values := headerValuesFold(request.Header, requestIDHeader)
		if len(values) != 1 || values[0] != fixedID.String() {
			t.Fatalf("request ID variants = %v, want one kernel UUID", values)
		}
		return responseWithBody(request, http.StatusOK, "ok"), nil
	}))
	client.requestIDSource = func() (uuid.UUID, error) { return fixedID, nil }
	response, err := client.Execute(context.Background(), mustRetryRequest(t, RetryIdempotentMutation, true, true, []byte(`{}`)))
	if err != nil || response.StatusCode() != http.StatusOK {
		t.Fatalf("Execute() = %#v, %v", response, err)
	}
}

type requestIDInjectingAuthorizer struct{}

func (requestIDInjectingAuthorizer) Authorize(request *http.Request) {
	request.Header.Set("X-ntnx-api-key", "safe-api-key")
	request.Header["ntnx-request-id"] = []string{
		"request-id-injection-secret-canary-1",
		"request-id-injection-secret-canary-2",
	}
}

func mustRetryRequest(
	t *testing.T,
	retryClass RetryClass,
	requestIDRequired bool,
	replayable bool,
	body []byte,
) Request {
	t.Helper()
	request, err := NewRequest(RequestOptions{
		Operation:         "prism.retry_test",
		Method:            http.MethodPost,
		PathTemplate:      "/api/test",
		JSONBody:          body,
		ExpectedStatuses:  []int{http.StatusOK},
		RetryClass:        retryClass,
		RequestIDRequired: requestIDRequired,
		Replayable:        replayable,
	})
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	return request
}

func responseWithBody(request *http.Request, status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    request,
	}
}
