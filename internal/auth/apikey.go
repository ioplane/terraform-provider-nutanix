package auth

import (
	"fmt"
	"io"
	"net/http"
)

const apiKeyHeader = "X-ntnx-api-key"

// APIKey applies an immutable Nutanix API key.
type APIKey struct {
	value string
}

// NewAPIKey returns an immutable API-key authenticator.
func NewAPIKey(value string) APIKey {
	return APIKey{value: value}
}

// Authorize replaces the request's API-key header.
func (a APIKey) Authorize(request *http.Request) {
	deleteHeaderEqualFold(request.Header, "Authorization")
	deleteHeaderEqualFold(request.Header, apiKeyHeader)
	request.Header.Set(apiKeyHeader, a.value)
}

// Format redacts the credential from every fmt rendering.
func (a APIKey) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, "auth.APIKey(redacted)")
}
