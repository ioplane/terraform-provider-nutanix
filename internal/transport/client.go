package transport

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
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
	errResponseWithError         = errors.New("round trip returned a response and an error")
)

type authorizer interface {
	Authorize(*http.Request)
}

type requestPlan struct {
	method         string
	pathTemplate   string
	pathParameters map[string]string
	query          url.Values
	headers        http.Header
	jsonBody       []byte
	requestID      string
}

type attemptRoundTripper struct {
	base       http.RoundTripper
	authorizer authorizer
	userAgent  string
}

// Client owns one immutable HTTPS origin, authentication mode, and HTTP client.
type Client struct {
	origin          Origin
	httpClient      *http.Client
	eventSink       eventSink
	requestIDSource requestIDSource
	jitterSource    jitterSource
	retrySleeper    retrySleeper
	now             func() time.Time
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
	return newClientWithSink(
		origin,
		selectedAuthorizer,
		tlsConfig,
		timeout,
		providerVersion,
		terraformVersion,
		base,
		tflogEventSink{},
	)
}

func newClientWithSink(
	origin Origin,
	selectedAuthorizer authorizer,
	tlsConfig *tls.Config,
	timeout time.Duration,
	providerVersion string,
	terraformVersion string,
	base http.RoundTripper,
	sink eventSink,
) (*Client, error) {
	if err := validateClientDependencies(origin, selectedAuthorizer, tlsConfig, timeout); err != nil {
		return nil, err
	}
	if isNilInterface(base) {
		return nil, ErrMissingRoundTripper
	}
	if isNilInterface(sink) {
		sink = tflogEventSink{}
	}
	attempt := &attemptRoundTripper{
		base:       base,
		authorizer: selectedAuthorizer,
		userAgent:  buildUserAgent(providerVersion, terraformVersion),
	}
	return &Client{
		origin:          origin,
		eventSink:       sink,
		requestIDSource: newRandomRequestID,
		jitterSource:    fullJitter,
		retrySleeper:    sleepForRetry,
		now:             time.Now,
		httpClient: &http.Client{
			Transport: attempt,
			Timeout:   timeout,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return ErrRedirectRefused
			},
		},
	}, nil
}

// Execute performs one logical operation with its immutable retry policy.
func (c *Client) Execute(ctx context.Context, request Request) (response Response, resultErr error) {
	operation := safeOperation(request.operation)
	if c == nil || c.origin.host == "" || c.httpClient == nil ||
		isNilInterface(c.httpClient.Transport) || isNilInterface(c.eventSink) {
		return Response{}, newTransportError(operation, TransportFailureRequest, ErrRequestFailed)
	}
	if ctx == nil || !request.valid {
		return Response{}, newTransportError(operation, TransportFailureRequest, ErrInvalidRequest)
	}
	if err := ctx.Err(); err != nil {
		return Response{}, newTransportError(operation, transportKindForCause(err), err)
	}
	if c.now == nil {
		return Response{}, newTransportError(operation, TransportFailureRequest, ErrRequestFailed)
	}

	requestID := ""
	if request.retryClass == RetryIdempotentMutation && request.permitsRetry() {
		var ok bool
		requestID, ok = requestIDFor(request, c.requestIDSource)
		if !ok {
			return Response{}, newTransportError(operation, TransportFailureRequest, ErrRequestFailed)
		}
	}

	plan := requestPlan{
		method:         request.method,
		pathTemplate:   request.pathTemplate,
		pathParameters: cloneStringMap(request.pathParameters),
		query:          cloneQuery(request.query),
		headers:        request.headers.Clone(),
		jsonBody:       slices.Clone(request.jsonBody),
		requestID:      requestID,
	}

	budget := retryBudget{}
	for attempt := 1; attempt <= maximumAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return Response{}, newTransportError(operation, transportKindForCause(err), err)
		}

		started := c.now()
		rawResponse, attemptErr := c.executeAttempt(ctx, plan)
		event := attemptEvent{
			operation:     operation,
			method:        request.method,
			pathTemplate:  request.pathTemplate,
			attempt:       attempt,
			status:        statusFromResponse(rawResponse),
			correlationID: requestID,
		}
		if rawResponse != nil && event.correlationID == "" {
			event.correlationID = responseCorrelationID(rawResponse.Header)
		}

		var (
			currentError error
			retryable    bool
			retryAfter   string
		)
		if attemptErr != nil {
			if !sameKnownError(canonicalErrorCause(attemptErr), ErrRedirectRefused) {
				closeResponseBody(rawResponse)
			}
			retryable = classifyRetryableTransportError(ctx, attemptErr)
			cause := stableAttemptCause(ctx, attemptErr)
			if retryable && sameKnownError(cause, context.DeadlineExceeded) {
				cause = ErrRequestFailed
			}
			currentError = newTransportError(operation, transportKindForCause(cause), cause)
		} else if rawResponse == nil {
			currentError = newTransportError(operation, TransportFailureRequest, ErrRequestFailed)
		} else {
			if rawResponse.Body == nil {
				rawResponse.Body = http.NoBody
			}
			headers := rawResponse.Header.Clone()
			correlationID := responseCorrelationID(headers)
			limit := request.successBodyLimit
			if !request.expects(rawResponse.StatusCode) {
				limit = DefaultErrorBodyLimit
			}
			body, readErr := readBoundedBody(rawResponse.Body, limit)
			closeErr := rawResponse.Body.Close()
			switch {
			case readErr != nil:
				cause := stableReadCause(ctx, readErr)
				currentError = newTransportError(operation, transportKindForCause(cause), cause)
			case closeErr != nil:
				currentError = newTransportError(operation, TransportFailureResponseClose, ErrResponseClose)
			case request.expects(rawResponse.StatusCode):
				event.duration = nonNegativeDuration(c.now().Sub(started))
				c.eventSink.EmitAttempt(ctx, event)
				return newResponse(rawResponse.StatusCode, headers, body, correlationID), nil
			default:
				currentError = newHTTPError(operation, rawResponse.StatusCode, correlationID, body)
				retryable = retryableHTTPStatus(rawResponse.StatusCode)
				retryAfter = uniqueHeaderValueEqualFold(headers, "Retry-After")
			}
		}

		event.duration = nonNegativeDuration(c.now().Sub(started))
		c.eventSink.EmitAttempt(ctx, event)
		if attempt == maximumAttempts || !request.permitsRetry() || !retryable ||
			c.jitterSource == nil || c.retrySleeper == nil {
			return Response{}, currentError
		}
		if err := ctx.Err(); err != nil {
			return Response{}, newTransportError(operation, transportKindForCause(err), err)
		}
		delay, ok := budget.nextDelay(ctx, attempt, retryAfter, c.now(), c.jitterSource)
		if !ok {
			if err := ctx.Err(); err != nil {
				return Response{}, newTransportError(operation, transportKindForCause(err), err)
			}
			return Response{}, currentError
		}
		if err := c.retrySleeper(ctx, delay); err != nil {
			cause := stableAttemptCause(ctx, err)
			return Response{}, newTransportError(operation, transportKindForCause(cause), cause)
		}
	}
	return Response{}, newTransportError(operation, TransportFailureRequest, ErrRequestFailed)
}

