package transport

import (
	"context"
	"net/http"
)

const ifMatchHeader = "If-Match"

type ifMatchContextKey struct{}

func responseETag(header http.Header) string {
	return uniqueHeaderValueEqualFold(header, "ETag")
}

func lockIfMatch(required bool, value string) (string, bool) {
	if !required {
		return "", value == ""
	}
	if !validOpaqueETag(value) {
		return "", false
	}
	return value, true
}

func validOpaqueETag(value string) bool {
	if value == "" || !validHTTPFieldValue(value) {
		return false
	}
	if value[0] == ' ' || value[0] == '\t' || value[len(value)-1] == ' ' || value[len(value)-1] == '\t' {
		return false
	}
	for index := 0; index < len(value); index++ {
		if value[index] != ' ' && value[index] != '\t' {
			return true
		}
	}
	return false
}

func contextWithIfMatch(ctx context.Context, value string) context.Context {
	if value == "" {
		return ctx
	}
	return context.WithValue(ctx, ifMatchContextKey{}, value)
}

func ifMatchFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	value, _ := ctx.Value(ifMatchContextKey{}).(string)
	if !validOpaqueETag(value) {
		return ""
	}
	return value
}
