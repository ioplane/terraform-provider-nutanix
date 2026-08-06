package prism

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ioplane/terraform-provider-nutanix/internal/auth"
	ntnxtask "github.com/ioplane/terraform-provider-nutanix/internal/task"
	"github.com/ioplane/terraform-provider-nutanix/internal/transport"
)

const lockedTaskSelect = "extId,status,progressPercentage,entitiesAffected,errorMessages,warnings,completionDetails,lastUpdatedTime"

func TestGetTaskByIDUsesLockedPrismV43WireContract(t *testing.T) {
	t.Parallel()

	extID := "YmFzZTY0L3Rhc2s+=:1234"
	var calls atomic.Int32
	reader, closeServer := testReader(t, func(request *http.Request) (int, string) {
		calls.Add(1)
		if request.Method != http.MethodGet {
			t.Fatalf("method = %q, want GET", request.Method)
		}
		wantEscapedPath := "/api/prism/v4.3/config/tasks/" + url.PathEscape(extID)
		if request.URL.EscapedPath() != wantEscapedPath {
			t.Fatalf("escaped path = %q, want %q", request.URL.EscapedPath(), wantEscapedPath)
		}
		if request.URL.RawQuery != "%24select="+url.QueryEscape(lockedTaskSelect) {
			t.Fatalf("raw query = %q", request.URL.RawQuery)
		}
		if got := request.URL.Query(); len(got) != 1 || got.Get("$select") != lockedTaskSelect {
			t.Fatalf("query = %v", got)
		}
		if values := request.Header.Values("NTNX-Request-Id"); len(values) != 0 {
			t.Fatalf("request ID values = %v, want none", values)
		}
		return http.StatusOK, taskEnvelope(extID, "SUCCEEDED", nil, nil, nil)
	})
	defer closeServer()

	snapshot, err := reader.Read(context.Background(), extID)
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if calls.Load() != 1 || snapshot.ExtID != extID || snapshot.Status != ntnxtask.StatusSucceeded {
		t.Fatalf("Read() calls=%d snapshot=%#v", calls.Load(), snapshot)
	}
}

func TestGetTaskByIDMapsEveryLockedStatusAndFailsFutureClosed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		remote string
		want   ntnxtask.Status
	}{
		{"QUEUED", ntnxtask.StatusQueued},
		{"RUNNING", ntnxtask.StatusRunning},
		{"CANCELING", ntnxtask.StatusCanceling},
		{"SUSPENDED", ntnxtask.StatusSuspended},
		{"SUCCEEDED", ntnxtask.StatusSucceeded},
		{"FAILED", ntnxtask.StatusFailed},
		{"CANCELED", ntnxtask.StatusCanceled},
		{"$UNKNOWN", ntnxtask.StatusUnknown},
		{"$REDACTED", ntnxtask.StatusRedacted},
		{"FUTURE_STATUS", ntnxtask.StatusUnknown},
	}
	for _, test := range tests {
		test := test
		t.Run(test.remote, func(t *testing.T) {
			t.Parallel()
			reader, closeServer := testReader(t, func(*http.Request) (int, string) {
				return http.StatusOK, taskEnvelope("task-id", test.remote, nil, nil, nil)
			})
			defer closeServer()
			snapshot, err := reader.Read(context.Background(), "task-id")
			if err != nil || snapshot.Status != test.want ||
				(test.want == ntnxtask.StatusUnknown && snapshot.Status.Outcome() != ntnxtask.OutcomeUnsupported) {
				t.Fatalf("Read() = %#v, %v; want %q", snapshot, err, test.want)
			}
		})
	}
}

func TestGetTaskByIDUsesReadRetryWithoutRequestID(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	reader, closeServer := testReader(t, func(request *http.Request) (int, string) {
		call := calls.Add(1)
		if values := request.Header.Values("NTNX-Request-Id"); len(values) != 0 {
			t.Fatalf("request ID values = %v, want none", values)
		}
		if call == 1 {
			return http.StatusServiceUnavailable, `{}`
		}
		return http.StatusOK, taskEnvelope("task-id", "SUCCEEDED", nil, nil, nil)
	})
	defer closeServer()
	if _, err := reader.Read(context.Background(), "task-id"); err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if calls.Load() != 2 {
		t.Fatalf("server calls = %d, want one read retry", calls.Load())
	}
}

