// Package queryid derives stable opaque identities for Terraform list data sources.
package queryid

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/url"
	"strings"
	"unicode/utf8"
)

const identityPrefix = "ioplane/nutanix/query-id/v1\x00"

var (
	// ErrInvalidInput identifies a type name or query outside the identity contract.
	ErrInvalidInput = errors.New("query identity input is invalid")
)

// New returns the normative lowercase SHA-256 identity for caller query values.
func New(terraformTypeName string, values url.Values) (string, error) {
	if !validTypeName(terraformTypeName) || !validValues(values) {
		return "", ErrInvalidInput
	}
	hash := sha256.New()
	_, _ = hash.Write([]byte(identityPrefix))
	_, _ = hash.Write([]byte(terraformTypeName))
	_, _ = hash.Write([]byte{0})
	_, _ = hash.Write([]byte(values.Encode()))
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func validTypeName(value string) bool {
	if value == "" || len(value) > 128 || value[0] < 'a' || value[0] > 'z' {
		return false
	}
	for index := 1; index < len(value); index++ {
		character := value[index]
		if (character >= 'a' && character <= 'z') ||
			(character >= '0' && character <= '9') || character == '_' {
			continue
		}
		return false
	}
	return true
}

func validValues(values url.Values) bool {
	for key, entries := range values {
		if key == "" || len(entries) != 1 || !utf8.ValidString(key) ||
			!utf8.ValidString(entries[0]) || strings.ContainsRune(key, '\x00') ||
			strings.ContainsRune(entries[0], '\x00') {
			return false
		}
	}
	return true
}
