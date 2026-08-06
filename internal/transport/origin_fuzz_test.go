package transport

import (
	"net/url"
	"strings"
	"testing"
)

func FuzzOriginAndPathConstruction(f *testing.F) {
	for _, seed := range []struct {
		origin string
		id     string
	}{
		{origin: "https://pc.example.test:9440", id: "task/+ = :"},
		{origin: "https://[2001:db8::10]:9440/", id: "3d3971b7-4f46-4f15-9049-b5035b9c4113"},
		{origin: "https://user:secret@pc.example.test", id: "safe"},
		{origin: "https://pc.example.test/api", id: "../escape"},
		{origin: "http://pc.example.test", id: "task%2Fother"},
	} {
		f.Add(seed.origin, seed.id)
	}

	f.Fuzz(func(t *testing.T, rawOrigin, taskID string) {
		origin, err := ParseOrigin(rawOrigin)
		if err != nil {
			return
		}
		requestURL, err := origin.url(
			"/api/prism/v4.3/config/tasks/{extId}",
			map[string]string{"extId": taskID},
		)
		if err != nil {
			return
		}
		if requestURL.Scheme != "https" || requestURL.Host != origin.host {
			t.Fatalf("constructed URL escaped configured authority: %s", requestURL.Redacted())
		}
		if requestURL.RawQuery != "" || requestURL.Fragment != "" {
			t.Fatalf("constructed URL contains query or fragment: %s", requestURL.Redacted())
		}
		prefix := "/api/prism/v4.3/config/tasks/"
		escapedPath := requestURL.EscapedPath()
		if !strings.HasPrefix(escapedPath, prefix) {
			t.Fatalf("escaped path = %q, want API template prefix", escapedPath)
		}
		escapedID := strings.TrimPrefix(escapedPath, prefix)
		if strings.Contains(escapedID, "/") {
			t.Fatalf("path parameter spans multiple wire segments: %q", escapedID)
		}
		if unescaped, unescapeErr := url.PathUnescape(escapedID); unescapeErr != nil || unescaped != taskID {
			t.Fatalf("path parameter round trip = %q, %v; want original", unescaped, unescapeErr)
		}
	})
}
