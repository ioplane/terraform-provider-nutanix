package transport

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"reflect"
	"slices"
	"strings"
	"time"
)

const (
	maximumVersionTokenBytes = 64
	maximumUserAgentBytes    = 192
)

var (
	// ErrRedirectRefused identifies an HTTP redirect rejected by the origin-bound client.
	ErrRedirectRefused = errors.New("http redirects are not followed")
	// ErrInvalidClientOrigin identifies a zero or otherwise invalid client origin dependency.
	ErrInvalidClientOrigin = errors.New("transport client origin is invalid")
	// ErrMissingAuthorizer identifies an absent authentication dependency.
	ErrMissingAuthorizer = errors.New("transport client authorizer is missing")
	// ErrMissingTLSConfig identifies an absent TLS dependency.
	ErrMissingTLSConfig = errors.New("transport client TLS configuration is missing")
	// ErrInvalidTLSConfig identifies a TLS dependency that violates the client security contract.
	ErrInvalidTLSConfig = errors.New("transport client TLS configuration is invalid")
	// ErrInvalidClientTimeout identifies a non-positive HTTP request timeout.
	ErrInvalidClientTimeout = errors.New("transport client timeout is invalid")
	// ErrMissingRoundTripper identifies an absent HTTP attempt dependency.
	ErrMissingRoundTripper = errors.New("transport client round tripper is missing")
	// ErrInvalidAuthorizationState identifies an authorizer that did not apply exactly one authentication mode.
	ErrInvalidAuthorizationState = errors.New("transport authorization state is invalid")
)

type authorizer interface {
	Authorize(*http.Request)
}

type requestPlan struct {
	method         string
	pathTemplate   string
	pathParameters map[string]string
	headers        http.Header
	jsonBody       []byte
}

type attemptRoundTripper struct {
	base       http.RoundTripper
	authorizer authorizer
	userAgent  string
}

// Client owns one immutable HTTPS origin, authentication mode, and HTTP client.
type Client struct {
	origin     Origin
	httpClient *http.Client
}

// NewClient constructs an origin-bound client without making a network request.
func NewClient(
	origin Origin,
	selectedAuthorizer authorizer,
	tlsConfig *tls.Config,
	timeout time.Duration,
	providerVersion string,
	terraformVersion string,
) (*Client, error) {
	if err := validateClientDependencies(origin, selectedAuthorizer, tlsConfig, timeout); err != nil {
		return nil, err
	}
	base := newOwnedHTTPTransport(deepCloneTLSConfig(tlsConfig))
	return newClient(
		origin,
		selectedAuthorizer,
		tlsConfig,
		timeout,
		providerVersion,
		terraformVersion,
		base,
	)
}

func newClient(
	origin Origin,
	selectedAuthorizer authorizer,
	tlsConfig *tls.Config,
	timeout time.Duration,
	providerVersion string,
	terraformVersion string,
	base http.RoundTripper,
) (*Client, error) {
	if err := validateClientDependencies(origin, selectedAuthorizer, tlsConfig, timeout); err != nil {
		return nil, err
	}
	if isNilInterface(base) {
		return nil, ErrMissingRoundTripper
	}
	attempt := &attemptRoundTripper{
		base:       base,
		authorizer: selectedAuthorizer,
		userAgent:  buildUserAgent(providerVersion, terraformVersion),
	}
	return &Client{
		origin: origin,
		httpClient: &http.Client{
			Transport: attempt,
			Timeout:   timeout,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return ErrRedirectRefused
			},
		},
	}, nil
}

func validateClientDependencies(
	origin Origin,
	selectedAuthorizer authorizer,
	tlsConfig *tls.Config,
	timeout time.Duration,
) error {
	if origin.host == "" {
		return ErrInvalidClientOrigin
	}
	if isNilInterface(selectedAuthorizer) {
		return ErrMissingAuthorizer
	}
	if tlsConfig == nil {
		return ErrMissingTLSConfig
	}
	if tlsConfig.MinVersion < tls.VersionTLS12 || !strings.EqualFold(tlsConfig.ServerName, origin.Hostname()) {
		return ErrInvalidTLSConfig
	}
	if timeout <= 0 {
		return ErrInvalidClientTimeout
	}
	return nil
}

func isNilInterface(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}

func newOwnedHTTPTransport(tlsConfig *tls.Config) *http.Transport {
	dialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}
	return &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           dialer.DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: time.Second,
		TLSClientConfig:       tlsConfig,
	}
}

func deepCloneTLSConfig(source *tls.Config) *tls.Config {
	cloned := source.Clone()
	if source.RootCAs != nil {
		cloned.RootCAs = source.RootCAs.Clone()
	}
	if source.ClientCAs != nil {
		cloned.ClientCAs = source.ClientCAs.Clone()
	}
	cloned.NextProtos = slices.Clone(source.NextProtos)
	cloned.CipherSuites = slices.Clone(source.CipherSuites)
	cloned.CurvePreferences = slices.Clone(source.CurvePreferences)
	cloned.Certificates = slices.Clone(source.Certificates)
	for index := range cloned.Certificates {
		cloned.Certificates[index].Certificate = cloneBytes2D(source.Certificates[index].Certificate)
		cloned.Certificates[index].OCSPStaple = slices.Clone(source.Certificates[index].OCSPStaple)
		cloned.Certificates[index].SignedCertificateTimestamps = cloneBytes2D(
			source.Certificates[index].SignedCertificateTimestamps,
		)
	}
	return cloned
}

