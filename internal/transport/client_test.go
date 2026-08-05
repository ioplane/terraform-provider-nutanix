package transport

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/ioplane/terraform-provider-nutanix/internal/auth"
)

func TestClientAttemptUsesImmutableInputsAndKernelOwnedHeaders(t *testing.T) {
	t.Parallel()

	const (
		originalID   = "task/+ = :"
		originalBody = `{"name":"original"}`
	)
	parameters := map[string]string{"extId": originalID}
	headers := http.Header{
		"X-Test-Input":        []string{"original"},
		"x-allowlisted-input": []string{"lower-case-original"},
		"Authorization":       []string{"Basic stale", "Bearer stale"},
		"authorization":       []string{"Basic lower-case-stale"},
		"X-ntnx-api-key":      []string{"stale-api-key"},
		"x-NTNX-api-KEY":      []string{"mixed-case-stale-api-key"},
		"User-Agent":          []string{"caller-user-agent"},
		"user-agent":          []string{"lower-case-caller-user-agent"},
		"accept":              []string{"text/plain"},
		"content-type":        []string{"text/plain"},
		"Host":                []string{"evil.example.test"},
		"hOsT":                []string{"mixed-case-evil.example.test"},
		"Connection":          []string{"close"},
		"connection":          []string{"upgrade"},
		"Content-Length":      []string{"999999"},
		"content-length":      []string{"888888"},
		"Transfer-Encoding":   []string{"chunked"},
		"transfer-encoding":   []string{"compress"},
		"NTNX-Request-Id":     []string{"caller-request-id"},
		"ntnx-request-id":     []string{"lower-case-caller-request-id"},
	}
	body := []byte(originalBody)
	plan := newRequestPlan(
		http.MethodPost,
		"/api/prism/v4.3/config/tasks/{extId}",
		parameters,
		headers,
		body,
	)

	parameters["extId"] = "mutated/../escape"
	headers.Set("X-Test-Input", "mutated")
	headers.Set("Authorization", "Basic mutated")
	copy(body, strings.Repeat("x", len(body)))

	var calls int
	client := testClientWithRoundTripper(t, auth.NewAPIKey("selected-api-key"), roundTripFunc(func(request *http.Request) (*http.Response, error) {
		calls++
		wantPath := "/api/prism/v4.3/config/tasks/" + url.PathEscape(originalID)
		if got := request.URL.EscapedPath(); got != wantPath {
			t.Fatalf("wire path = %q, want immutable %q", got, wantPath)
		}
		if request.URL.Host != "pc.example.test:9440" || request.Host != "" {
			t.Fatalf("request authority = URL %q Host override %q", request.URL.Host, request.Host)
		}
		gotBody, err := io.ReadAll(request.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		if string(gotBody) != originalBody {
			t.Fatalf("request body = %q, want immutable original", gotBody)
		}
		if got := request.Header.Get("X-Test-Input"); got != "original" {
			t.Fatalf("copied input header = %q, want original", got)
		}
		if got := headerValuesFold(request.Header, "X-Allowlisted-Input"); len(got) != 1 || got[0] != "lower-case-original" {
			t.Fatalf("non-reserved mixed-case header = %v, want preserved", got)
		}
		if got := request.UserAgent(); got != "terraform-provider-nutanix/1.2.3 terraform/1.15.8" {
			t.Fatalf("User-Agent = %q, want kernel-owned value", got)
		}
		if got := headerValuesFold(request.Header, "User-Agent"); len(got) != 1 || got[0] != request.UserAgent() {
			t.Fatalf("User-Agent variants = %v, want exactly kernel-owned value", got)
		}
		if got := request.Header.Get("Accept"); got != "application/json" {
			t.Fatalf("Accept = %q, want application/json", got)
		}
		if got := headerValuesFold(request.Header, "Accept"); len(got) != 1 || got[0] != "application/json" {
			t.Fatalf("Accept variants = %v, want exactly application/json", got)
		}
		if got := request.Header.Get("Content-Type"); got != "application/json" {
			t.Fatalf("Content-Type = %q, want application/json", got)
		}
		if got := headerValuesFold(request.Header, "Content-Type"); len(got) != 1 || got[0] != "application/json" {
			t.Fatalf("Content-Type variants = %v, want exactly application/json", got)
		}
		if got := headerValuesFold(request.Header, "X-ntnx-api-key"); len(got) != 1 || got[0] != "selected-api-key" {
			t.Fatalf("X-ntnx-api-key values = %v, want exactly selected authentication", got)
		}
		if got := headerValuesFold(request.Header, "Authorization"); len(got) != 0 {
			t.Fatalf("Authorization variants = %v, want scrubbed cross-mode input", got)
		}
		for _, name := range []string{"Host", "Connection", "Content-Length", "Transfer-Encoding", "NTNX-Request-Id"} {
			if got := headerValuesFold(request.Header, name); len(got) != 0 {
				t.Fatalf("reserved header %s variants = %v, want scrubbed", name, got)
			}
		}
		if request.ContentLength != int64(len(originalBody)) || len(request.TransferEncoding) != 0 || request.Close {
			t.Fatalf("request framing = length %d transfer %v close %t", request.ContentLength, request.TransferEncoding, request.Close)
		}
		return noContentResponse(request), nil
	}))

	response, err := client.executeAttempt(context.Background(), plan)
	if err != nil {
		t.Fatalf("executeAttempt() error = %v", err)
	}
	_ = response.Body.Close()
	if calls != 1 {
		t.Fatalf("RoundTrip calls = %d, want exactly 1", calls)
	}
}

func TestClientAttemptBasicAuthScrubsCrossModeInput(t *testing.T) {
	t.Parallel()

	client := testClientWithRoundTripper(t, auth.NewBasic("admin", "password"), roundTripFunc(func(request *http.Request) (*http.Response, error) {
		username, password, ok := request.BasicAuth()
		if !ok || username != "admin" || password != "password" {
			t.Fatal("attempt request has incorrect Basic authentication")
		}
		if got := headerValuesFold(request.Header, "X-ntnx-api-key"); len(got) != 0 {
			t.Fatalf("X-ntnx-api-key variants = %v, want scrubbed cross-mode input", got)
		}
		if got := len(headerValuesFold(request.Header, "Authorization")); got != 1 {
			t.Fatalf("Authorization value count = %d, want exactly 1", got)
		}
		return noContentResponse(request), nil
	}))
	plan := newRequestPlan(
		http.MethodGet,
		"/api/test/{id}",
		map[string]string{"id": "safe"},
		http.Header{
			"Authorization":  []string{"Bearer stale"},
			"authorization":  []string{"Basic lower-case-stale"},
			"X-ntnx-api-key": []string{"stale-api-key"},
			"x-NTNX-api-KEY": []string{"mixed-case-stale-api-key"},
		},
		nil,
	)
	response, err := client.executeAttempt(context.Background(), plan)
	if err != nil {
		t.Fatalf("executeAttempt() error = %v", err)
	}
	_ = response.Body.Close()
}

func TestClientAttemptRejectsCaseVariantDualAuthentication(t *testing.T) {
	t.Parallel()

	var calls int
	client := testClientWithRoundTripper(t, dualCaseVariantAuthorizer{}, roundTripFunc(func(request *http.Request) (*http.Response, error) {
		calls++
		return noContentResponse(request), nil
	}))
	response, err := client.executeAttempt(context.Background(), newRequestPlan(
		http.MethodGet,
		"/api/test/{id}",
		map[string]string{"id": "safe"},
		nil,
		nil,
	))
	if response != nil {
		_ = response.Body.Close()
	}
	if !errors.Is(err, ErrInvalidAuthorizationState) {
		t.Fatalf("executeAttempt() error = %v, want ErrInvalidAuthorizationState", err)
	}
	if calls != 0 {
		t.Fatalf("base RoundTrip calls = %d, want 0 for dual authentication", calls)
	}
}

func TestClientAttemptAcceptsExactlyOneCaseVariantAuthentication(t *testing.T) {
	t.Parallel()

	var calls int
	client := testClientWithRoundTripper(t, singleCaseVariantAuthorizer{}, roundTripFunc(func(request *http.Request) (*http.Response, error) {
		calls++
		if got := headerValuesFold(request.Header, "Authorization"); len(got) != 1 || got[0] != "Basic case-variant" {
			t.Fatalf("Authorization variants = %v, want exactly one", got)
		}
		return noContentResponse(request), nil
	}))
	response, err := client.executeAttempt(context.Background(), newRequestPlan(
		http.MethodGet,
		"/api/test/{id}",
		map[string]string{"id": "safe"},
		nil,
		nil,
	))
	if err != nil {
		t.Fatalf("executeAttempt() error = %v", err)
	}
	_ = response.Body.Close()
	if calls != 1 {
		t.Fatalf("base RoundTrip calls = %d, want 1", calls)
	}
}

func TestClientUserAgentIsInjectedSanitizedAndBoundedAtRoundTrip(t *testing.T) {
	t.Parallel()

	var gotUserAgent string
	client := testClientWithVersions(
		t,
		auth.NewAPIKey("safe-api-key"),
		"1.2.3\r\nInjected: true",
		"1.15.8\tpreview",
		roundTripFunc(func(request *http.Request) (*http.Response, error) {
			gotUserAgent = request.UserAgent()
			return noContentResponse(request), nil
		}),
	)
	response, err := client.executeAttempt(context.Background(), newRequestPlan(
		http.MethodGet,
		"/api/test/{id}",
		map[string]string{"id": "safe"},
		http.Header{"User-Agent": []string{"caller-override"}},
		nil,
	))
	if err != nil {
		t.Fatalf("executeAttempt() error = %v", err)
	}
	_ = response.Body.Close()
	want := "terraform-provider-nutanix/1.2.3__Injected__true terraform/1.15.8_preview"
	if gotUserAgent != want {
		t.Fatalf("User-Agent at RoundTrip = %q, want %q", gotUserAgent, want)
	}
	if strings.ContainsAny(gotUserAgent, "\r\n\t") {
		t.Fatalf("User-Agent contains a control character: %q", gotUserAgent)
	}

	client = testClientWithVersions(
		t,
		auth.NewAPIKey("safe-api-key"),
		strings.Repeat("p", 1024),
		strings.Repeat("t", 1024),
		roundTripFunc(func(request *http.Request) (*http.Response, error) {
			gotUserAgent = request.UserAgent()
			return noContentResponse(request), nil
		}),
	)
	response, err = client.executeAttempt(context.Background(), newRequestPlan(
		http.MethodGet,
		"/api/test/{id}",
		map[string]string{"id": "safe"},
		nil,
		nil,
	))
	if err != nil {
		t.Fatalf("bounded executeAttempt() error = %v", err)
	}
	_ = response.Body.Close()
	if len(gotUserAgent) > maximumUserAgentBytes {
		t.Fatalf("bounded User-Agent length = %d, maximum %d", len(gotUserAgent), maximumUserAgentBytes)
	}
}

func TestClientJSONHeadersAreAppliedAtRoundTrip(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		body            []byte
		wantContentType string
	}{
		{name: "no body", body: nil},
		{name: "JSON body", body: []byte(`{}`), wantContentType: "application/json"},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			client := testClientWithRoundTripper(t, auth.NewAPIKey("safe-api-key"), roundTripFunc(func(request *http.Request) (*http.Response, error) {
				if got := headerValuesFold(request.Header, "Accept"); len(got) != 1 || got[0] != "application/json" {
					t.Fatalf("Accept variants = %v, want exactly application/json", got)
				}
				contentTypes := headerValuesFold(request.Header, "Content-Type")
				if test.wantContentType == "" && len(contentTypes) != 0 {
					t.Fatalf("Content-Type variants = %v, want absent", contentTypes)
				}
				if test.wantContentType != "" && (len(contentTypes) != 1 || contentTypes[0] != test.wantContentType) {
					t.Fatalf("Content-Type variants = %v, want exactly %q", contentTypes, test.wantContentType)
				}
				return noContentResponse(request), nil
			}))
			response, err := client.executeAttempt(context.Background(), newRequestPlan(
				http.MethodPost,
				"/api/test/{id}",
				map[string]string{"id": "safe"},
				http.Header{
					"accept":       []string{"text/plain"},
					"content-type": []string{"text/plain"},
				},
				test.body,
			))
			if err != nil {
				t.Fatalf("executeAttempt() error = %v", err)
			}
			_ = response.Body.Close()
		})
	}
}

