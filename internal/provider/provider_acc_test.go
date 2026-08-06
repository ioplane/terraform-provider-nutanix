package provider

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccProviderProtocol6(t *testing.T) {
	isolateProviderEnvironment(t)
	t.Setenv("NUTANIX_ENDPOINT", "https://environment.example.test:9440")
	t.Setenv("NUTANIX_API_KEY", "environment-test-api-key")

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"nutanix": providerserver.NewProtocol6WithError(New("test")()),
		},
		Steps: []resource.TestStep{{
			Config: `
terraform {
  required_providers {
    nutanix = {
      source = "ioplane/nutanix"
    }
  }
}

provider "nutanix" {
  endpoint = "https://terraform.example.test:9440"
  api_key  = "terraform-test-api-key"
}
`,
		}},
	})

	server, err := providerserver.NewProtocol6WithError(New("test")())()
	if err != nil {
		t.Fatalf("create Protocol 6 server: %v", err)
	}
	ctx := context.Background()
	schemaResponse, err := server.GetProviderSchema(ctx, &tfprotov6.GetProviderSchemaRequest{})
	if err != nil {
		t.Fatalf("GetProviderSchema() error = %v", err)
	}
	if diagnosticsHaveErrors(schemaResponse.Diagnostics) {
		t.Fatalf("GetProviderSchema() diagnostics = %v", schemaResponse.Diagnostics)
	}
	if len(schemaResponse.ResourceSchemas) != 4 ||
		len(schemaResponse.DataSourceSchemas) != 8 ||
		len(schemaResponse.Functions) != 0 ||
		len(schemaResponse.EphemeralResourceSchemas) != 0 ||
		len(schemaResponse.ListResourceSchemas) != 0 ||
		len(schemaResponse.ActionSchemas) != 0 ||
		len(schemaResponse.StateStoreSchemas) != 0 {
		t.Fatal("Protocol 6 schema registered an unexpected Terraform type")
	}

	sensitive := map[string]bool{}
	for _, attribute := range schemaResponse.Provider.Block.Attributes {
		sensitive[attribute.Name] = attribute.Sensitive
	}
	for name, want := range map[string]bool{
		"endpoint": false, "username": false, "password": true,
		"api_key": true, "insecure": false, "ca_certificate": true,
		"request_timeout_seconds": false,
	} {
		if got := sensitive[name]; got != want {
			t.Errorf("Protocol 6 attribute %q sensitive = %t, want %t", name, got, want)
		}
	}

	t.Run("environment fallback", func(t *testing.T) {
		config := protocolProviderConfig(t, schemaResponse.Provider, nil)
		response, err := server.ConfigureProvider(ctx, &tfprotov6.ConfigureProviderRequest{Config: &config})
		if err != nil {
			t.Fatalf("ConfigureProvider() error = %v", err)
		}
		if diagnosticsHaveErrors(response.Diagnostics) {
			t.Fatalf("ConfigureProvider() diagnostics = %v", response.Diagnostics)
		}
	})

	t.Run("explicit configuration", func(t *testing.T) {
		config := protocolProviderConfig(t, schemaResponse.Provider, map[string]string{
			"endpoint": "https://terraform.example.test:9440",
			"api_key":  "terraform-test-api-key",
		})
		response, err := server.ConfigureProvider(ctx, &tfprotov6.ConfigureProviderRequest{Config: &config})
		if err != nil {
			t.Fatalf("ConfigureProvider() error = %v", err)
		}
		if diagnosticsHaveErrors(response.Diagnostics) {
			t.Fatalf("ConfigureProvider() diagnostics = %v", response.Diagnostics)
		}
	})

	t.Run("secret-safe diagnostics", func(t *testing.T) {
		const canary = "protocol-canary invalid-api-key"
		assertProtocolConfigureRedacts(
			t,
			server,
			ctx,
			schemaResponse.Provider,
			map[string]tftypes.Value{
				"endpoint": tftypes.NewValue(tftypes.String, "https://terraform.example.test:9440"),
				"api_key":  tftypes.NewValue(tftypes.String, canary),
			},
			"api_key",
			canary,
		)
	})

	t.Run("password conflict diagnostic redaction", func(t *testing.T) {
		const canary = "protocol-password-canary-54f099c0"
		assertProtocolConfigureRedacts(
			t,
			server,
			ctx,
			schemaResponse.Provider,
			map[string]tftypes.Value{
				"endpoint": tftypes.NewValue(tftypes.String, "https://pc.example.test"),
				"username": tftypes.NewValue(tftypes.String, "admin"),
				"password": tftypes.NewValue(tftypes.String, canary),
				"api_key":  tftypes.NewValue(tftypes.String, "safe-api-key"),
			},
			"api_key",
			canary,
		)
	})

	t.Run("CA conflict diagnostic redaction", func(t *testing.T) {
		const canary = "protocol-ca-canary-29c26d88"
		assertProtocolConfigureRedacts(
			t,
			server,
			ctx,
			schemaResponse.Provider,
			map[string]tftypes.Value{
				"endpoint":       tftypes.NewValue(tftypes.String, "https://pc.example.test"),
				"api_key":        tftypes.NewValue(tftypes.String, "safe-api-key"),
				"insecure":       tftypes.NewValue(tftypes.Bool, true),
				"ca_certificate": tftypes.NewValue(tftypes.String, canary),
			},
			"ca_certificate",
			canary,
		)
	})
}

