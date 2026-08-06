package transport

import (
	"net/http"
	"testing"
	"time"
)

func FuzzParseRetryAfter(f *testing.F) {
	now := time.Date(2026, time.August, 5, 12, 0, 0, 0, time.UTC)
	for _, seed := range []string{
		"0",
		"17",
		" 17 ",
		now.Add(time.Minute).Format(http.TimeFormat),
		now.Add(-time.Second).Format(http.TimeFormat),
		"+1",
		"-1",
		"1.5",
		"9223372036854775808",
		"Fri, 31 Dec 9999 23:59:59 GMT",
		"retry-after-fuzz-secret-canary",
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, value string) {
		delay, ok := parseRetryAfter(value, now)
		if delay < 0 {
			t.Fatalf("parseRetryAfter(%q) returned negative delay %s", value, delay)
		}
		if !ok && delay != 0 {
			t.Fatalf("parseRetryAfter(%q) = %s, false; want zero fallback", value, delay)
		}
	})
}
