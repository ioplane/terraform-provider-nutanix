package transport

import (
	"fmt"
	"net/url"
	"strings"
	"testing"
)

func TestOriginAcceptsStrictHTTPSOrigins(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"https://pc.example.test":       "https://pc.example.test",
		"https://pc.example.test/":      "https://pc.example.test",
		"https://pc.example.test:9440/": "https://pc.example.test:9440",
		"https://192.0.2.10:9440":       "https://192.0.2.10:9440",
		"https://[2001:db8::10]:9440/":  "https://[2001:db8::10]:9440",
	}
	for input, want := range tests {
		input, want := input, want
		t.Run(input, func(t *testing.T) {
			t.Parallel()
			origin, err := ParseOrigin(input)
			if err != nil {
				t.Fatalf("ParseOrigin() error = %v", err)
			}
			if got := origin.String(); got != want {
				t.Fatalf("origin = %q, want %q", got, want)
			}
		})
	}
}

func TestOriginRejectsNonOrigins(t *testing.T) {
	t.Parallel()

	invalid := []string{
		"http://pc.example.test",
		"https://",
		"https://user:password@pc.example.test",
		"https://pc.example.test/api",
		"https://pc.example.test//",
		"https://pc.example.test/%2F",
		"https://pc.example.test?query=canary",
		"https://pc.example.test?",
		"https://pc.example.test#fragment",
		"https://pc.example.test#",
		"https://pc.example.test:",
		"https://pc.example.test:0",
		"https://pc.example.test:65536",
		"https://pc.example.test:bad-port",
		"//pc.example.test",
		"pc.example.test",
	}
	for _, input := range invalid {
		input := input
		t.Run(input, func(t *testing.T) {
			t.Parallel()
			if _, err := ParseOrigin(input); err == nil {
				t.Fatalf("ParseOrigin(%q) error = nil, want rejection", input)
			}
		})
	}
}

func TestOriginErrorRedactsInput(t *testing.T) {
	t.Parallel()

	const canary = "origin-password-canary-346d5f55"
	_, err := ParseOrigin("https://user:" + canary + "@pc.example.test/api")
	if err == nil {
		t.Fatal("ParseOrigin() error = nil, want rejection")
	}
	for _, rendered := range []string{err.Error(), fmt.Sprintf("%v", err), fmt.Sprintf("%+v", err)} {
		if strings.Contains(rendered, canary) {
			t.Fatalf("origin error exposes input canary: %q", rendered)
		}
	}
}

func TestOriginBuildsOneEscapedSegmentPerPlaceholder(t *testing.T) {
	t.Parallel()

	origin, err := ParseOrigin("https://pc.example.test:9440/")
	if err != nil {
		t.Fatalf("ParseOrigin() error = %v", err)
	}
	const taskID = "task/+ = :"
	requestURL, err := origin.url(
		"/api/prism/v4.3/config/tasks/{extId}",
		map[string]string{"extId": taskID},
	)
	if err != nil {
		t.Fatalf("origin.url() error = %v", err)
	}

	wantEscapedPath := "/api/prism/v4.3/config/tasks/" + url.PathEscape(taskID)
	if got := requestURL.EscapedPath(); got != wantEscapedPath {
		t.Fatalf("escaped path = %q, want %q", got, wantEscapedPath)
	}
	if got := requestURL.Path; got != "/api/prism/v4.3/config/tasks/"+taskID {
		t.Fatalf("decoded path = %q, want task ID preserved", got)
	}
	if requestURL.Scheme != "https" || requestURL.Host != "pc.example.test:9440" {
		t.Fatalf("request authority = %s://%s, want configured origin", requestURL.Scheme, requestURL.Host)
	}
	if requestURL.RawQuery != "" || requestURL.Fragment != "" {
		t.Fatal("constructed URL unexpectedly contains a query or fragment")
	}
	escapedParameter := strings.TrimPrefix(requestURL.EscapedPath(), "/api/prism/v4.3/config/tasks/")
	if strings.Contains(escapedParameter, "/") {
		t.Fatalf("escaped task ID occupies more than one wire segment: %q", escapedParameter)
	}
}

func TestOriginRejectsUnsafePathConstruction(t *testing.T) {
	t.Parallel()

	origin, err := ParseOrigin("https://pc.example.test:9440")
	if err != nil {
		t.Fatalf("ParseOrigin() error = %v", err)
	}
	tests := []struct {
		name       string
		template   string
		parameters map[string]string
	}{
		{name: "absolute URL", template: "https://evil.example.test/api/tasks/{id}", parameters: map[string]string{"id": "safe"}},
		{name: "authority escape", template: "//evil.example.test/api/tasks/{id}", parameters: map[string]string{"id": "safe"}},
		{name: "outside API root", template: "/tasks/{id}", parameters: map[string]string{"id": "safe"}},
		{name: "query", template: "/api/tasks/{id}?x=1", parameters: map[string]string{"id": "safe"}},
		{name: "fragment", template: "/api/tasks/{id}#x", parameters: map[string]string{"id": "safe"}},
		{name: "pre-encoded template", template: "/api/tasks/%2F/{id}", parameters: map[string]string{"id": "safe"}},
		{name: "template traversal", template: "/api/tasks/../{id}", parameters: map[string]string{"id": "safe"}},
		{name: "partial placeholder", template: "/api/tasks/prefix-{id}", parameters: map[string]string{"id": "safe"}},
		{name: "missing parameter", template: "/api/tasks/{id}", parameters: nil},
		{name: "unused parameter", template: "/api/tasks/{id}", parameters: map[string]string{"id": "safe", "other": "unused"}},
		{name: "empty parameter", template: "/api/tasks/{id}", parameters: map[string]string{"id": ""}},
		{name: "pre-encoded parameter", template: "/api/tasks/{id}", parameters: map[string]string{"id": "task%2Fother"}},
		{name: "direct traversal parameter", template: "/api/tasks/{id}", parameters: map[string]string{"id": ".."}},
		{name: "nested traversal parameter", template: "/api/tasks/{id}", parameters: map[string]string{"id": "task/../other"}},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			requestURL, buildErr := origin.url(test.template, test.parameters)
			if buildErr == nil {
				t.Fatalf("origin.url() = %v, want rejection", requestURL)
			}
			if strings.Contains(buildErr.Error(), "evil.example.test") || strings.Contains(buildErr.Error(), "task%2Fother") {
				t.Fatalf("path-construction error exposes untrusted input: %q", buildErr.Error())
			}
		})
	}
}