func TestGetTaskByIDAcceptsOnlyHTTP200(t *testing.T) {
	t.Parallel()

	reader, closeServer := testReader(t, func(*http.Request) (int, string) {
		return http.StatusCreated, taskEnvelope("task-id", "SUCCEEDED", nil, nil, nil)
	})
	defer closeServer()
	_, err := reader.Read(context.Background(), "task-id")
	var httpError *transport.HTTPError
	if !errors.As(err, &httpError) || httpError.StatusCode() != http.StatusCreated {
		t.Fatalf("Read() error = %v, want typed HTTP 201", err)
	}
}

func TestGetTaskByIDProjectsOnlyBoundedSafeFields(t *testing.T) {
	t.Parallel()

	errorsBody := []map[string]any{{
		"code":         "TASK.CODE-1",
		"errorGroup":   "TASK_GROUP",
		"severity":     "ERROR",
		"message":      "vendor-error-message-secret-canary",
		"locale":       "private-locale-secret-canary",
		"argumentsMap": map[string]string{"secret": "argument-secret-canary"},
	}}
	warningsBody := []map[string]any{{
		"code":       "TASK.WARNING-1",
		"errorGroup": "TASK_GROUP",
		"severity":   "WARNING",
		"message":    "vendor-warning-message-secret-canary",
	}}
	completion := []map[string]any{{"name": "private-name-secret-canary", "value": "private-value-secret-canary"}}
	reader, closeServer := testReader(t, func(*http.Request) (int, string) {
		return http.StatusOK, taskEnvelope("task-id", "FAILED", errorsBody, warningsBody, completion)
	})
	defer closeServer()

	snapshot, err := reader.Read(context.Background(), "task-id")
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if len(snapshot.Errors) != 1 || len(snapshot.Warnings) != 1 {
		t.Fatalf("projected counts = %d/%d", len(snapshot.Errors), len(snapshot.Warnings))
	}
	wantError := ntnxtask.Projection{Code: "TASK.CODE-1", ErrorGroup: "TASK_GROUP", Severity: "ERROR"}
	if snapshot.Errors[0] != wantError {
		t.Fatalf("error projection = %#v, want %#v", snapshot.Errors[0], wantError)
	}
	for _, rendered := range []string{fmt.Sprintf("%v", snapshot), fmt.Sprintf("%+v", snapshot), fmt.Sprintf("%#v", snapshot)} {
		for _, canary := range []string{"vendor-error-message-secret-canary", "private-locale-secret-canary", "argument-secret-canary", "vendor-warning-message-secret-canary", "private-name-secret-canary", "private-value-secret-canary"} {
			if strings.Contains(rendered, canary) {
				t.Fatalf("snapshot leaked %q: %q", canary, rendered)
			}
		}
	}
}

func TestGetTaskByIDProjectsAffectedEntities(t *testing.T) {
	t.Parallel()

	entities := []map[string]any{{
		"extId": "7ccae44f-d067-4d49-a4d0-7409e2455894",
		"rel":   "networking:config:subnet",
		"name":  "subnet-safe-name",
	}}
	reader, closeServer := testReader(t, func(*http.Request) (int, string) {
		return http.StatusOK, taskEnvelopeWithEntities("task-id", "SUCCEEDED", nil, nil, nil, entities)
	})
	defer closeServer()

	snapshot, err := reader.Read(context.Background(), "task-id")
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if len(snapshot.EntitiesAffected) != 1 ||
		snapshot.EntitiesAffected[0].ExtID != entities[0]["extId"] ||
		snapshot.EntitiesAffected[0].Rel != entities[0]["rel"] {
		t.Fatalf("entities = %#v", snapshot.EntitiesAffected)
	}
}

