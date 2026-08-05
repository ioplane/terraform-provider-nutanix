package transport

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
)

var (
	// ErrInvalidCAPEM identifies a custom CA bundle that is not strict certificate PEM.
	ErrInvalidCAPEM = errors.New("invalid CA certificate PEM")
	// ErrTLSConflict identifies mutually exclusive TLS configuration.
	ErrTLSConflict = errors.New("custom CA certificate conflicts with insecure TLS mode")
	// ErrSystemRootsUnavailable identifies a failure to load the platform trust store.
	ErrSystemRootsUnavailable = errors.New("system certificate roots are unavailable")
)

// NewTLSConfig constructs the TLS policy for one validated origin.
func NewTLSConfig(origin Origin, insecure bool, caCertificate string) (*tls.Config, error) {
	return newTLSConfig(origin, insecure, caCertificate, x509.SystemCertPool)
}

func newTLSConfig(
	origin Origin,
	insecure bool,
	caCertificate string,
	systemRoots func() (*x509.CertPool, error),
) (*tls.Config, error) {
	if insecure && caCertificate != "" {
		return nil, ErrTLSConflict
	}
	if insecure {
		return &tls.Config{
			MinVersion:         tls.VersionTLS12,
			ServerName:         origin.Hostname(),
			InsecureSkipVerify: true, // #nosec G402 -- enabled only by explicit provider configuration.
		}, nil
	}

	roots, err := systemRoots()
	if err != nil || roots == nil {
		return nil, ErrSystemRootsUnavailable
	}
	if caCertificate != "" {
		if !validCertificatePEM([]byte(caCertificate)) || !roots.AppendCertsFromPEM([]byte(caCertificate)) {
			return nil, ErrInvalidCAPEM
		}
	}

	return &tls.Config{
		MinVersion: tls.VersionTLS12,
		ServerName: origin.Hostname(),
		RootCAs:    roots,
	}, nil
}

func validCertificatePEM(input []byte) bool {
	remaining := bytes.TrimSpace(input)
	if len(remaining) == 0 {
		return false
	}
	for len(remaining) > 0 {
		if !bytes.HasPrefix(remaining, []byte("-----BEGIN CERTIFICATE-----")) {
			return false
		}
		block, rest := pem.Decode(remaining)
		if block == nil || block.Type != "CERTIFICATE" || len(block.Headers) != 0 {
			return false
		}
		if _, err := x509.ParseCertificate(block.Bytes); err != nil {
			return false
		}
		remaining = bytes.TrimSpace(rest)
	}
	return true
}
