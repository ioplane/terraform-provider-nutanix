package provider

import (
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestResolveConfigPrecedence(t *testing.T) {
	t.Parallel()

	model := nullProviderConfig()
	model.Endpoint = types.StringValue("https://terraform.example.test:9440/")
	model.APIKey = types.StringValue("terraform-api-key")
	model.Insecure = types.BoolValue(false)
	model.RequestTimeoutSeconds = types.Int64Value(42)

	got, err := resolveConfig(model, mapEnvironment(map[string]string{
		"NUTANIX_ENDPOINT":                "https://environment.example.test:9440",
		"NUTANIX_API_KEY":                 "environment-api-key",
		"NUTANIX_INSECURE":                "true",
		"NUTANIX_REQUEST_TIMEOUT_SECONDS": "73",
	}))
	if err != nil {
		t.Fatalf("resolveConfig() error = %v", err)
	}

	wantURL, err := url.Parse("https://terraform.example.test:9440")
	if err != nil {
		t.Fatalf("parse expected URL: %v", err)
	}
	wantSafe := resolvedConfig{
		endpoint:       wantURL,
		insecure:       false,
		requestTimeout: 42 * time.Second,
	}
	gotSafe := resolvedConfig{
		endpoint:       got.endpoint,
		insecure:       got.insecure,
		requestTimeout: got.requestTimeout,
	}
	if diff := cmp.Diff(wantSafe, gotSafe, cmp.AllowUnexported(resolvedConfig{})); diff != "" {
		t.Fatalf("safe resolved configuration mismatch (-want +got):\n%s", diff)
	}
	if got.apiKey != "terraform-api-key" {
		t.Fatal("explicit API key did not take precedence over the environment")
	}
	if got.username != "" || got.password != "" {
		t.Fatal("API-key configuration unexpectedly resolved Basic credentials")
	}
}

func TestResolveConfigBasicAndCAPrecedence(t *testing.T) {
	t.Parallel()

	model := nullProviderConfig()
	model.Endpoint = types.StringValue("https://terraform.example.test:9440")
	model.Username = types.StringValue("terraform-user")
	model.Password = types.StringValue("terraform-password")
	model.CACertificate = types.StringValue("terraform-ca")
	model.Insecure = types.BoolValue(false)
	model.RequestTimeoutSeconds = types.Int64Value(42)

	got, err := resolveConfig(model, mapEnvironment(map[string]string{
		"NUTANIX_ENDPOINT":                "https://environment.example.test:9440",
		"NUTANIX_USERNAME":                "environment-user",
		"NUTANIX_PASSWORD":                "environment-password",
		"NUTANIX_CA_CERTIFICATE":          "environment-ca",
		"NUTANIX_INSECURE":                "true",
		"NUTANIX_REQUEST_TIMEOUT_SECONDS": "73",
	}))
	if err != nil {
		t.Fatalf("resolveConfig() error = %v", err)
	}
	if got.endpoint.String() != "https://terraform.example.test:9440" {
		t.Fatal("explicit endpoint did not take precedence over the environment")
	}
	if got.username != "terraform-user" || got.password != "terraform-password" {
		t.Fatal("explicit Basic credentials did not take precedence over the environment")
	}
	if got.caCertificate != "terraform-ca" {
		t.Fatal("explicit CA certificate did not take precedence over the environment")
	}
	if got.insecure || got.requestTimeout != 42*time.Second {
		t.Fatal("explicit TLS mode or timeout did not take precedence over the environment")
	}
}

func TestResolveConfigNullUsesEnvironmentAndDefaults(t *testing.T) {
	t.Parallel()

	got, err := resolveConfig(nullProviderConfig(), mapEnvironment(map[string]string{
		"NUTANIX_ENDPOINT": "https://pc.example.test:9440/",
		"NUTANIX_USERNAME": "admin",
		"NUTANIX_PASSWORD": "environment-password",
	}))
	if err != nil {
		t.Fatalf("resolveConfig() error = %v", err)
	}
	if got.endpoint.String() != "https://pc.example.test:9440" {
		t.Fatalf("resolved endpoint = %q, want normalized origin", got.endpoint.String())
	}
	if got.username != "admin" || got.password != "environment-password" || got.apiKey != "" {
		t.Fatal("environment Basic credentials were not resolved")
	}
	if got.insecure {
		t.Fatal("insecure default = true, want false")
	}
	if got.requestTimeout != 60*time.Second {
		t.Fatalf("request timeout = %s, want 1m0s", got.requestTimeout)
	}
}

func TestResolveConfigValidation(t *testing.T) {
	t.Parallel()

	longAPIKey := strings.Repeat("a", 4097)
	validLongAPIKey := strings.Repeat("a", 4096)
	tests := []struct {
		name          string
		model         providerConfig
		environment   map[string]string
		wantAttribute string
		wantCode      ConfigurationErrorCode
	}{
		{
			name:          "unknown endpoint",
			model:         withEndpoint(types.StringUnknown()),
			wantAttribute: "endpoint",
			wantCode:      ConfigurationErrorUnknown,
		},
		{
			name:          "explicit empty endpoint does not fall back",
			model:         withEndpoint(types.StringValue("")),
			environment:   validAPIKeyEnvironment(),
			wantAttribute: "endpoint",
			wantCode:      ConfigurationErrorEmpty,
		},
		{
			name:          "missing endpoint",
			model:         nullProviderConfig(),
			environment:   map[string]string{"NUTANIX_API_KEY": "api-key"},
			wantAttribute: "endpoint",
			wantCode:      ConfigurationErrorMissing,
		},
		{
			name:          "environment endpoint empty",
			model:         nullProviderConfig(),
			environment:   map[string]string{"NUTANIX_ENDPOINT": "", "NUTANIX_API_KEY": "api-key"},
			wantAttribute: "endpoint",
			wantCode:      ConfigurationErrorEmpty,
		},
		{
			name:          "unknown password",
			model:         withPassword(types.StringUnknown()),
			environment:   validAPIKeyEnvironment(),
			wantAttribute: "password",
			wantCode:      ConfigurationErrorUnknown,
		},
		{
			name:          "explicit empty secret does not fall back",
			model:         withAPIKey(types.StringValue("")),
			environment:   validAPIKeyEnvironment(),
			wantAttribute: "api_key",
			wantCode:      ConfigurationErrorEmpty,
		},
		{
			name:          "unknown insecure",
			model:         withInsecure(types.BoolUnknown()),
			environment:   validAPIKeyEnvironment(),
			wantAttribute: "insecure",
			wantCode:      ConfigurationErrorUnknown,
		},
		{
			name:          "unknown timeout",
			model:         withTimeout(types.Int64Unknown()),
			environment:   validAPIKeyEnvironment(),
			wantAttribute: "request_timeout_seconds",
			wantCode:      ConfigurationErrorUnknown,
		},
		{
			name:          "no authentication",
			model:         nullProviderConfig(),
			environment:   map[string]string{"NUTANIX_ENDPOINT": "https://pc.example.test"},
			wantAttribute: "username",
			wantCode:      ConfigurationErrorAuthentication,
		},
		{
			name:          "username without password",
			model:         nullProviderConfig(),
			environment:   map[string]string{"NUTANIX_ENDPOINT": "https://pc.example.test", "NUTANIX_USERNAME": "admin"},
			wantAttribute: "password",
			wantCode:      ConfigurationErrorAuthentication,
		},
		{
			name:          "password without username",
			model:         nullProviderConfig(),
			environment:   map[string]string{"NUTANIX_ENDPOINT": "https://pc.example.test", "NUTANIX_PASSWORD": "secret"},
			wantAttribute: "username",
			wantCode:      ConfigurationErrorAuthentication,
		},
		{
			name:          "both authentication modes",
			model:         nullProviderConfig(),
			environment:   map[string]string{"NUTANIX_ENDPOINT": "https://pc.example.test", "NUTANIX_USERNAME": "admin", "NUTANIX_PASSWORD": "secret", "NUTANIX_API_KEY": "api-key"},
			wantAttribute: "api_key",
			wantCode:      ConfigurationErrorAuthentication,
		},
		{
			name:          "username contains colon",
			model:         nullProviderConfig(),
			environment:   map[string]string{"NUTANIX_ENDPOINT": "https://pc.example.test", "NUTANIX_USERNAME": "admin:name", "NUTANIX_PASSWORD": "secret"},
			wantAttribute: "username",
			wantCode:      ConfigurationErrorUsername,
		},
		{
			name:          "API key contains space",
			model:         nullProviderConfig(),
			environment:   map[string]string{"NUTANIX_ENDPOINT": "https://pc.example.test", "NUTANIX_API_KEY": "api key"},
			wantAttribute: "api_key",
			wantCode:      ConfigurationErrorAPIKey,
		},
		{
			name:          "API key contains non-ASCII",
			model:         nullProviderConfig(),
			environment:   map[string]string{"NUTANIX_ENDPOINT": "https://pc.example.test", "NUTANIX_API_KEY": "api-key-ключ"},
			wantAttribute: "api_key",
			wantCode:      ConfigurationErrorAPIKey,
		},
		{
			name:          "API key contains tab",
			model:         nullProviderConfig(),
			environment:   map[string]string{"NUTANIX_ENDPOINT": "https://pc.example.test", "NUTANIX_API_KEY": "api\tkey"},
			wantAttribute: "api_key",
			wantCode:      ConfigurationErrorAPIKey,
		},
		{
			name:          "API key contains newline",
			model:         nullProviderConfig(),
			environment:   map[string]string{"NUTANIX_ENDPOINT": "https://pc.example.test", "NUTANIX_API_KEY": "api\nkey"},
			wantAttribute: "api_key",
			wantCode:      ConfigurationErrorAPIKey,
		},
		{
			name:          "API key too long",
			model:         nullProviderConfig(),
			environment:   map[string]string{"NUTANIX_ENDPOINT": "https://pc.example.test", "NUTANIX_API_KEY": longAPIKey},
			wantAttribute: "api_key",
			wantCode:      ConfigurationErrorAPIKey,
		},
		{
			name:          "environment bool is not strict",
			model:         nullProviderConfig(),
			environment:   map[string]string{"NUTANIX_ENDPOINT": "https://pc.example.test", "NUTANIX_API_KEY": "api-key", "NUTANIX_INSECURE": "1"},
			wantAttribute: "insecure",
			wantCode:      ConfigurationErrorBoolean,
		},
		{
			name:          "environment bool has whitespace",
			model:         nullProviderConfig(),
			environment:   map[string]string{"NUTANIX_ENDPOINT": "https://pc.example.test", "NUTANIX_API_KEY": "api-key", "NUTANIX_INSECURE": " true"},
			wantAttribute: "insecure",
			wantCode:      ConfigurationErrorBoolean,
		},
		{
			name:          "environment timeout is not decimal",
			model:         nullProviderConfig(),
			environment:   map[string]string{"NUTANIX_ENDPOINT": "https://pc.example.test", "NUTANIX_API_KEY": "api-key", "NUTANIX_REQUEST_TIMEOUT_SECONDS": "0x3c"},
			wantAttribute: "request_timeout_seconds",
			wantCode:      ConfigurationErrorTimeout,
		},
		{
			name:          "explicit timeout below range",
			model:         withTimeout(types.Int64Value(0)),
			environment:   validAPIKeyEnvironment(),
			wantAttribute: "request_timeout_seconds",
			wantCode:      ConfigurationErrorTimeout,
		},
		{
			name:          "environment timeout above range",
			model:         nullProviderConfig(),
			environment:   map[string]string{"NUTANIX_ENDPOINT": "https://pc.example.test", "NUTANIX_API_KEY": "api-key", "NUTANIX_REQUEST_TIMEOUT_SECONDS": "601"},
			wantAttribute: "request_timeout_seconds",
			wantCode:      ConfigurationErrorTimeout,
		},
		{
			name:          "insecure conflicts with CA",
			model:         nullProviderConfig(),
			environment:   map[string]string{"NUTANIX_ENDPOINT": "https://pc.example.test", "NUTANIX_API_KEY": "api-key", "NUTANIX_INSECURE": "TRUE", "NUTANIX_CA_CERTIFICATE": "canary-ca"},
			wantAttribute: "ca_certificate",
			wantCode:      ConfigurationErrorTLSConflict,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			_, err := resolveConfig(test.model, mapEnvironment(test.environment))
			assertConfigurationError(t, err, test.wantAttribute, test.wantCode)
		})
	}

	t.Run("visible ASCII API key at maximum length", func(t *testing.T) {
		t.Parallel()
		_, err := resolveConfig(nullProviderConfig(), mapEnvironment(map[string]string{
			"NUTANIX_ENDPOINT": "https://pc.example.test",
			"NUTANIX_API_KEY":  validLongAPIKey,
		}))
		if err != nil {
			t.Fatalf("resolveConfig() error = %v", err)
		}
	})

	for _, value := range []string{"true", "TRUE", "False", "false"} {
		t.Run("strict bool "+value, func(t *testing.T) {
			t.Parallel()
			_, err := resolveConfig(nullProviderConfig(), mapEnvironment(map[string]string{
				"NUTANIX_ENDPOINT": "https://pc.example.test",
				"NUTANIX_API_KEY":  "api-key",
				"NUTANIX_INSECURE": value,
			}))
			if err != nil {
				t.Fatalf("resolveConfig() error = %v", err)
			}
		})
	}

	for _, seconds := range []string{"1", "600"} {
		t.Run("timeout boundary "+seconds, func(t *testing.T) {
			t.Parallel()
			got, err := resolveConfig(nullProviderConfig(), mapEnvironment(map[string]string{
				"NUTANIX_ENDPOINT":                "https://pc.example.test",
				"NUTANIX_API_KEY":                 "api-key",
				"NUTANIX_REQUEST_TIMEOUT_SECONDS": seconds,
			}))
			if err != nil {
				t.Fatalf("resolveConfig() error = %v", err)
			}
			wantDuration, err := time.ParseDuration(seconds + "s")
			if err != nil {
				t.Fatalf("parse expected timeout: %v", err)
			}
			if got.requestTimeout != wantDuration {
				t.Fatalf("request timeout = %s, want %s", got.requestTimeout, wantDuration)
			}
		})
	}
}

