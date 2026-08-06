package auth

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestBasicAuthorizeReplacesHeader(t *testing.T) {
	t.Parallel()

	request, err := http.NewRequest(http.MethodGet, "https://pc.example.test/api/prism/v4.3/config/tasks/id", nil)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	request.Header.Add("Authorization", "first-stale-value")
	request.Header.Add("Authorization", "second-stale-value")
	request.Header.Set("X-ntnx-api-key", "stale-cross-mode-api-key")
	request.Header["authorization"] = []string{"lower-case-stale-basic"}
	request.Header["AUTHORIZATION"] = []string{"upper-case-stale-basic"}
	request.Header["x-NTNX-api-KEY"] = []string{"mixed-case-stale-api-key"}

	authenticator := NewBasic("admin", "password-canary-4d6358dd")
	authenticator.Authorize(request)

	username, password, ok := request.BasicAuth()
	if !ok {
		t.Fatal("request has no Basic authorization header")
	}
	if username != "admin" || password != "password-canary-4d6358dd" {
		t.Fatal("request Basic credentials do not match the immutable authenticator")
	}
	if got := len(headerValuesEqualFold(request.Header, "Authorization")); got != 1 {
		t.Fatalf("Authorization value count = %d, want 1", got)
	}
	if got := headerValuesEqualFold(request.Header, "X-ntnx-api-key"); len(got) != 0 {
		t.Fatalf("X-ntnx-api-key variants = %v, want stale cross-mode headers removed", got)
	}
}

func headerValuesEqualFold(header http.Header, name string) []string {
	var values []string
	for key, current := range header {
		if strings.EqualFold(key, name) {
			values = append(values, current...)
		}
	}
	return values
}

func TestBasicFormattingRedactsCredentials(t *testing.T) {
	t.Parallel()

	const (
		username = "username-canary-8fe977ba"
		password = "password-canary-b7b21349"
	)
	authenticator := NewBasic(username, password)

	for _, rendered := range []string{
		fmt.Sprintf("%s", authenticator),
		fmt.Sprintf("%q", authenticator),
		fmt.Sprintf("%v", authenticator),
		fmt.Sprintf("%+v", authenticator),
		fmt.Sprintf("%#v", authenticator),
	} {
		for _, canary := range []string{username, password} {
			if strings.Contains(rendered, canary) {
				t.Fatalf("formatted Basic authenticator exposes credential canary: %q", rendered)
			}
		}
	}
}
