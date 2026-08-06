package transport

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/ioplane/terraform-provider-nutanix/internal/auth"
)

func TestETagExtractionPreservesOpaqueValueAndFailsClosedOnDuplicates(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name   string
		header http.Header
		want   string
	}{
		{name: "opaque strong", header: http.Header{"ETag": {`"opaque value:YWJj=="`}}, want: `"opaque value:YWJj=="`},
		{name: "opaque weak", header: http.Header{"eTaG": {`W/"case/Sensitive+opaque=="`}}, want: `W/"case/Sensitive+opaque=="`},
		{name: "absent", header: http.Header{}, want: ""},
		{name: "duplicate values", header: http.Header{"ETag": {`"one"`, `"two"`}}, want: ""},
		{name: "case variants", header: http.Header{"ETag": {`"one"`}, "etag": {`"two"`}}, want: ""},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			response := newResponse(http.StatusOK, test.header, nil, "")
			if got := response.ETag(); got != test.want {
				t.Fatalf("ETag() = %q, want exact %q", got, test.want)
			}
		})
	}
}

func TestIfMatchRequiresExplicitLockedPolicyAndOpaqueToken(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name     string
		required bool
		etag     string
		wantOK   bool
	}{
		{name: "not required and absent", wantOK: true},
		{name: "required opaque", required: true, etag: `W/"opaque token:YWJj=="`, wantOK: true},
		{name: "not required but supplied", etag: `"unexpected-secret-canary"`},
		{name: "required but absent", required: true},
		{name: "required whitespace", required: true, etag: " \t"},
		{name: "required leading whitespace", required: true, etag: " \"opaque\""},
		{name: "required trailing whitespace", required: true, etag: "\"opaque\"\t"},
		{name: "required control", required: true, etag: "etag-secret-canary\r\nInjected: yes"},
	} {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			request, err := NewRequest(RequestOptions{
				Operation:        "prism.update_category",
				Method:           http.MethodPatch,
				PathTemplate:     "/api/prism/v4.3/config/categories/{extId}",
				PathParameters:   map[string]string{"extId": "safe"},
				ExpectedStatuses: []int{http.StatusOK},
				IfMatchRequired:  test.required,
				IfMatchETag:      test.etag,
			})
			if test.wantOK {
				if err != nil || !request.valid {
					t.Fatalf("NewRequest() = %#v, %v; want success", request, err)
				}
				return
			}
			if request.valid || !errors.Is(err, ErrInvalidRequest) {
				t.Fatalf("NewRequest() = %#v, %v; want ErrInvalidRequest", request, err)
			}
			for _, rendered := range formatEveryWay(err) {
				if strings.Contains(rendered, "secret-canary") {
					t.Fatalf("error leaked ETag input: %q", rendered)
				}
			}
		})
	}
}

func TestIfMatchFormattingNeverExposesToken(t *testing.T) {
	t.Parallel()

	const canary = `"format-etag-secret-canary"`
	options := RequestOptions{
		Operation:        "prism.update_category",
		Method:           http.MethodPatch,
		PathTemplate:     "/api/prism/v4.3/config/categories/{extId}",
		PathParameters:   map[string]string{"extId": "safe"},
		ExpectedStatuses: []int{http.StatusOK},
		IfMatchRequired:  true,
		IfMatchETag:      canary,
	}
	request, err := NewRequest(options)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	for _, value := range []any{options, request} {
		for _, rendered := range formatEveryWay(value) {
			if strings.Contains(rendered, "format-etag-secret-canary") {
				t.Fatalf("%T formatting leaked ETag: %q", value, rendered)
			}
		}
	}
}

func TestIfMatchRawAdapterHeadersAreReservedCaseInsensitively(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"If-Match", "if-match", "iF-mAtCh"} {
		name := name
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			request, err := NewRequest(RequestOptions{
				Operation:        "prism.update_category",
				Method:           http.MethodPatch,
				PathTemplate:     "/api/prism/v4.3/config/categories/{extId}",
				PathParameters:   map[string]string{"extId": "safe"},
				Headers:          http.Header{name: {"raw-if-match-secret-canary"}},
				ExpectedStatuses: []int{http.StatusOK},
			})
			if request.valid || !errors.Is(err, ErrReservedHeader) || strings.Contains(err.Error(), "raw-if-match-secret-canary") {
				t.Fatalf("NewRequest() = %#v, %v; want redacted ErrReservedHeader", request, err)
			}
		})
	}
}

func TestIfMatchKernelScrubsAuthorizerVariantsAndSetsExactlyOne(t *testing.T) {
	t.Parallel()

	const selectedETag = `W/"selected-opaque-token"`
	client := testClientWithRoundTripper(t, ifMatchInjectingAuthorizer{}, roundTripFunc(func(request *http.Request) (*http.Response, error) {
		values := headerValuesFold(request.Header, "If-Match")
		if len(values) != 1 || values[0] != selectedETag {
			t.Fatalf("If-Match variants = %v, want exact kernel token", values)
		}
		if got := headerValuesFold(request.Header, "Authorization"); len(got) != 1 {
			t.Fatalf("Authorization variants = %v, want exactly one", got)
		}
		return noContentResponse(request), nil
	}))
	request := mustIfMatchRequest(t, selectedETag, RetryNone)
	if _, err := client.Execute(context.Background(), request); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
}