func TestResolveConfigRejectsEveryUnknownAndExplicitEmptyString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		attribute string
		unknown   func(*providerConfig)
		empty     func(*providerConfig)
	}{
		{"endpoint", func(model *providerConfig) { model.Endpoint = types.StringUnknown() }, func(model *providerConfig) { model.Endpoint = types.StringValue("") }},
		{"username", func(model *providerConfig) { model.Username = types.StringUnknown() }, func(model *providerConfig) { model.Username = types.StringValue("") }},
		{"password", func(model *providerConfig) { model.Password = types.StringUnknown() }, func(model *providerConfig) { model.Password = types.StringValue("") }},
		{"api_key", func(model *providerConfig) { model.APIKey = types.StringUnknown() }, func(model *providerConfig) { model.APIKey = types.StringValue("") }},
		{"ca_certificate", func(model *providerConfig) { model.CACertificate = types.StringUnknown() }, func(model *providerConfig) { model.CACertificate = types.StringValue("") }},
	}

	for _, test := range tests {
		t.Run(test.attribute+" unknown", func(t *testing.T) {
			t.Parallel()
			model := nullProviderConfig()
			test.unknown(&model)
			_, err := resolveConfig(model, mapEnvironment(validAPIKeyEnvironment()))
			assertConfigurationError(t, err, test.attribute, ConfigurationErrorUnknown)
		})
		t.Run(test.attribute+" explicit empty", func(t *testing.T) {
			t.Parallel()
			model := nullProviderConfig()
			test.empty(&model)
			_, err := resolveConfig(model, mapEnvironment(validAPIKeyEnvironment()))
			assertConfigurationError(t, err, test.attribute, ConfigurationErrorEmpty)
		})
	}
}

