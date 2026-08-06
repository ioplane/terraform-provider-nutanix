package transport

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	// DefaultSuccessBodyLimit is the maximum accepted success body unless an
	// operation selects a reviewed lower ceiling.
	DefaultSuccessBodyLimit int64 = 16 << 20
	// DefaultErrorBodyLimit is the fixed maximum accepted non-success body.
	DefaultErrorBodyLimit int64 = 1 << 20
)

var (
	// ErrInvalidRequest identifies an invalid or zero operation request.
	ErrInvalidRequest = errors.New("transport request is invalid")
	// ErrReservedHeader identifies an adapter attempt to set a kernel-owned header.
	ErrReservedHeader = errors.New("transport request contains a reserved header")
	// ErrInvalidResponseLimit identifies a response ceiling outside the approved range.
	ErrInvalidResponseLimit = errors.New("transport request response limit is invalid")
)

// RequestOptions describes one namespace-owned operation policy and its
// request-specific inputs. NewRequest copies every reference-bearing value.
type RequestOptions struct {
	Operation         string
	Method            string
	PathTemplate      string
	PathParameters    map[string]string
	Query             url.Values
	Headers           http.Header
	JSONBody          []byte
	ExpectedStatuses  []int
	SuccessBodyLimit  int64
	RetryClass        RetryClass
	RequestIDRequired bool
	Replayable        bool
	IfMatchRequired   bool
	IfMatchETag       string
}

// Format prevents operation inputs from entering formatted diagnostics or logs.
func (RequestOptions) Format(state fmt.State, verb rune) {
	formatSafeText(state, verb, "transport.RequestOptions(redacted)")
}

// Request is an immutable, origin-independent operation request.
// Its zero value is invalid.
type Request struct {
	operation         string
	method            string
	pathTemplate      string
	pathParameters    map[string]string
	query             url.Values
	headers           http.Header
	jsonBody          []byte
	expectedStatuses  map[int]struct{}
	successBodyLimit  int64
	retryClass        RetryClass
	requestIDRequired bool
	replayable        bool
	ifMatch           string
	valid             bool
}

// Format prevents locked request inputs from entering formatted diagnostics or logs.
func (Request) Format(state fmt.State, verb rune) {
	formatSafeText(state, verb, "transport.Request(redacted)")
}

// NewRequest validates and locks one namespace operation request.
func NewRequest(options RequestOptions) (Request, error) {
	if !validOperation(options.Operation) || !validHTTPToken(options.Method) {
		return Request{}, ErrInvalidRequest
	}
	if _, err := (Origin{host: "validation.invalid"}).url(options.PathTemplate, options.PathParameters); err != nil {
		return Request{}, ErrInvalidRequest
	}
	if !validQuery(options.Query) {
		return Request{}, ErrInvalidRequest
	}
	for name, values := range options.Headers {
		if reservedAttemptHeader(name) {
			return Request{}, ErrReservedHeader
		}
		if !validHTTPToken(name) {
			return Request{}, ErrInvalidRequest
		}
		for _, value := range values {
			if !validHTTPFieldValue(value) {
				return Request{}, ErrInvalidRequest
			}
		}
	}
	if len(options.ExpectedStatuses) == 0 {
		return Request{}, ErrInvalidRequest
	}
	if !options.RetryClass.valid() ||
		(options.RequestIDRequired && options.RetryClass != RetryIdempotentMutation) {
		return Request{}, ErrInvalidRequest
	}
	ifMatch, ok := lockIfMatch(options.IfMatchRequired, options.IfMatchETag)
	if !ok {
		return Request{}, ErrInvalidRequest
	}
	expectedStatuses := make(map[int]struct{}, len(options.ExpectedStatuses))
	for _, status := range options.ExpectedStatuses {
		if status < http.StatusOK || status >= http.StatusMultipleChoices {
			return Request{}, ErrInvalidRequest
		}
		expectedStatuses[status] = struct{}{}
	}
	limit := options.SuccessBodyLimit
	if limit == 0 {
		limit = DefaultSuccessBodyLimit
	}
	if limit < 1 || limit > DefaultSuccessBodyLimit {
		return Request{}, ErrInvalidResponseLimit
	}

	return Request{
		operation:         options.Operation,
		method:            options.Method,
		pathTemplate:      options.PathTemplate,
		pathParameters:    cloneStringMap(options.PathParameters),
		query:             cloneQuery(options.Query),
		headers:           options.Headers.Clone(),
		jsonBody:          slices.Clone(options.JSONBody),
		expectedStatuses:  expectedStatuses,
		successBodyLimit:  limit,
		retryClass:        options.RetryClass,
		requestIDRequired: options.RequestIDRequired,
		replayable:        options.Replayable,
		ifMatch:           ifMatch,
		valid:             true,
	}, nil
}