func TestIfMatchDoesNotLeakAcrossConcurrentRequests(t *testing.T) {
	t.Parallel()

	const first = `"first-etag-secret-canary"`
	const second = `W/"second-etag-secret-canary"`
	observed := make(chan string, 2)
	client := testClientWithRoundTripper(t, auth.NewAPIKey("safe-api-key"), roundTripFunc(func(request *http.Request) (*http.Response, error) {
		values := headerValuesFold(request.Header, "If-Match")
		if len(values) != 1 {
			t.Fatalf("If-Match variants = %v, want exactly one", values)
		}
		observed <- values[0]
		return noContentResponse(request), nil
	}))

	requests := []Request{mustIfMatchRequest(t, first, RetryNone), mustIfMatchRequest(t, second, RetryNone)}
	var wait sync.WaitGroup
	errorsSeen := make(chan error, len(requests))
	for _, request := range requests {
		request := request
		wait.Add(1)
		go func() {
			defer wait.Done()
			_, err := client.Execute(context.Background(), request)
			errorsSeen <- err
		}()
	}
	wait.Wait()
	close(errorsSeen)
	for err := range errorsSeen {
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
	}
	got := map[string]int{<-observed: 1, <-observed: 1}
	if got[first] != 1 || got[second] != 1 || len(got) != 2 {
		t.Fatalf("observed ETags = %v, want isolated tokens", got)
	}
}

func TestETagPreconditionStatusesRemainTypedAndNeverRetry(t *testing.T) {
	t.Parallel()

	for _, status := range []int{http.StatusPreconditionFailed, http.StatusPreconditionRequired} {
		status := status
		for _, retryClass := range []RetryClass{RetryRead, RetryIdempotentMutation} {
			retryClass := retryClass
			t.Run(http.StatusText(status)+"/"+retryClassName(retryClass), func(t *testing.T) {
				t.Parallel()
				var calls atomic.Int32
				body := &trackedSizedBody{remaining: 32}
				client := testClientWithRoundTripper(t, auth.NewAPIKey("safe-api-key"), roundTripFunc(func(request *http.Request) (*http.Response, error) {
					calls.Add(1)
					if values := headerValuesFold(request.Header, ifMatchHeader); len(values) != 1 || values[0] != `"request-etag-secret-canary"` {
						t.Fatalf("If-Match variants = %v, want isolated kernel token", values)
					}
					requestIDCount := len(headerValuesFold(request.Header, requestIDHeader))
					if retryClass == RetryIdempotentMutation && requestIDCount != 1 {
						t.Fatalf("request ID count = %d, want 1", requestIDCount)
					}
					if retryClass == RetryRead && requestIDCount != 0 {
						t.Fatalf("request ID count = %d, want 0", requestIDCount)
					}
					return &http.Response{
						StatusCode: status,
						Header:     http.Header{"ETag": {`"response-etag-secret-canary"`}},
						Body:       body,
						Request:    request,
					}, nil
				}))
				request := mustIfMatchRequest(t, `"request-etag-secret-canary"`, retryClass)
				_, err := client.Execute(context.Background(), request)
				var httpError *HTTPError
				if !errors.As(err, &httpError) || httpError.StatusCode() != status || calls.Load() != 1 || !body.closed.Load() {
					t.Fatalf("Execute() err=%v calls=%d closed=%t", err, calls.Load(), body.closed.Load())
				}
				for _, rendered := range formatEveryWay(err) {
					for _, canary := range []string{"request-etag-secret-canary", "response-etag-secret-canary"} {
						if strings.Contains(rendered, canary) {
							t.Fatalf("HTTP error leaked %q: %q", canary, rendered)
						}
					}
				}
			})
		}
	}
}

type ifMatchInjectingAuthorizer struct{}

func (ifMatchInjectingAuthorizer) Authorize(request *http.Request) {
	request.Header.Set("Authorization", "Basic safe")
	request.Header["If-Match"] = []string{`"authorizer-secret-one"`}
	request.Header["if-match"] = []string{`"authorizer-secret-two"`}
}

func mustIfMatchRequest(t *testing.T, etag string, retryClass RetryClass) Request {
	t.Helper()
	options := RequestOptions{
		Operation:        "prism.update_category",
		Method:           http.MethodPatch,
		PathTemplate:     "/api/prism/v4.3/config/categories/{extId}",
		PathParameters:   map[string]string{"extId": "safe"},
		JSONBody:         []byte(`{"description":"safe"}`),
		ExpectedStatuses: []int{http.StatusNoContent},
		RetryClass:       retryClass,
		IfMatchRequired:  true,
		IfMatchETag:      etag,
	}
	if retryClass == RetryIdempotentMutation {
		options.RequestIDRequired = true
		options.Replayable = true
	}
	request, err := NewRequest(options)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}
	return request
}

func retryClassName(class RetryClass) string {
	switch class {
	case RetryRead:
		return "read"
	case RetryIdempotentMutation:
		return "idempotent"
	default:
		return "none"
	}
}