func TestGetTaskByIDAcceptsEveryLockedCompletionDetailValueShape(t *testing.T) {
	t.Parallel()

	values := []any{
		"string-value-secret-canary",
		int64(42),
		true,
		[]string{"one", "two"},
		[]int64{1, 2},
		map[string]string{"key": "value-secret-canary"},
		[]map[string]any{{}, {"map": map[string]string{"key": "value-secret-canary"}}},
	}
	details := make([]map[string]any, len(values))
	for index, value := range values {
		details[index] = map[string]any{"name": fmt.Sprintf("detail-%d", index), "value": value}
	}
	reader, closeServer := testReader(t, func(*http.Request) (int, string) {
		return http.StatusOK, taskEnvelope("task-id", "SUCCEEDED", nil, nil, details)
	})
	defer closeServer()
	snapshot, err := reader.Read(context.Background(), "task-id")
	if err != nil || snapshot.Status != ntnxtask.StatusSucceeded {
		t.Fatalf("Read() = %#v, %v; want valid union values discarded", snapshot, err)
	}
	for _, rendered := range []string{fmt.Sprintf("%v", snapshot), fmt.Sprintf("%+v", snapshot), fmt.Sprintf("%#v", snapshot)} {
		for _, canary := range []string{"string-value-secret-canary", "value-secret-canary"} {
			if strings.Contains(rendered, canary) {
				t.Fatalf("snapshot retained completion value %q: %q", canary, rendered)
			}
		}
	}
}

func TestGetTaskByIDRejectsInvalidOrOversizedCompletionDetailUnionValues(t *testing.T) {
	t.Parallel()

	overStrings := make([]string, 101)
	overIntegers := make([]int64, 101)
	overWrappers := make([]map[string]any, 21)
	for index := range overStrings {
		overStrings[index] = "safe"
		overIntegers[index] = int64(index)
	}
	for index := range overWrappers {
		overWrappers[index] = map[string]any{"map": map[string]string{"key": "safe"}}
	}
	tests := []struct {
		name  string
		value any
	}{
		{name: "too many strings", value: overStrings},
		{name: "too many integers", value: overIntegers},
		{name: "too many wrappers", value: overWrappers},
		{name: "raw map array", value: []map[string]string{{"key": "invalid-secret-canary"}}},
		{name: "wrapper additional property", value: []map[string]any{{"map": map[string]string{"key": "safe"}, "extra": "invalid-secret-canary"}}},
		{name: "non-string map value", value: map[string]any{"key": 1}},
		{name: "mixed array", value: []any{"one", int64(2)}},
		{name: "fractional number", value: 1.5},
		{name: "null", value: nil},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			details := []map[string]any{{"name": "detail", "value": test.value}}
			reader, closeServer := testReader(t, func(*http.Request) (int, string) {
				return http.StatusOK, taskEnvelope("task-id", "SUCCEEDED", nil, nil, details)
			})
			defer closeServer()
			_, err := reader.Read(context.Background(), "task-id")
			if !errors.Is(err, ErrInvalidTaskResponse) {
				t.Fatalf("Read() error = %v, want ErrInvalidTaskResponse", err)
			}
			if strings.Contains(fmt.Sprintf("%+v", err), "invalid-secret-canary") {
				t.Fatalf("error leaked invalid union value: %+v", err)
			}
		})
	}
}

func TestGetTaskByIDRejectsInvalidAndOversizedDecodedFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body func() string
	}{
		{name: "missing data", body: func() string { return `{}` }},
		{name: "missing status", body: func() string { return `{"data":{"extId":"task-id"}}` }},
		{name: "mismatched ext ID", body: func() string { return taskEnvelope("other-id", "RUNNING", nil, nil, nil) }},
		{name: "negative progress", body: func() string { return `{"data":{"extId":"task-id","status":"RUNNING","progressPercentage":-1}}` }},
		{name: "progress over 100", body: func() string { return `{"data":{"extId":"task-id","status":"RUNNING","progressPercentage":101}}` }},
		{name: "invalid time", body: func() string {
			return `{"data":{"extId":"task-id","status":"RUNNING","lastUpdatedTime":"time-secret-canary"}}`
		}},
		{name: "too many errors", body: func() string { return taskEnvelope("task-id", "FAILED", repeatedMessages(101), nil, nil) }},
		{name: "too many warnings", body: func() string { return taskEnvelope("task-id", "RUNNING", nil, repeatedMessages(51), nil) }},
		{name: "too many details", body: func() string { return taskEnvelope("task-id", "SUCCEEDED", nil, nil, repeatedDetails(51)) }},
		{name: "invalid detail shape", body: func() string { return `{"data":{"extId":"task-id","status":"SUCCEEDED","completionDetails":[42]}}` }},
		{name: "trailing JSON", body: func() string { return taskEnvelope("task-id", "RUNNING", nil, nil, nil) + ` {}` }},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			reader, closeServer := testReader(t, func(*http.Request) (int, string) {
				return http.StatusOK, test.body()
			})
			defer closeServer()
			_, err := reader.Read(context.Background(), "task-id")
			if !errors.Is(err, ErrInvalidTaskResponse) {
				t.Fatalf("Read() error = %v, want ErrInvalidTaskResponse", err)
			}
			for _, canary := range []string{"time-secret-canary", "message-secret-canary"} {
				if strings.Contains(fmt.Sprintf("%+v", err), canary) {
					t.Fatalf("error leaked %q: %v", canary, err)
				}
			}
		})
	}
}

