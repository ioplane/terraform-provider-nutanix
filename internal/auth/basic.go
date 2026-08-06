// Package auth applies immutable credentials to one HTTP request.
package auth

import (
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Basic applies immutable HTTP Basic credentials.
type Basic struct {
	username string
	password string
}

// NewBasic returns an immutable Basic authenticator.
func NewBasic(username, password string) Basic {
	return Basic{username: username, password: password}
}

// Authorize replaces the request's Authorization header with Basic credentials.
func (a Basic) Authorize(request *http.Request) {
	deleteHeaderEqualFold(request.Header, "Authorization")
	deleteHeaderEqualFold(request.Header, apiKeyHeader)
	request.SetBasicAuth(a.username, a.password)
}

// Format redacts credentials from every fmt rendering.
func (a Basic) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, "auth.Basic(redacted)")
}

func deleteHeaderEqualFold(header http.Header, name string) {
	for key := range header {
		if strings.EqualFold(key, name) {
			delete(header, key)
		}
	}
}