func nonNegativeDuration(value time.Duration) time.Duration {
	if value < 0 {
		return 0
	}
	return value
}

func readBoundedBody(body io.Reader, limit int64) ([]byte, error) {
	bounded, err := io.ReadAll(io.LimitReader(body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(bounded)) > limit {
		return nil, ErrResponseTooLarge
	}
	return bounded, nil
}

func closeResponseBody(response *http.Response) {
	if response != nil && response.Body != nil {
		_ = response.Body.Close()
	}
}

func stableAttemptCause(ctx context.Context, err error) error {
	if ctx != nil {
		if contextErr := ctx.Err(); contextErr != nil {
			return contextErr
		}
	}
	switch cause := canonicalErrorCause(err); cause {
	case ErrRedirectRefused, context.Canceled, context.DeadlineExceeded:
		return cause
	}
	return ErrRequestFailed
}

func stableReadCause(ctx context.Context, err error) error {
	if ctx != nil {
		if contextErr := ctx.Err(); contextErr != nil {
			return contextErr
		}
	}
	switch cause := canonicalErrorCause(err); cause {
	case ErrResponseTooLarge, context.Canceled, context.DeadlineExceeded:
		return cause
	}
	return ErrResponseRead
}

func transportKindForCause(cause error) TransportFailureKind {
	switch cause {
	case context.Canceled:
		return TransportFailureCanceled
	case context.DeadlineExceeded:
		return TransportFailureDeadline
	case ErrRedirectRefused:
		return TransportFailureRedirect
	case ErrResponseTooLarge:
		return TransportFailureResponseLimit
	case ErrResponseRead:
		return TransportFailureResponseRead
	case ErrResponseClose:
		return TransportFailureResponseClose
	default:
		return TransportFailureRequest
	}
}

func responseCorrelationID(header http.Header) string {
	return validCorrelationIDOrEmpty(uniqueHeaderValueEqualFold(header, "NTNX-Request-Id"))
}

func uniqueHeaderValueEqualFold(header http.Header, name string) string {
	var found string
	var count int
	for key, values := range header {
		if !strings.EqualFold(key, name) {
			continue
		}
		count += len(values)
		if count > 1 {
			return ""
		}
		if len(values) == 1 {
			found = values[0]
		}
	}
	if count != 1 {
		return ""
	}
	return found
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
		query:          nil,
		headers:        headers.Clone(),
		jsonBody:       bodyCopy,
	}
}

func (c *Client) executeAttempt(ctx context.Context, plan requestPlan) (*http.Response, error) {
	requestURL, err := c.origin.url(plan.pathTemplate, plan.pathParameters)
	if err != nil {
		return nil, err
	}
	requestURL.RawQuery = plan.query.Encode()

	var body *bytes.Reader
	if plan.jsonBody != nil {
		body = bytes.NewReader(plan.jsonBody)
	} else {
		body = bytes.NewReader(nil)
	}
	request, err := http.NewRequestWithContext(
		contextWithRequestID(ctx, plan.requestID),
		plan.method,
		requestURL.String(),
		body,
	)
	if err != nil {
		return nil, ErrInvalidRequest
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
	deleteHeaderEqualFold(attempt.Header, requestIDHeader)
	if requestID := requestIDFromContext(attempt.Context()); requestID != "" {
		attempt.Header.Set(requestIDHeader, requestID)
	}
	response, err := t.base.RoundTrip(attempt)
	if response != nil && err != nil {
		closeResponseBody(response)
		if sameKnownError(canonicalErrorCause(err), ErrRedirectRefused) {
			return nil, ErrRedirectRefused
		}
		return nil, errResponseWithError
	}
	return response, err
}

func scrubAttemptHeaders(header http.Header) {
	for key := range header {
		if reservedAttemptHeader(key) {
			delete(header, key)
		}
	}
}

func deleteHeaderEqualFold(header http.Header, name string) {
	for key := range header {
		if strings.EqualFold(key, name) {
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