func TestClientRejectsInvalidDependenciesWithoutPanic(t *testing.T) {
	t.Parallel()

	origin := testOrigin(t)
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12, ServerName: origin.Hostname()}
	validAuthorizer := auth.NewAPIKey("safe-api-key")
	tweakTLS := func(update func(*tls.Config)) *tls.Config {
		config := tlsConfig.Clone()
		update(config)
		return config
	}
	validRoundTripper := roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return noContentResponse(request), nil
	})
	var typedNilAuthorizer *nilAuthorizer
	tests := []struct {
		name      string
		construct func() (*Client, error)
		want      error
	}{
		{name: "zero origin", construct: func() (*Client, error) {
			return NewClient(Origin{}, validAuthorizer, tlsConfig, time.Second, "test", "1.15.8")
		}, want: ErrInvalidClientOrigin},
		{name: "nil authorizer", construct: func() (*Client, error) {
			return NewClient(origin, nil, tlsConfig, time.Second, "test", "1.15.8")
		}, want: ErrMissingAuthorizer},
		{name: "typed nil authorizer", construct: func() (*Client, error) {
			return NewClient(origin, typedNilAuthorizer, tlsConfig, time.Second, "test", "1.15.8")
		}, want: ErrMissingAuthorizer},
		{name: "nil TLS", construct: func() (*Client, error) {
			return NewClient(origin, validAuthorizer, nil, time.Second, "test", "1.15.8")
		}, want: ErrMissingTLSConfig},
		{name: "TLS minimum below 1.2", construct: func() (*Client, error) {
			return NewClient(origin, validAuthorizer, tweakTLS(func(config *tls.Config) {
				config.MinVersion = tls.VersionTLS11
			}), time.Second, "test", "1.15.8")
		}, want: ErrInvalidTLSConfig},
		{name: "TLS server name mismatch", construct: func() (*Client, error) {
			return NewClient(origin, validAuthorizer, tweakTLS(func(config *tls.Config) {
				config.ServerName = "other.example.test"
			}), time.Second, "test", "1.15.8")
		}, want: ErrInvalidTLSConfig},
		{name: "zero timeout", construct: func() (*Client, error) {
			return NewClient(origin, validAuthorizer, tlsConfig, 0, "test", "1.15.8")
		}, want: ErrInvalidClientTimeout},
		{name: "negative timeout", construct: func() (*Client, error) {
			return NewClient(origin, validAuthorizer, tlsConfig, -time.Second, "test", "1.15.8")
		}, want: ErrInvalidClientTimeout},
		{name: "nil round tripper", construct: func() (*Client, error) {
			return newClient(origin, validAuthorizer, tlsConfig, time.Second, "test", "1.15.8", nil)
		}, want: ErrMissingRoundTripper},
		{name: "valid round tripper", construct: func() (*Client, error) {
			return newClient(origin, validAuthorizer, tlsConfig, time.Second, "test", "1.15.8", validRoundTripper)
		}},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			defer func() {
				if recovered := recover(); recovered != nil {
					t.Fatalf("client constructor panicked: %v", recovered)
				}
			}()
			client, err := test.construct()
			if test.want == nil {
				if err != nil || client == nil {
					t.Fatalf("valid constructor = %v, %v; want client", client, err)
				}
				return
			}
			if client != nil || !errors.Is(err, test.want) {
				t.Fatalf("constructor = %v, %v; want nil and %v", client, err, test.want)
			}
		})
	}
}

