package transport

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestRequestAndOptionsRedactEveryFormatting(t *testing.T) {
	t.Parallel()

	options := RequestOptions{
		Operation:        "prism.get_task",
		Method:           http.MethodPost,
		PathTemplate:     "/api/test/{id}",
		PathParameters:   map[string]string{"id": "path-format-secret-canary"},
		Query:            url.Values{"filter": {"query-format-secret-canary"}},
		Headers:          http.Header{"X-Metadata": {"header-format-secret-canary"}},
		JSONBody:         []byte("body-format-secret-canary"),
		ExpectedStatuses: []int{http.StatusOK},
	}
	request, err := NewRequest(options)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	for _, value := range []any{options, request} {
		for _, rendered := range formatEveryWay(value) {
			for _, canary := range []string{"path-format-secret-canary", "query-format-secret-canary", "header-format-secret-canary", "body-format-secret-canary"} {
				if strings.Contains(rendered, canary) {
					t.Fatalf("%T formatting leaked %q: %q", value, canary, rendered)
				}
			}
		}
	}
}

func TestResponseRedactsEveryFormatting(t *testing.T) {
	t.Parallel()

	response := newResponse(
		http.StatusOK,
		http.Header{"ETag": {"etag-format-secret-canary"}, "X-Vendor": {"header-format-secret-canary"}},
		[]byte("body-format-secret-canary"),
		"",
	)
	for _, rendered := range formatEveryWay(response) {
		for _, canary := range []string{"etag-format-secret-canary", "header-format-secret-canary", "body-format-secret-canary"} {
			if strings.Contains(rendered, canary) {
				t.Fatalf("Response formatting leaked %q: %q", canary, rendered)
			}
		}
	}
}

func formatEveryWay(value any) []string {
	return []string{
		fmt.Sprint(value),
		fmt.Sprintf("%s", value),
		fmt.Sprintf("%v", value),
		fmt.Sprintf("%+v", value),
		fmt.Sprintf("%#v", value),
		fmt.Sprintf("%q", value),
	}
}

func TestRequestCopiesAndLocksOperationInputs(t *testing.T) {
	t.Parallel()

	parameters := map[string]string{"extId": "task/+ = :"}
	query := url.Values{"filter": {"name eq secret-object"}}
	headers := http.Header{"X-Operation-Metadata": {"secret-header"}}
	body := []byte(`{"name":"secret-body"}`)
	statuses := []int{http.StatusOK, http.StatusAccepted}

	request, err := NewRequest(RequestOptions{
		Operation:        "prism.get_task",
		Method:           http.MethodPost,
		PathTemplate:     "/api/prism/v4.3/config/tasks/{extId}",
		PathParameters:   parameters,
		Query:            query,
		Headers:          headers,
		JSONBody:         body,
		ExpectedStatuses: statuses,
	})
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}

	parameters["extId"] = "mutated"
	query.Set("filter", "mutated")
	headers.Set("X-Operation-Metadata", "mutated")
	body[0] = 'x'
	statuses[0] = http.StatusTeapot

	if request.pathParameters["extId"] != "task/+ = :" {
		t.Fatalf("path parameter changed through caller mutation: %q", request.pathParameters["extId"])
	}
	if got := request.query.Get("filter"); got != "name eq secret-object" {
		t.Fatalf("query changed through caller mutation: %q", got)
	}
	if got := request.headers.Get("X-Operation-Metadata"); got != "secret-header" {
		t.Fatalf("header changed through caller mutation: %q", got)
	}
	if got := string(request.jsonBody); got != `{"name":"secret-body"}` {
		t.Fatalf("body changed through caller mutation: %q", got)
	}
	if !request.expects(http.StatusOK) || request.expects(http.StatusTeapot) {
		t.Fatal("expected statuses changed through caller mutation")
	}
	if request.successBodyLimit != DefaultSuccessBodyLimit {
		t.Fatalf("default success limit = %d, want %d", request.successBodyLimit, DefaultSuccessBodyLimit)
	}
}

func TestRequestAcceptsOperationSpecificLowerSuccessLimit(t *testing.T) {
	t.Parallel()

	request, err := NewRequest(RequestOptions{
		Operation:        "prism.get_task",
		Method:           http.MethodGet,
		PathTemplate:     "/api/prism/v4.3/config/tasks/{extId}",
		PathParameters:   map[string]string{"extId": "safe"},
		ExpectedStatuses: []int{http.StatusOK},
		SuccessBodyLimit: 4 << 20,
	})
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	if request.successBodyLimit != 4<<20 {
		t.Fatalf("success limit = %d, want 4 MiB", request.successBodyLimit)
	}
}

