// Package transport implements the vendor-neutral, origin-bound HTTP kernel.
package transport

import (
	"errors"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"
)

var (
	errInvalidOrigin        = errors.New("origin must be a strict HTTPS origin")
	errInvalidPathTemplate  = errors.New("path template must be a relative API path with complete segment placeholders")
	errInvalidPathParameter = errors.New(
		"path parameter must be an unencoded, non-traversing segment value",
	)
)

// Origin is a validated, normalized HTTPS authority.
type Origin struct {
	host string
}

// ParseOrigin validates and normalizes a strict HTTPS origin.
func ParseOrigin(raw string) (Origin, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return Origin{}, errInvalidOrigin
	}

	invalidPort := strings.HasSuffix(parsed.Host, ":")
	if port := parsed.Port(); port != "" {
		value, parseErr := strconv.ParseUint(port, 10, 16)
		invalidPort = parseErr != nil || value == 0
	}
	if strings.Contains(raw, "#") ||
		invalidPort ||
		parsed.Scheme != "https" ||
		parsed.Host == "" ||
		parsed.Hostname() == "" ||
		parsed.User != nil ||
		parsed.Opaque != "" ||
		parsed.RawQuery != "" ||
		parsed.ForceQuery ||
		parsed.Fragment != "" ||
		parsed.RawFragment != "" ||
		parsed.RawPath != "" ||
		(parsed.Path != "" && parsed.Path != "/") {
		return Origin{}, errInvalidOrigin
	}

	return Origin{host: parsed.Host}, nil
}

// String returns the normalized origin without a trailing slash.
func (o Origin) String() string {
	return "https://" + o.host
}

// Hostname returns the origin host without an optional port.
func (o Origin) Hostname() string {
	parsed := &url.URL{Scheme: "https", Host: o.host}
	return parsed.Hostname()
}

func (o Origin) url(pathTemplate string, parameters map[string]string) (*url.URL, error) {
	if o.host == "" ||
		(!strings.HasPrefix(pathTemplate, "/api/") && pathTemplate != "/api") ||
		strings.HasPrefix(pathTemplate, "//") ||
		strings.ContainsAny(pathTemplate, "?#%\\") {
		return nil, errInvalidPathTemplate
	}

	segments := strings.Split(pathTemplate, "/")
	if len(segments) < 2 || segments[0] != "" {
		return nil, errInvalidPathTemplate
	}
	decoded := make([]string, 0, len(segments))
	escaped := make([]string, 0, len(segments))
	used := make(map[string]struct{}, len(parameters))
	for index, segment := range segments {
		if index == 0 {
			decoded = append(decoded, "")
			escaped = append(escaped, "")
			continue
		}
		if segment == "" || segment == "." || segment == ".." {
			return nil, errInvalidPathTemplate
		}

		name, placeholder := parsePlaceholder(segment)
		if placeholder {
			value, ok := parameters[name]
			if !ok || !validPathParameter(value) {
				return nil, errInvalidPathParameter
			}
			used[name] = struct{}{}
			decoded = append(decoded, value)
			escaped = append(escaped, url.PathEscape(value))
			continue
		}
		if strings.ContainsAny(segment, "{}") || !validLiteralSegment(segment) {
			return nil, errInvalidPathTemplate
		}
		decoded = append(decoded, segment)
		escaped = append(escaped, segment)
	}
	if len(used) != len(parameters) {
		return nil, errInvalidPathParameter
	}

	return &url.URL{
		Scheme:  "https",
		Host:    o.host,
		Path:    strings.Join(decoded, "/"),
		RawPath: strings.Join(escaped, "/"),
	}, nil
}

func parsePlaceholder(segment string) (string, bool) {
	if len(segment) < 3 || segment[0] != '{' || segment[len(segment)-1] != '}' {
		return "", false
	}
	name := segment[1 : len(segment)-1]
	for index, character := range name {
		if (character >= 'a' && character <= 'z') ||
			(character >= 'A' && character <= 'Z') ||
			(index > 0 && character >= '0' && character <= '9') ||
			(index > 0 && character == '_') {
			continue
		}
		return "", false
	}
	return name, true
}

func validLiteralSegment(segment string) bool {
	for _, character := range segment {
		if (character >= 'a' && character <= 'z') ||
			(character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') ||
			strings.ContainsRune("-._~", character) {
			continue
		}
		return false
	}
	return true
}

func validPathParameter(value string) bool {
	if value == "" || !utf8.ValidString(value) {
		return false
	}
	for index := 0; index < len(value); index++ {
		if value[index] < 0x20 || value[index] == 0x7f {
			return false
		}
		if value[index] == '%' && index+2 < len(value) && isHex(value[index+1]) && isHex(value[index+2]) {
			return false
		}
	}
	for _, segment := range strings.Split(value, "/") {
		if segment == "." || segment == ".." {
			return false
		}
	}
	return true
}

func isHex(value byte) bool {
	return (value >= '0' && value <= '9') ||
		(value >= 'a' && value <= 'f') ||
		(value >= 'A' && value <= 'F')
}
