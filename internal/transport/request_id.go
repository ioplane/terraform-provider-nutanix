package transport

import (
	"context"

	"github.com/google/uuid"
)

const requestIDHeader = "NTNX-Request-Id"

type requestIDSource func() (uuid.UUID, error)

type requestIDContextKey struct{}

func newRandomRequestID() (uuid.UUID, error) {
	return uuid.NewRandom()
}

func requestIDFor(request Request, source requestIDSource) (string, bool) {
	if request.retryClass != RetryIdempotentMutation ||
		!request.requestIDRequired ||
		!request.replayable ||
		source == nil {
		return "", false
	}
	value, err := source()
	if err != nil || value == uuid.Nil ||
		value.Version() != uuid.Version(4) || value.Variant() != uuid.RFC4122 {
		return "", false
	}
	return value.String(), true
}

func contextWithRequestID(ctx context.Context, requestID string) context.Context {
	if requestID == "" {
		return ctx
	}
	return context.WithValue(ctx, requestIDContextKey{}, requestID)
}

func requestIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	requestID, _ := ctx.Value(requestIDContextKey{}).(string)
	return validCorrelationIDOrEmpty(requestID)
}
