package provider

import (
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

const (
	defaultRequestTimeoutSeconds int64 = 60
	minimumRequestTimeoutSeconds int64 = 1
	maximumRequestTimeoutSeconds int64 = 600
	maximumAPIKeyBytes                 = 4096
)

type providerConfig struct {
	Endpoint              types.String `tfsdk:"endpoint"`
	Username              types.String `tfsdk:"username"`
	Password              types.String `tfsdk:"password"`
	APIKey                types.String `tfsdk:"api_key"`
	Insecure              types.Bool   `tfsdk:"insecure"`
	CACertificate         types.String `tfsdk:"ca_certificate"`
	RequestTimeoutSeconds types.Int64  `tfsdk:"request_timeout_seconds"`
}

type resolvedConfig struct {
	endpoint       *url.URL
	username       string
	password       string
	apiKey         string
	insecure       bool
	caCertificate  string
	requestTimeout time.Duration
}

type environmentLookup func(string) (string, bool)

func resolveConfig(model providerConfig, lookup environmentLookup) (resolvedConfig, error) {
	endpointText, endpointSet, err := resolveString(model.Endpoint, lookup, "endpoint", "NUTANIX_ENDPOINT")
	if err != nil {
		return resolvedConfig{}, err
	}
	if !endpointSet {
		return resolvedConfig{}, newConfigurationError(
			"endpoint",
			ConfigurationErrorMissing,
			"set endpoint or NUTANIX_ENDPOINT",
		)
	}
	endpoint, err := parseEndpoint(endpointText)
	if err != nil {
		return resolvedConfig{}, err
	}

	username, usernameSet, err := resolveString(model.Username, lookup, "username", "NUTANIX_USERNAME")
	if err != nil {
		return resolvedConfig{}, err
	}
	password, passwordSet, err := resolveString(model.Password, lookup, "password", "NUTANIX_PASSWORD")
	if err != nil {
		return resolvedConfig{}, err
	}
	apiKey, apiKeySet, err := resolveString(model.APIKey, lookup, "api_key", "NUTANIX_API_KEY")
	if err != nil {
		return resolvedConfig{}, err
	}
	caCertificate, caCertificateSet, err := resolveString(
		model.CACertificate,
		lookup,
		"ca_certificate",
		"NUTANIX_CA_CERTIFICATE",
	)
	if err != nil {
		return resolvedConfig{}, err
	}

	insecure, err := resolveBool(model.Insecure, lookup)
	if err != nil {
		return resolvedConfig{}, err
	}
	requestTimeoutSeconds, err := resolveTimeout(model.RequestTimeoutSeconds, lookup)
	if err != nil {
		return resolvedConfig{}, err
	}

	if usernameSet != passwordSet {
		missing := "password"
		if passwordSet {
			missing = "username"
		}
		return resolvedConfig{}, newConfigurationError(
			missing,
			ConfigurationErrorAuthentication,
			"Basic authentication requires both username and password",
		)
	}
	if usernameSet && apiKeySet {
		return resolvedConfig{}, newConfigurationError(
			"api_key",
			ConfigurationErrorAuthentication,
			"configure either Basic authentication or an API key, not both",
		)
	}
	if !usernameSet && !apiKeySet {
		return resolvedConfig{}, newConfigurationError(
			"username",
			ConfigurationErrorAuthentication,
			"configure Basic authentication or an API key",
		)
	}
	if strings.Contains(username, ":") {
		return resolvedConfig{}, newConfigurationError(
			"username",
			ConfigurationErrorUsername,
			"Basic authentication username must not contain a colon",
		)
	}
	if apiKeySet && !isVisibleASCII(apiKey) {
		return resolvedConfig{}, newConfigurationError(
			"api_key",
			ConfigurationErrorAPIKey,
			"API key must contain 1 through 4096 visible ASCII bytes",
		)
	}
	if insecure && caCertificateSet {
		return resolvedConfig{}, newConfigurationError(
			"ca_certificate",
			ConfigurationErrorTLSConflict,
			"custom CA certificate conflicts with insecure TLS mode",
		)
	}

	return resolvedConfig{
		endpoint:       endpoint,
		username:       username,
		password:       password,
		apiKey:         apiKey,
		insecure:       insecure,
		caCertificate:  caCertificate,
		requestTimeout: time.Duration(requestTimeoutSeconds) * time.Second,
	}, nil
}

func resolveString(
	value types.String,
	lookup environmentLookup,
	attribute string,
	environmentName string,
) (string, bool, error) {
	if value.IsUnknown() {
		return "", false, newConfigurationError(
			attribute,
			ConfigurationErrorUnknown,
			"value must be known during provider configuration",
		)
	}
	if !value.IsNull() {
		if value.ValueString() == "" {
			return "", false, newConfigurationError(
				attribute,
				ConfigurationErrorEmpty,
				"value must not be empty",
			)
		}
		return value.ValueString(), true, nil
	}

	environmentValue, ok := lookup(environmentName)
	if !ok {
		return "", false, nil
	}
	if environmentValue == "" {
		return "", false, newConfigurationError(
			attribute,
			ConfigurationErrorEmpty,
			"environment value must not be empty",
		)
	}
	return environmentValue, true, nil
}

func resolveBool(value types.Bool, lookup environmentLookup) (bool, error) {
	if value.IsUnknown() {
		return false, newConfigurationError(
			"insecure",
			ConfigurationErrorUnknown,
			"value must be known during provider configuration",
		)
	}
	if !value.IsNull() {
		return value.ValueBool(), nil
	}

	raw, ok := lookup("NUTANIX_INSECURE")
	if !ok {
		return false, nil
	}
	switch {
	case strings.EqualFold(raw, "true"):
		return true, nil
	case strings.EqualFold(raw, "false"):
		return false, nil
	default:
		return false, newConfigurationError(
			"insecure",
			ConfigurationErrorBoolean,
			"NUTANIX_INSECURE must be true or false",
		)
	}
}

func resolveTimeout(value types.Int64, lookup environmentLookup) (int64, error) {
	if value.IsUnknown() {
		return 0, newConfigurationError(
			"request_timeout_seconds",
			ConfigurationErrorUnknown,
			"value must be known during provider configuration",
		)
	}

	seconds := defaultRequestTimeoutSeconds
	if !value.IsNull() {
		seconds = value.ValueInt64()
	} else if raw, ok := lookup("NUTANIX_REQUEST_TIMEOUT_SECONDS"); ok {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return 0, newConfigurationError(
				"request_timeout_seconds",
				ConfigurationErrorTimeout,
				"NUTANIX_REQUEST_TIMEOUT_SECONDS must be a base-10 integer from 1 through 600",
			)
		}
		seconds = parsed
	}
	if seconds < minimumRequestTimeoutSeconds || seconds > maximumRequestTimeoutSeconds {
		return 0, newConfigurationError(
			"request_timeout_seconds",
			ConfigurationErrorTimeout,
			"request timeout must be from 1 through 600 seconds",
		)
	}
	return seconds, nil
}

