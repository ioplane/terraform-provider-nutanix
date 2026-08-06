package transport

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/ioplane/terraform-provider-nutanix/internal/auth"
)

func FuzzBoundedErrorResponse(f *testing.F) {
	f.Add([]byte("vendor-fuzz-canary"))
	f.Add([]byte{})

	f.Fuzz(func(t *testing.T, fuzzed []byte) {
		if int64(len(fuzzed)) > DefaultErrorBodyLimit+1 {
			t.Skip()
		}
		body := append([]byte("bounded-error-fuzz-secret:"), fuzzed...)
		client := testClientWithRoundTripper(t, auth.NewAPIKey("fuzz-api-key"), roundTripFunc(func(request *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusBadRequest,
				Header:     make(http.Header),
				Body:       io.NopCloser(strings.NewReader(string(body))),
				Request:    request,
			}, nil
		}))
		request, err := NewRequest(RequestOptions{
			Operation:        "fuzz.error_response",
			Method:           http.MethodGet,
			PathTemplate:     "/api/fuzz",
			ExpectedStatuses: []int{http.StatusOK},
		})
		if err != nil {
			t.Fatalf("NewRequest() error = %v", err)
		}
		_, executeErr := client.Execute(context.Background(), request)
		if executeErr == nil {
			t.Fatal("Execute() error = nil")
		}
		if strings.Contains(executeErr.Error(), "bounded-error-fuzz-secret") {
			t.Fatal("error rendering contains bounded response body")
		}
		var httpError *HTTPError
		if errors.As(executeErr, &httpError) && int64(len(httpError.Body())) > DefaultErrorBodyLimit {
			t.Fatalf("HTTPError body length = %d, limit %d", len(httpError.Body()), DefaultErrorBodyLimit)
		}
	})
}
