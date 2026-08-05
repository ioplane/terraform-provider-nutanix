package transport

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestLogAttemptUsesExactAllowlist(t *testing.T) {
	t.Parallel()

	event := attemptEvent{
		operation:     "prism.get_task",
		method:        http.MethodGet,
		pathTemplate:  "/api/prism/v4.3/config/tasks/{extId}",
		attempt:       1,
		status:        http.StatusOK,
		duration:      25 * time.Millisecond,
		correlationID: "123e4567-e89b-12d3-a456-426614174000",
	}
	fields := event.fields()
	wantKeys := []string{"attempt", "correlation_id", "duration", "method", "operation", "path_template", "status"}
	gotKeys := make([]string, 0, len(fields))
	for key := range fields {
		gotKeys = append(gotKeys, key)
	}
	slices.Sort(gotKeys)
	if !reflect.DeepEqual(gotKeys, wantKeys) {
		t.Fatalf("log fields = %v, want exact allowlist %v", gotKeys, wantKeys)
	}
	if fields["operation"] != event.operation || fields["path_template"] != event.pathTemplate {
		t.Fatalf("log fields lost locked policy metadata: %#v", fields)
	}
}

func TestLogAttemptOmitsAbsentCorrelationID(t *testing.T) {
	t.Parallel()

	fields := (attemptEvent{
		operation:    "prism.get_task",
		method:       http.MethodGet,
		pathTemplate: "/api/prism/v4.3/config/tasks/{extId}",
		attempt:      1,
		status:       0,
		duration:     time.Millisecond,
	}).fields()
	wantKeys := []string{"attempt", "duration", "method", "operation", "path_template", "status"}
	gotKeys := make([]string, 0, len(fields))
	for key := range fields {
		gotKeys = append(gotKeys, key)
	}
	slices.Sort(gotKeys)
	if !reflect.DeepEqual(gotKeys, wantKeys) {
		t.Fatalf("log fields without correlation = %v, want %v", gotKeys, wantKeys)
	}
	if fields["status"] != 0 {
		t.Fatalf("transport-failure status = %#v, want explicit 0", fields["status"])
	}
}

func TestLogAttemptNeverIncludesRequestOrVendorCanaries(t *testing.T) {
	t.Parallel()

	sink := &captureEventSink{}
	client := testClientWithSink(t, sink, roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusInternalServerError,
			Header: http.Header{
				"NTNX-Request-Id": {"invalid-correlation-secret-canary"},
				"X-Vendor-Object": {"vendor-header-secret-canary"},
			},
			Body:    io.NopCloser(strings.NewReader("vendor-response-secret-canary object-id-secret-canary")),
			Request: request,
		}, nil
	}))
	request, err := NewRequest(RequestOptions{
		Operation:      "prism.get_task",
		Method:         http.MethodPost,
		PathTemplate:   "/api/prism/v4.3/config/tasks/{extId}",
		PathParameters: map[string]string{"extId": "path-parameter-secret-canary"},
		Query:          map[string][]string{"filter": {"query-secret-canary"}},
		Headers:        http.Header{"X-Operation-Metadata": {"request-header-secret-canary"}},
		JSONBody:       []byte(`{"name":"request-body-secret-canary"}`),
		ExpectedStatuses: []int{
			http.StatusOK,
		},
	})
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	_, executeErr := client.Execute(context.Background(), request)
	if executeErr == nil {
		t.Fatal("Execute() error = nil, want HTTPError")
	}

	events := sink.Events()
	if len(events) != 1 {
		t.Fatalf("captured event count = %d, want 1", len(events))
	}
	for key, value := range events[0].fields() {
		rendered := fmt.Sprint(value)
		for _, canary := range allTransportCanaries() {
			if strings.Contains(rendered, canary) {
				t.Fatalf("log field %q leaked %q: %q", key, canary, rendered)
			}
		}
	}
	assertErrorRenderingsRedacted(t, executeErr, allTransportCanaries())
}

func TestLogProductionSinkIsSafeWithoutFrameworkLogger(t *testing.T) {
	t.Parallel()

	(tflogEventSink{}).EmitAttempt(context.Background(), attemptEvent{
		operation:    "prism.get_task",
		method:       http.MethodGet,
		pathTemplate: "/api/prism/v4.3/config/tasks/{extId}",
		attempt:      1,
	})
}

type captureEventSink struct {
	mu     sync.Mutex
	events []attemptEvent
}

func (s *captureEventSink) EmitAttempt(_ context.Context, event attemptEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, event)
}

func (s *captureEventSink) Events() []attemptEvent {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]attemptEvent(nil), s.events...)
}

func allTransportCanaries() []string {
	return []string{
		"api-key-secret-canary",
		"path-parameter-secret-canary",
		"query-secret-canary",
		"request-header-secret-canary",
		"request-body-secret-canary",
		"invalid-correlation-secret-canary",
		"vendor-header-secret-canary",
		"vendor-response-secret-canary",
		"object-id-secret-canary",
	}
}