func (r Request) expects(status int) bool {
	_, ok := r.expectedStatuses[status]
	return ok
}

func cloneStringMap(source map[string]string) map[string]string {
	cloned := make(map[string]string, len(source))
	for key, value := range source {
		cloned[key] = value
	}
	return cloned
}

func cloneQuery(source url.Values) url.Values {
	cloned := make(url.Values, len(source))
	for key, values := range source {
		cloned[key] = slices.Clone(values)
	}
	return cloned
}

func validOperation(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	for index := 0; index < len(value); index++ {
		character := value[index]
		if (character >= 'a' && character <= 'z') ||
			(index > 0 && character >= 'A' && character <= 'Z') ||
			(index > 0 && character >= '0' && character <= '9') ||
			(index > 0 && strings.ContainsRune("._-", rune(character))) {
			continue
		}
		return false
	}
	return true
}

func validHTTPToken(value string) bool {
	if value == "" {
		return false
	}
	for index := 0; index < len(value); index++ {
		if !validHTTPTokenByte(value[index]) {
			return false
		}
	}
	return true
}

func validHTTPFieldValue(value string) bool {
	for index := 0; index < len(value); index++ {
		if (value[index] < 0x20 && value[index] != '\t') || value[index] == 0x7f {
			return false
		}
	}
	return true
}

func validQuery(query url.Values) bool {
	for key, values := range query {
		if key == "" || !utf8.ValidString(key) || !validHTTPFieldValue(key) {
			return false
		}
		for _, value := range values {
			if !utf8.ValidString(value) || !validHTTPFieldValue(value) {
				return false
			}
		}
	}
	return true
}

// Response is a copied, bounded, vendor-neutral HTTP response.
type Response struct {
	statusCode    int
	headers       http.Header
	body          []byte
	etag          string
	correlationID string
}

// Format prevents response headers and bodies from entering formatted diagnostics or logs.
func (r Response) Format(state fmt.State, verb rune) {
	formatSafeText(state, verb, "transport.Response(status="+strconv.Itoa(r.statusCode)+", redacted)")
}

func newResponse(statusCode int, headers http.Header, body []byte, correlationID string) Response {
	return Response{
		statusCode:    statusCode,
		headers:       headers.Clone(),
		body:          slices.Clone(body),
		etag:          responseETag(headers),
		correlationID: validCorrelationIDOrEmpty(correlationID),
	}
}

// StatusCode returns the HTTP status code.
func (r Response) StatusCode() int { return r.statusCode }

// Headers returns a copy of the response headers.
func (r Response) Headers() http.Header { return r.headers.Clone() }

// Body returns a copy of the bounded response body for namespace decoding.
func (r Response) Body() []byte { return slices.Clone(r.body) }

// ETag returns the opaque response entity tag.
func (r Response) ETag() string { return r.etag }

// CorrelationID returns a validated UUID correlation identifier when present.
func (r Response) CorrelationID() string { return r.correlationID }
