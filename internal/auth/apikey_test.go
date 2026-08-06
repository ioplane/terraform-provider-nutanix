package auth

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestAPIKeyAuthorizeReplacesHeader(t *testing.T) {
	t.Parallel()

	request, err := http.NewRequest(http.MethodGet, "https://pc.example.test/api/prism/v4.3/config/tasks/id", nil)
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	request.Header.Add("X-ntnx-api-key", "first-stale-value")
	request.Header.Add("X-ntnx-api-key", "second-stale-value")
	request.Header.Set("Authorization", "Basic stale-cross-mode-basic")
	request.Header["x-ntnx-api-key"] = []string{"lower-case-stale-api-key"}
	request.Header["X-NTNX-API-KEY"] = []string{"upper-case-stale-api-key"}
	request.Header["aUtHoRiZaTiOn"] = []string{"Basic mixed-case-stale-basic"}

	authenticator := NewAPIKey("api-key-canary-65fce862")
	authenticator.Authorize(request)

	values := headerValuesEqualFold(request.Header, "X-ntnx-api-key")
	if len(values) != 1 || values[0] != "api-key-canary-65fce862" {
		t.Fatalf("X-ntnx-api-key values do not match the immutable authenticator")
	}
	if got := headerValuesEqualFold(request.Header, "Authorization"); len(got) != 0 {
		t.Fatalf("Authorization variants = %v, want stale cross-mode headers removed", got)
	}
}

func TestAPIKeyFormattingRedactsCredential(t *testing.T) {
	t.Parallel()

	const canary = "api-key-canary-f654578d"
	authenticator := NewAPIKey(canary)

	for _, rendered := range []string{
		fmt.Sprintf("%s", authenticator),
		fmt.Sprintf("%q", authenticator),
		fmt.Sprintf("%v", authenticator),
		fmt.Sprintf("%+v", authenticator),
		fmt.Sprintf("%#v", authenticator),
	} {
		if strings.Contains(rendered, canary) {
			t.Fatalf("formatted API-key authenticator exposes credential canary: %q", rendered)
		}
	}
}
