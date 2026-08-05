package provider

import (
	"fmt"
	"io"
)

// ConfigurationErrorCode identifies a stable class of provider configuration failure.
type ConfigurationErrorCode string

const (
	// ConfigurationErrorUnknown identifies a Terraform value that is not yet known.
	ConfigurationErrorUnknown ConfigurationErrorCode = "unknown_value"
	// ConfigurationErrorEmpty identifies an explicitly configured empty value.
	ConfigurationErrorEmpty ConfigurationErrorCode = "empty_value"
	// ConfigurationErrorMissing identifies a required effective value that is absent.
	ConfigurationErrorMissing ConfigurationErrorCode = "missing_value"
	// ConfigurationErrorEndpoint identifies an endpoint that is not a strict HTTPS origin.
	ConfigurationErrorEndpoint ConfigurationErrorCode = "invalid_endpoint"
	// ConfigurationErrorAuthentication identifies a missing, partial, or conflicting authentication mode.
	ConfigurationErrorAuthentication ConfigurationErrorCode = "invalid_authentication"
	// ConfigurationErrorUsername identifies an invalid Basic authentication username.
	ConfigurationErrorUsername ConfigurationErrorCode = "invalid_username"
	// ConfigurationErrorAPIKey identifies an invalid API-key value.
	ConfigurationErrorAPIKey ConfigurationErrorCode = "invalid_api_key"
	// ConfigurationErrorBoolean identifies an invalid environment boolean.
	ConfigurationErrorBoolean ConfigurationErrorCode = "invalid_boolean"
	// ConfigurationErrorTimeout identifies an invalid request timeout.
	ConfigurationErrorTimeout ConfigurationErrorCode = "invalid_timeout"
	// ConfigurationErrorTLSConflict identifies mutually exclusive TLS settings.
	ConfigurationErrorTLSConflict ConfigurationErrorCode = "conflicting_tls_settings"
)

// ConfigurationError is a structured, secret-safe provider configuration failure.
type ConfigurationError struct {
	Attribute string
	Code      ConfigurationErrorCode
	summary   string
}

func newConfigurationError(
	attribute string,
	code ConfigurationErrorCode,
	summary string,
) *ConfigurationError {
	return &ConfigurationError{
		Attribute: attribute,
		Code:      code,
		summary:   summary,
	}
}

// Error returns a stable description that never includes a configured value.
func (e *ConfigurationError) Error() string {
	return fmt.Sprintf("invalid provider configuration for %s: %s", e.Attribute, e.summary)
}

// Format keeps every fmt rendering limited to the same secret-safe description.
func (e *ConfigurationError) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, e.Error())
}