func parseEndpoint(value string) (*url.URL, error) {
	endpoint, err := url.Parse(value)
	if err != nil {
		return nil, newConfigurationError(
			"endpoint",
			ConfigurationErrorEndpoint,
			"endpoint must be an HTTPS origin",
		)
	}
	invalidPort := strings.HasSuffix(endpoint.Host, ":")
	if port := endpoint.Port(); port != "" {
		parsedPort, parseErr := strconv.ParseUint(port, 10, 16)
		invalidPort = parseErr != nil || parsedPort == 0
	}
	if strings.Contains(value, "#") ||
		invalidPort ||
		endpoint.Scheme != "https" ||
		endpoint.Host == "" ||
		endpoint.Hostname() == "" ||
		endpoint.User != nil ||
		endpoint.Opaque != "" ||
		endpoint.RawQuery != "" ||
		endpoint.ForceQuery ||
		endpoint.Fragment != "" ||
		endpoint.RawFragment != "" ||
		endpoint.RawPath != "" ||
		(endpoint.Path != "" && endpoint.Path != "/") {
		return nil, newConfigurationError(
			"endpoint",
			ConfigurationErrorEndpoint,
			"endpoint must be an HTTPS origin without credentials, query, fragment, or path",
		)
	}
	endpoint.Path = ""
	return endpoint, nil
}

func isVisibleASCII(value string) bool {
	if len(value) == 0 || len(value) > maximumAPIKeyBytes {
		return false
	}
	for index := range len(value) {
		if value[index] < 0x21 || value[index] > 0x7e {
			return false
		}
	}
	return true
}
