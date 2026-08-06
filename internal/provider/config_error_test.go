package provider

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestConfigurationErrorSupportsErrorsAsWithoutUnwrap(t *testing.T) {
	t.Parallel()

	err := newConfigurationError("endpoint", ConfigurationErrorEndpoint, "must be an HTTPS origin")

	assertConfigurationError(t, err, "endpoint", ConfigurationErrorEndpoint)
	if unwrapped := errors.Unwrap(err); unwrapped != nil {
		t.Fatalf("errors.Unwrap() = %v, want nil", unwrapped)
	}
}

func TestConfigurationErrorDoesNotExposeConfiguredValue(t *testing.T) {
	t.Parallel()

	const canary = "configuration-canary-secret-6bd5984d"
	model := nullProviderConfig()
	model.Endpoint = types.StringValue("https://pc.example.test")
	model.APIKey = types.StringValue(canary + " invalid")
	_, err := resolveConfig(model, mapEnvironment(nil))
	assertConfigurationError(t, err, "api_key", ConfigurationErrorAPIKey)

	assertErrorRenderingsRedact(t, err, canary)
}

func TestResolveConfigErrorChainDoesNotExposeConfiguredValues(t *testing.T) {
	t.Parallel()

	const canary = "resolver-error-chain-canary-99c67c"
	tests := []struct {
		name        string
		model       providerConfig
		environment map[string]string
	}{
		{
			name:  "malformed endpoint",
			model: withEndpoint(types.StringValue("https://user:" + canary + "@pc.example.test:%zz")),
			environment: map[string]string{
				"NUTANIX_API_KEY": "api-key",
			},
		},
		{
			name:  "malformed timeout",
			model: nullProviderConfig(),
			environment: map[string]string{
				"NUTANIX_ENDPOINT":                "https://pc.example.test",
				"NUTANIX_API_KEY":                 "api-key",
				"NUTANIX_REQUEST_TIMEOUT_SECONDS": canary,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := resolveConfig(test.model, mapEnvironment(test.environment))
			if err == nil {
				t.Fatal("resolveConfig() error = nil, want validation failure")
			}
			for current := err; current != nil; current = errors.Unwrap(current) {
				if strings.Contains(current.Error(), canary) {
					t.Fatalf("configuration error chain exposes a configured-value canary: %q", current.Error())
				}
			}
		})
	}
}

func TestResolveConfigSecretErrorRedaction(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		model         providerConfig
		canary        string
		wantAttribute string
		wantCode      ConfigurationErrorCode
	}{
		{
			name: "password in authentication conflict",
			model: func() providerConfig {
				model := nullProviderConfig()
				model.Endpoint = types.StringValue("https://pc.example.test")
				model.Username = types.StringValue("admin")
				model.Password = types.StringValue("password-canary-5d49471f")
				model.APIKey = types.StringValue("safe-api-key")
				return model
			}(),
			canary:        "password-canary-5d49471f",
			wantAttribute: "api_key",
			wantCode:      ConfigurationErrorAuthentication,
		},
		{
			name: "CA bundle in insecure conflict",
			model: func() providerConfig {
				model := nullProviderConfig()
				model.Endpoint = types.StringValue("https://pc.example.test")
				model.APIKey = types.StringValue("safe-api-key")
				model.Insecure = types.BoolValue(true)
				model.CACertificate = types.StringValue("ca-bundle-canary-20f8694e")
				return model
			}(),
			canary:        "ca-bundle-canary-20f8694e",
			wantAttribute: "ca_certificate",
			wantCode:      ConfigurationErrorTLSConflict,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := resolveConfig(test.model, mapEnvironment(nil))
			assertConfigurationError(t, err, test.wantAttribute, test.wantCode)
			assertErrorRenderingsRedact(t, err, test.canary)
		})
	}
}

func assertConfigurationError(
	t *testing.T,
	err error,
	wantAttribute string,
	wantCode ConfigurationErrorCode,
) {
	t.Helper()
	if err == nil {
		t.Fatalf("resolveConfig() error = nil, want %s for %s", wantCode, wantAttribute)
	}
	var configurationError *ConfigurationError
	if !errors.As(err, &configurationError) {
		t.Fatalf("error type = %T, want *ConfigurationError", err)
	}
	if configurationError.Attribute != wantAttribute {
		t.Errorf("configuration error attribute = %q, want %q", configurationError.Attribute, wantAttribute)
	}
	if configurationError.Code != wantCode {
		t.Errorf("configuration error code = %q, want %q", configurationError.Code, wantCode)
	}
}

func assertErrorRenderingsRedact(t *testing.T, err error, canaries ...string) {
	t.Helper()
	for _, rendered := range []string{
		err.Error(),
		fmt.Sprintf("%s", err),
		fmt.Sprintf("%q", err),
		fmt.Sprintf("%v", err),
		fmt.Sprintf("%+v", err),
		fmt.Sprintf("%#v", err),
	} {
		for _, canary := range canaries {
			if strings.Contains(rendered, canary) {
				t.Fatalf("configuration error exposes a secret canary: %q", rendered)
			}
		}
	}
}
