package transport

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"testing"
	"time"
)

func TestTLSUsesSystemRootsAndSecureDefaults(t *testing.T) {
	t.Parallel()

	origin, err := ParseOrigin("https://pc.example.test:9440")
	if err != nil {
		t.Fatalf("ParseOrigin() error = %v", err)
	}
	config, err := NewTLSConfig(origin, false, "")
	if err != nil {
		t.Fatalf("NewTLSConfig() error = %v", err)
	}
	if config.RootCAs == nil {
		t.Fatal("TLS RootCAs = nil, want explicit system trust pool")
	}
	if config.MinVersion != tls.VersionTLS12 {
		t.Fatalf("TLS MinVersion = %d, want TLS 1.2", config.MinVersion)
	}
	if config.ServerName != "pc.example.test" {
		t.Fatalf("TLS ServerName = %q, want endpoint hostname", config.ServerName)
	}
	if config.InsecureSkipVerify {
		t.Fatal("TLS verification disabled by default")
	}
}

func TestTLSAppendsCustomCAPEM(t *testing.T) {
	t.Parallel()

	certificatePEM, certificate := testCACertificate(t, "pc.example.test")
	origin, err := ParseOrigin("https://pc.example.test:9440")
	if err != nil {
		t.Fatalf("ParseOrigin() error = %v", err)
	}
	config, err := NewTLSConfig(origin, false, certificatePEM)
	if err != nil {
		t.Fatalf("NewTLSConfig() error = %v", err)
	}
	if _, err := certificate.Verify(x509.VerifyOptions{
		DNSName: "pc.example.test",
		Roots:   config.RootCAs,
	}); err != nil {
		t.Fatalf("custom CA is not trusted: %v", err)
	}
}

func TestTLSRejectsInvalidCAPEMWithStableCause(t *testing.T) {
	t.Parallel()

	const canary = "invalid-ca-pem-canary-c44eb7b2"
	origin, err := ParseOrigin("https://pc.example.test:9440")
	if err != nil {
		t.Fatalf("ParseOrigin() error = %v", err)
	}
	_, err = NewTLSConfig(origin, false, canary)
	if !errors.Is(err, ErrInvalidCAPEM) {
		t.Fatalf("NewTLSConfig() error = %v, want ErrInvalidCAPEM", err)
	}
	for _, rendered := range []string{err.Error(), fmt.Sprintf("%v", err), fmt.Sprintf("%+v", err)} {
		if strings.Contains(rendered, canary) {
			t.Fatalf("TLS error exposes CA canary: %q", rendered)
		}
	}
}

func TestTLSRejectsInsecureWithCustomCA(t *testing.T) {
	t.Parallel()

	certificatePEM, _ := testCACertificate(t, "pc.example.test")
	origin, err := ParseOrigin("https://pc.example.test:9440")
	if err != nil {
		t.Fatalf("ParseOrigin() error = %v", err)
	}
	_, err = NewTLSConfig(origin, true, certificatePEM)
	if !errors.Is(err, ErrTLSConflict) {
		t.Fatalf("NewTLSConfig() error = %v, want ErrTLSConflict", err)
	}
}

func TestTLSAllowsExplicitInsecureMode(t *testing.T) {
	t.Parallel()

	origin, err := ParseOrigin("https://192.0.2.10:9440")
	if err != nil {
		t.Fatalf("ParseOrigin() error = %v", err)
	}
	config, err := NewTLSConfig(origin, true, "")
	if err != nil {
		t.Fatalf("NewTLSConfig() error = %v", err)
	}
	if !config.InsecureSkipVerify {
		t.Fatal("TLS InsecureSkipVerify = false, want explicit insecure mode")
	}
	if config.ServerName != "192.0.2.10" || config.MinVersion != tls.VersionTLS12 {
		t.Fatal("explicit insecure mode changed server name or TLS minimum")
	}
}

func TestTLSInsecureModeDoesNotLoadSystemRoots(t *testing.T) {
	t.Parallel()

	origin, err := ParseOrigin("https://192.0.2.10:9440")
	if err != nil {
		t.Fatalf("ParseOrigin() error = %v", err)
	}
	var calls int
	failingSystemRoots := func() (*x509.CertPool, error) {
		calls++
		return nil, errors.New("system-roots-canary-must-not-escape")
	}

	config, err := newTLSConfig(origin, true, "", failingSystemRoots)
	if err != nil {
		t.Fatalf("explicit insecure NewTLSConfig() error = %v", err)
	}
	if calls != 0 {
		t.Fatalf("system root loader calls = %d, want 0 in explicit insecure mode", calls)
	}
	if config.RootCAs != nil || !config.InsecureSkipVerify {
		t.Fatal("explicit insecure TLS config unexpectedly owns roots or enables verification")
	}

	_, err = newTLSConfig(origin, false, "", failingSystemRoots)
	if !errors.Is(err, ErrSystemRootsUnavailable) {
		t.Fatalf("verified NewTLSConfig() error = %v, want ErrSystemRootsUnavailable", err)
	}
	if calls != 1 {
		t.Fatalf("verified system root loader calls = %d, want 1", calls)
	}
}

func testCACertificate(t *testing.T, dnsName string) (string, *x509.Certificate) {
	t.Helper()
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate CA private key: %v", err)
	}
	template := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "terraform-provider-nutanix-test-ca"},
		DNSNames:              []string{dnsName},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	if err != nil {
		t.Fatalf("create CA certificate: %v", err)
	}
	certificate, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("parse CA certificate: %v", err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})), certificate
}