func TestClientUsesOwnedTransportWhenDefaultTransportIsReplaced(t *testing.T) {
	originalDefault := http.DefaultTransport
	http.DefaultTransport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		panic("process-wide default transport must not be used")
	})
	t.Cleanup(func() { http.DefaultTransport = originalDefault })

	origin := testOrigin(t)
	client, err := NewClient(
		origin,
		auth.NewAPIKey("safe-api-key"),
		&tls.Config{MinVersion: tls.VersionTLS12, ServerName: origin.Hostname()},
		23*time.Second,
		"test",
		"1.15.8",
	)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	configuredTransport := clientBaseTransport(t, client)
	if !configuredTransport.ForceAttemptHTTP2 {
		t.Fatal("owned transport does not enable standard HTTP/2 attempts")
	}
	if configuredTransport.Proxy == nil ||
		reflect.ValueOf(configuredTransport.Proxy).Pointer() != reflect.ValueOf(http.ProxyFromEnvironment).Pointer() {
		t.Fatal("owned transport proxy selector is not http.ProxyFromEnvironment")
	}
	if client.httpClient.Timeout != 23*time.Second {
		t.Fatalf("HTTP timeout = %s, want 23s", client.httpClient.Timeout)
	}
}

func TestClientDeepClonesTLSRootPool(t *testing.T) {
	t.Parallel()

	origin := testOrigin(t)
	firstPEM, firstCertificate := testCACertificate(t, "first.example.test")
	_ = firstPEM
	_, laterCertificate := testCACertificate(t, "later.example.test")
	roots := x509.NewCertPool()
	roots.AddCert(firstCertificate)
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
		ServerName: origin.Hostname(),
		RootCAs:    roots,
	}
	client, err := NewClient(origin, auth.NewAPIKey("safe-api-key"), tlsConfig, time.Second, "test", "1.15.8")
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	roots.AddCert(laterCertificate)

	clientRoots := clientBaseTransport(t, client).TLSClientConfig.RootCAs
	if _, err := firstCertificate.Verify(x509.VerifyOptions{Roots: clientRoots, DNSName: "first.example.test"}); err != nil {
		t.Fatalf("client lost initial root: %v", err)
	}
	if _, err := laterCertificate.Verify(x509.VerifyOptions{Roots: clientRoots, DNSName: "later.example.test"}); err == nil {
		t.Fatal("caller RootCAs mutation changed the client-owned trust pool")
	}
}