func TestResolveConfigEndpointGrammar(t *testing.T) {
	t.Parallel()

	invalid := []string{
		"http://pc.example.test",
		"https://",
		"https://user:password@pc.example.test",
		"https://pc.example.test/api",
		"https://pc.example.test//",
		"https://pc.example.test/%2F",
		"https://pc.example.test?query=canary",
		"https://pc.example.test#",
		"https://pc.example.test/#",
		"https://pc.example.test#fragment",
		"https://pc.example.test:",
		"https://pc.example.test:0",
		"https://pc.example.test:65536",
		"https://pc.example.test:bad-port",
		"//pc.example.test",
		"pc.example.test",
	}
	for _, endpoint := range invalid {
		endpoint := endpoint
		t.Run(endpoint, func(t *testing.T) {
			t.Parallel()
			model := nullProviderConfig()
			model.Endpoint = types.StringValue(endpoint)
			model.APIKey = types.StringValue("api-key")
			_, err := resolveConfig(model, mapEnvironment(nil))
			assertConfigurationError(t, err, "endpoint", ConfigurationErrorEndpoint)
		})
	}
}

func TestResolveConfigAcceptsHTTPSOrigins(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"https://pc.example.test":       "https://pc.example.test",
		"https://pc.example.test/":      "https://pc.example.test",
		"https://pc.example.test:1":     "https://pc.example.test:1",
		"https://pc.example.test:9440":  "https://pc.example.test:9440",
		"https://pc.example.test:65535": "https://pc.example.test:65535",
		"https://192.0.2.10:9440":       "https://192.0.2.10:9440",
		"https://[2001:db8::10]:9440/":  "https://[2001:db8::10]:9440",
	}
	for endpoint, want := range tests {
		endpoint, want := endpoint, want
		t.Run(endpoint, func(t *testing.T) {
			t.Parallel()
			model := nullProviderConfig()
			model.Endpoint = types.StringValue(endpoint)
			model.APIKey = types.StringValue("api-key")
			got, err := resolveConfig(model, mapEnvironment(nil))
			if err != nil {
				t.Fatalf("resolveConfig() error = %v", err)
			}
			if got.endpoint.String() != want {
				t.Fatalf("endpoint = %q, want %q", got.endpoint.String(), want)
			}
		})
	}
}