func cloneBytes2D(source [][]byte) [][]byte {
	cloned := make([][]byte, len(source))
	for index := range source {
		cloned[index] = slices.Clone(source[index])
	}
	return cloned
}

func newRequestPlan(
	method string,
	pathTemplate string,
	pathParameters map[string]string,
	headers http.Header,
	jsonBody []byte,
) requestPlan {
	parametersCopy := make(map[string]string, len(pathParameters))
	for name, value := range pathParameters {
		parametersCopy[name] = value
	}
	var bodyCopy []byte
	if jsonBody != nil {
		bodyCopy = make([]byte, len(jsonBody))
		copy(bodyCopy, jsonBody)
	}
	return requestPlan{
		method:         method,
		pathTemplate:   pathTemplate,
		pathParameters: parametersCopy,
		headers:        headers.Clone(),
		jsonBody:       bodyCopy,
	}
}

func (c *Client) executeAttempt(ctx context.Context, plan requestPlan) (*http.Response, error) {
	requestURL, err := c.origin.url(plan.pathTemplate, plan.pathParameters)
	if err != nil {
		return nil, err
	}

	var body *bytes.Reader
	if plan.jsonBody != nil {
		body = bytes.NewReader(plan.jsonBody)
	} else {
		body = bytes.NewReader(nil)
	}
	request, err := http.NewRequestWithContext(ctx, plan.method, requestURL.String(), body)
	if err != nil {
		return nil, errors.New("request construction failed")
	}
	if plan.jsonBody == nil {
		request.Body = nil
		request.GetBody = nil
		request.ContentLength = 0
	}
	request.Header = plan.headers.Clone()
	if request.Header == nil {
		request.Header = make(http.Header)
	}
	return c.httpClient.Do(request)
}

func (t *attemptRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	attempt := request.Clone(request.Context())
	attempt.Header = request.Header.Clone()
	if attempt.Header == nil {
		attempt.Header = make(http.Header)
	}
	scrubAttemptHeaders(attempt.Header)
	attempt.Host = ""
	attempt.Close = false
	attempt.TransferEncoding = nil
	attempt.Header.Set("User-Agent", t.userAgent)
	attempt.Header.Set("Accept", "application/json")
	if attempt.Body != nil {
		attempt.Header.Set("Content-Type", "application/json")
	} else {
		attempt.Header.Del("Content-Type")
	}
	t.authorizer.Authorize(attempt)
	authorizationCount := headerValueCountEqualFold(attempt.Header, "Authorization")
	apiKeyCount := headerValueCountEqualFold(attempt.Header, "X-ntnx-api-key")
	if (authorizationCount != 1 || apiKeyCount != 0) &&
		(authorizationCount != 0 || apiKeyCount != 1) {
		return nil, ErrInvalidAuthorizationState
	}
	return t.base.RoundTrip(attempt)
}

func scrubAttemptHeaders(header http.Header) {
	for key := range header {
		if reservedAttemptHeader(key) {
			delete(header, key)
		}
	}
}

func reservedAttemptHeader(name string) bool {
	return strings.EqualFold(name, "Authorization") ||
		strings.EqualFold(name, "X-ntnx-api-key") ||
		strings.EqualFold(name, "User-Agent") ||
		strings.EqualFold(name, "Accept") ||
		strings.EqualFold(name, "Content-Type") ||
		strings.EqualFold(name, "Host") ||
		strings.EqualFold(name, "Connection") ||
		strings.EqualFold(name, "Content-Length") ||
		strings.EqualFold(name, "Transfer-Encoding") ||
		strings.EqualFold(name, "NTNX-Request-Id")
}

func headerValueCountEqualFold(header http.Header, name string) int {
	var count int
	for key, values := range header {
		if strings.EqualFold(key, name) {
			count += len(values)
		}
	}
	return count
}

func buildUserAgent(providerVersion, terraformVersion string) string {
	userAgent := "terraform-provider-nutanix/" + sanitizeVersionToken(providerVersion) +
		" terraform/" + sanitizeVersionToken(terraformVersion)
	if len(userAgent) > maximumUserAgentBytes {
		return userAgent[:maximumUserAgentBytes]
	}
	return userAgent
}

func sanitizeVersionToken(value string) string {
	if value == "" {
		return "unknown"
	}
	var builder strings.Builder
	builder.Grow(min(len(value), maximumVersionTokenBytes))
	for index := 0; index < len(value) && builder.Len() < maximumVersionTokenBytes; index++ {
		character := value[index]
		if validHTTPTokenByte(character) {
			builder.WriteByte(character)
		} else {
			builder.WriteByte('_')
		}
	}
	if builder.Len() == 0 {
		return "unknown"
	}
	return builder.String()
}

func validHTTPTokenByte(character byte) bool {
	if (character >= 'a' && character <= 'z') ||
		(character >= 'A' && character <= 'Z') ||
		(character >= '0' && character <= '9') {
		return true
	}
	return strings.ContainsRune("!#$%&'*+-.^_`|~", rune(character))
}