func TestClientRefusesRedirects(t *testing.T) {
	t.Parallel()

	server := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		http.Redirect(response, request, "/next", http.StatusFound)
	}))
	t.Cleanup(server.Close)
	origin, err := ParseOrigin(server.URL)
	if err != nil {
		t.Fatalf("ParseOrigin() error = %v", err)
	}
	tlsConfig := server.Client().Transport.(*http.Transport).TLSClientConfig.Clone()
	tlsConfig.ServerName = origin.Hostname()
	tlsConfig.MinVersion = tls.VersionTLS12
	client, err := NewClient(origin, auth.NewAPIKey("safe-api-key"), tlsConfig, 5*time.Second, "test", "1.15.8")
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	response, err := client.executeAttempt(context.Background(), newRequestPlan(
		http.MethodGet,
		"/api/test/{id}",
		map[string]string{"id": "redirect"},
		nil,
		nil,
	))
	if response != nil {
		_ = response.Body.Close()
	}
	if !errors.Is(err, ErrRedirectRefused) {
		t.Fatalf("HTTP redirect error = %v, want ErrRedirectRefused", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

type nilAuthorizer struct{}

func (*nilAuthorizer) Authorize(*http.Request) {}

type dualCaseVariantAuthorizer struct{}

func (dualCaseVariantAuthorizer) Authorize(request *http.Request) {
	request.Header["authorization"] = []string{"Basic case-variant"}
	request.Header["x-NTNX-api-KEY"] = []string{"case-variant-api-key"}
}

type singleCaseVariantAuthorizer struct{}

func (singleCaseVariantAuthorizer) Authorize(request *http.Request) {
	request.Header["authorization"] = []string{"Basic case-variant"}
}

func testClientWithRoundTripper(t *testing.T, selected authorizer, base http.RoundTripper) *Client {
	t.Helper()
	return testClientWithVersions(t, selected, "1.2.3", "1.15.8", base)
}

func testClientWithVersions(
	t *testing.T,
	selected authorizer,
	providerVersion string,
	terraformVersion string,
	base http.RoundTripper,
) *Client {
	t.Helper()
	origin := testOrigin(t)
	client, err := newClient(
		origin,
		selected,
		&tls.Config{MinVersion: tls.VersionTLS12, ServerName: origin.Hostname()},
		17*time.Second,
		providerVersion,
		terraformVersion,
		base,
	)
	if err != nil {
		t.Fatalf("newClient() error = %v", err)
	}
	return client
}

func testOrigin(t *testing.T) Origin {
	t.Helper()
	origin, err := ParseOrigin("https://pc.example.test:9440")
	if err != nil {
		t.Fatalf("ParseOrigin() error = %v", err)
	}
	return origin
}

func clientBaseTransport(t *testing.T, client *Client) *http.Transport {
	t.Helper()
	attempt, ok := client.httpClient.Transport.(*attemptRoundTripper)
	if !ok {
		t.Fatalf("client transport type = %T, want *attemptRoundTripper", client.httpClient.Transport)
	}
	base, ok := attempt.base.(*http.Transport)
	if !ok {
		t.Fatalf("base transport type = %T, want *http.Transport", attempt.base)
	}
	return base
}

func noContentResponse(request *http.Request) *http.Response {
	return &http.Response{
		StatusCode: http.StatusNoContent,
		Header:     make(http.Header),
		Body:       http.NoBody,
		Request:    request,
	}
}

func headerValuesFold(header http.Header, name string) []string {
	var values []string
	for key, current := range header {
		if strings.EqualFold(key, name) {
			values = append(values, current...)
		}
	}
	return values
}