func TestRequestRejectsReservedHeadersCaseInsensitively(t *testing.T) {
	t.Parallel()

	for _, name := range []string{
		"Authorization",
		"authorization",
		"x-NTNX-api-KEY",
		"uSeR-aGeNt",
		"aCcEpT",
		"CONTENT-type",
		"hOsT",
		"connection",
		"content-LENGTH",
		"TRANSFER-encoding",
		"ntnx-REQUEST-id",
	} {
		name := name
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			request, err := NewRequest(RequestOptions{
				Operation:        "prism.get_task",
				Method:           http.MethodGet,
				PathTemplate:     "/api/test/{id}",
				PathParameters:   map[string]string{"id": "safe"},
				Headers:          http.Header{name: {"reserved-canary"}},
				ExpectedStatuses: []int{http.StatusOK},
			})
			if request.valid || !errors.Is(err, ErrReservedHeader) {
				t.Fatalf("NewRequest() = %#v, %v; want invalid request and ErrReservedHeader", request, err)
			}
			if rendered := err.Error(); rendered != ErrReservedHeader.Error() {
				t.Fatalf("reserved-header error = %q, want stable generic text", rendered)
			}
		})
	}
}

func TestRequestRejectsInvalidPoliciesWithoutEchoingInputs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		update func(*RequestOptions)
		want   error
	}{
		{name: "empty operation", update: func(options *RequestOptions) { options.Operation = "" }, want: ErrInvalidRequest},
		{name: "unsafe operation", update: func(options *RequestOptions) { options.Operation = "operation-secret-canary\n" }, want: ErrInvalidRequest},
		{name: "invalid method", update: func(options *RequestOptions) { options.Method = "GET method-secret-canary" }, want: ErrInvalidRequest},
		{name: "invalid path", update: func(options *RequestOptions) { options.PathTemplate = "/outside/path-secret-canary" }, want: ErrInvalidRequest},
		{name: "missing statuses", update: func(options *RequestOptions) { options.ExpectedStatuses = nil }, want: ErrInvalidRequest},
		{name: "informational status", update: func(options *RequestOptions) { options.ExpectedStatuses = []int{199} }, want: ErrInvalidRequest},
		{name: "redirect status", update: func(options *RequestOptions) { options.ExpectedStatuses = []int{300} }, want: ErrInvalidRequest},
		{name: "error status", update: func(options *RequestOptions) { options.ExpectedStatuses = []int{400} }, want: ErrInvalidRequest},
		{name: "out-of-range status", update: func(options *RequestOptions) { options.ExpectedStatuses = []int{999} }, want: ErrInvalidRequest},
		{name: "negative limit", update: func(options *RequestOptions) { options.SuccessBodyLimit = -1 }, want: ErrInvalidResponseLimit},
		{name: "unapproved higher limit", update: func(options *RequestOptions) { options.SuccessBodyLimit = DefaultSuccessBodyLimit + 1 }, want: ErrInvalidResponseLimit},
		{name: "invalid query", update: func(options *RequestOptions) { options.Query = url.Values{"query-secret-canary\n": {"value"}} }, want: ErrInvalidRequest},
		{name: "invalid header value", update: func(options *RequestOptions) { options.Headers = http.Header{"X-Safe": {"header-secret-canary\r\n"}} }, want: ErrInvalidRequest},
		{name: "control header value", update: func(options *RequestOptions) { options.Headers = http.Header{"X-Safe": {"header-secret-canary\x01"}} }, want: ErrInvalidRequest},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			options := RequestOptions{
				Operation:        "prism.get_task",
				Method:           http.MethodGet,
				PathTemplate:     "/api/test/{id}",
				PathParameters:   map[string]string{"id": "safe"},
				ExpectedStatuses: []int{http.StatusOK},
			}
			test.update(&options)
			request, err := NewRequest(options)
			if request.valid || !errors.Is(err, test.want) {
				t.Fatalf("NewRequest() = %#v, %v; want invalid request and %v", request, err, test.want)
			}
			for _, canary := range []string{"operation-secret-canary", "method-secret-canary", "path-secret-canary", "query-secret-canary", "header-secret-canary"} {
				if strings.Contains(err.Error(), canary) {
					t.Fatalf("error leaked %q: %q", canary, err)
				}
			}
		})
	}
}