func protocolProviderConfig(
	t *testing.T,
	schema *tfprotov6.Schema,
	configuredStrings map[string]string,
) tfprotov6.DynamicValue {
	t.Helper()
	values := make(map[string]tftypes.Value, len(configuredStrings))
	for name, value := range configuredStrings {
		values[name] = tftypes.NewValue(tftypes.String, value)
	}
	return protocolProviderConfigValues(t, schema, values)
}

func protocolProviderConfigValues(
	t *testing.T,
	schema *tfprotov6.Schema,
	overrides map[string]tftypes.Value,
) tfprotov6.DynamicValue {
	t.Helper()
	values := map[string]tftypes.Value{
		"endpoint":                tftypes.NewValue(tftypes.String, nil),
		"username":                tftypes.NewValue(tftypes.String, nil),
		"password":                tftypes.NewValue(tftypes.String, nil),
		"api_key":                 tftypes.NewValue(tftypes.String, nil),
		"insecure":                tftypes.NewValue(tftypes.Bool, nil),
		"ca_certificate":          tftypes.NewValue(tftypes.String, nil),
		"request_timeout_seconds": tftypes.NewValue(tftypes.Number, nil),
	}
	for name, value := range overrides {
		values[name] = value
	}

	valueType := schema.ValueType()
	value := tftypes.NewValue(valueType, values)
	dynamicValue, err := tfprotov6.NewDynamicValue(valueType, value)
	if err != nil {
		t.Fatalf("NewDynamicValue() error = %v", err)
	}
	return dynamicValue
}

func diagnosticsHaveErrors(diagnostics []*tfprotov6.Diagnostic) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Severity == tfprotov6.DiagnosticSeverityError {
			return true
		}
	}
	return false
}

func assertProtocolConfigureRedacts(
	t *testing.T,
	server tfprotov6.ProviderServer,
	ctx context.Context,
	schema *tfprotov6.Schema,
	values map[string]tftypes.Value,
	wantErrorPath string,
	canaries ...string,
) {
	t.Helper()
	config := protocolProviderConfigValues(t, schema, values)
	response, err := server.ConfigureProvider(ctx, &tfprotov6.ConfigureProviderRequest{Config: &config})
	if err != nil {
		t.Fatalf("ConfigureProvider() error = %v", err)
	}
	wantPath := tftypes.NewAttributePath().WithAttributeName(wantErrorPath)
	var errorCount int
	for _, diagnostic := range response.Diagnostics {
		for _, canary := range canaries {
			if strings.Contains(diagnostic.Summary, canary) || strings.Contains(diagnostic.Detail, canary) {
				t.Fatalf("ConfigureProvider() diagnostic exposes a secret canary for %s", wantErrorPath)
			}
		}
		if diagnostic.Severity != tfprotov6.DiagnosticSeverityError {
			continue
		}
		errorCount++
		if diagnostic.Attribute == nil || !diagnostic.Attribute.Equal(wantPath) {
			t.Fatalf("ConfigureProvider() diagnostic path = %v, want %s", diagnostic.Attribute, wantErrorPath)
		}
	}
	if errorCount == 0 {
		t.Fatal("ConfigureProvider() returned no error diagnostic")
	}
}

func isolateProviderEnvironment(t *testing.T) {
	t.Helper()
	for _, name := range []string{
		"NUTANIX_ENDPOINT",
		"NUTANIX_USERNAME",
		"NUTANIX_PASSWORD",
		"NUTANIX_API_KEY",
		"NUTANIX_INSECURE",
		"NUTANIX_CA_CERTIFICATE",
		"NUTANIX_REQUEST_TIMEOUT_SECONDS",
	} {
		value, present := os.LookupEnv(name)
		if err := os.Unsetenv(name); err != nil {
			t.Fatalf("unset %s: %v", name, err)
		}
		t.Cleanup(func() {
			if present {
				if err := os.Setenv(name, value); err != nil {
					t.Errorf("restore %s: %v", name, err)
				}
				return
			}
			if err := os.Unsetenv(name); err != nil {
				t.Errorf("restore unset %s: %v", name, err)
			}
		})
	}
}