func TestGetTaskByIDEnforcesFourMiBResponseLimit(t *testing.T) {
	t.Parallel()

	reader, closeServer := testReader(t, func(*http.Request) (int, string) {
		return http.StatusOK, strings.Repeat("x", (4<<20)+1)
	})
	defer closeServer()
	_, err := reader.Read(context.Background(), "task-id")
	var transportError *transport.TransportError
	if !errors.As(err, &transportError) || transportError.Kind() != transport.TransportFailureResponseLimit {
		t.Fatalf("Read() error = %v, want response-limit TransportError", err)
	}
}

func TestGetTaskByIDRejectsInvalidInputsWithoutExecution(t *testing.T) {
	t.Parallel()

	if _, err := NewReader(nil); !errors.Is(err, ErrInvalidTaskReader) {
		t.Fatalf("NewReader(nil) error = %v", err)
	}
	reader := &Reader{}
	for _, test := range []struct {
		name  string
		ctx   context.Context
		extID string
	}{
		{name: "nil context", extID: "task-id"},
		{name: "empty ID", ctx: context.Background()},
		{name: "control ID", ctx: context.Background(), extID: "task\nsecret-canary"},
	} {
		_, err := reader.Read(test.ctx, test.extID)
		if !errors.Is(err, ErrInvalidTaskRequest) {
			t.Fatalf("%s Read() error = %v", test.name, err)
		}
	}
}

func testReader(t *testing.T, handler func(*http.Request) (int, string)) (*Reader, func()) {
	t.Helper()
	server := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		status, body := handler(request)
		response.Header().Set("Content-Type", "application/json")
		response.WriteHeader(status)
		_, _ = response.Write([]byte(body))
	}))
	origin, err := transport.ParseOrigin(server.URL)
	if err != nil {
		server.Close()
		t.Fatalf("ParseOrigin() error = %v", err)
	}
	tlsConfig, err := transport.NewTLSConfig(origin, true, "")
	if err != nil {
		server.Close()
		t.Fatalf("NewTLSConfig() error = %v", err)
	}
	client, err := transport.NewClient(origin, auth.NewAPIKey("safe-api-key"), tlsConfig, 5*time.Second, "test", "test")
	if err != nil {
		server.Close()
		t.Fatalf("NewClient() error = %v", err)
	}
	reader, err := NewReader(client)
	if err != nil {
		server.Close()
		t.Fatalf("NewReader() error = %v", err)
	}
	return reader, server.Close
}

func taskEnvelope(extID, status string, errorMessages, warnings, completionDetails []map[string]any) string {
	return taskEnvelopeWithEntities(extID, status, errorMessages, warnings, completionDetails, nil)
}

func taskEnvelopeWithEntities(
	extID, status string,
	errorMessages, warnings, completionDetails, entities []map[string]any,
) string {
	data := map[string]any{
		"extId":              extID,
		"status":             status,
		"progressPercentage": 50,
		"entitiesAffected":   entities,
		"errorMessages":      errorMessages,
		"warnings":           warnings,
		"completionDetails":  completionDetails,
		"lastUpdatedTime":    "2026-08-05T15:00:00Z",
	}
	body, _ := json.Marshal(map[string]any{"data": data})
	return string(body)
}

func repeatedMessages(count int) []map[string]any {
	values := make([]map[string]any, count)
	for index := range values {
		values[index] = map[string]any{"code": "SAFE", "message": "message-secret-canary"}
	}
	return values
}

func repeatedDetails(count int) []map[string]any {
	values := make([]map[string]any, count)
	for index := range values {
		values[index] = map[string]any{"name": "safe", "value": "private"}
	}
	return values
}