func nullProviderConfig() providerConfig {
	return providerConfig{
		Endpoint:              types.StringNull(),
		Username:              types.StringNull(),
		Password:              types.StringNull(),
		APIKey:                types.StringNull(),
		Insecure:              types.BoolNull(),
		CACertificate:         types.StringNull(),
		RequestTimeoutSeconds: types.Int64Null(),
	}
}

func withEndpoint(value types.String) providerConfig {
	model := nullProviderConfig()
	model.Endpoint = value
	return model
}

func withPassword(value types.String) providerConfig {
	model := nullProviderConfig()
	model.Password = value
	return model
}

func withAPIKey(value types.String) providerConfig {
	model := nullProviderConfig()
	model.APIKey = value
	return model
}

func withInsecure(value types.Bool) providerConfig {
	model := nullProviderConfig()
	model.Insecure = value
	return model
}

func withTimeout(value types.Int64) providerConfig {
	model := nullProviderConfig()
	model.RequestTimeoutSeconds = value
	return model
}

func validAPIKeyEnvironment() map[string]string {
	return map[string]string{
		"NUTANIX_ENDPOINT": "https://pc.example.test",
		"NUTANIX_API_KEY":  "api-key",
	}
}

func mapEnvironment(values map[string]string) environmentLookup {
	return func(name string) (string, bool) {
		value, ok := values[name]
		return value, ok
	}
}
