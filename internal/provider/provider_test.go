package provider

import (
	"context"
	"reflect"
	"strings"
	"testing"

	frameworkprovider "github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

var _ frameworkprovider.Provider = New("compile-test")()

func TestProviderMetadata(t *testing.T) {
	t.Parallel()

	response := &frameworkprovider.MetadataResponse{}
	New("1.2.3")().Metadata(
		context.Background(),
		frameworkprovider.MetadataRequest{},
		response,
	)

	if response.TypeName != "nutanix" {
		t.Fatalf("provider type name = %q, want %q", response.TypeName, "nutanix")
	}
	if response.Version != "1.2.3" {
		t.Fatalf("provider version = %q, want %q", response.Version, "1.2.3")
	}
}

func TestProviderSchema(t *testing.T) {
	t.Parallel()

	response := &frameworkprovider.SchemaResponse{}
	New("test")().Schema(
		context.Background(),
		frameworkprovider.SchemaRequest{},
		response,
	)

	if response.Diagnostics.HasError() {
		t.Fatalf("provider schema returned errors: %v", response.Diagnostics.Errors())
	}
	if response.Schema.Description == "" {
		t.Fatal("provider schema description is empty")
	}

	want := map[string]struct {
		typeName    string
		sensitive   bool
		description string
	}{
		"endpoint": {
			typeName:    "StringAttribute",
			description: "HTTPS origin of one Nutanix control plane. May be set with NUTANIX_ENDPOINT.",
		},
		"username": {
			typeName:    "StringAttribute",
			description: "Basic authentication username. May be set with NUTANIX_USERNAME.",
		},
		"password": {
			typeName:    "StringAttribute",
			sensitive:   true,
			description: "Basic authentication password. May be set with NUTANIX_PASSWORD.",
		},
		"api_key": {
			typeName:    "StringAttribute",
			sensitive:   true,
			description: "Nutanix API key. May be set with NUTANIX_API_KEY.",
		},
		"insecure": {
			typeName:    "BoolAttribute",
			description: "Disable TLS certificate verification. May be set with NUTANIX_INSECURE; defaults to false.",
		},
		"ca_certificate": {
			typeName:    "StringAttribute",
			sensitive:   true,
			description: "PEM CA bundle appended to system roots. May be set with NUTANIX_CA_CERTIFICATE.",
		},
		"request_timeout_seconds": {
			typeName:    "Int64Attribute",
			description: "Per-attempt HTTP timeout in seconds. May be set with NUTANIX_REQUEST_TIMEOUT_SECONDS; defaults to 60.",
		},
	}

	if got := len(response.Schema.Attributes); got != len(want) {
		t.Fatalf("provider attribute count = %d, want %d", got, len(want))
	}

	for name, expected := range want {
		attribute, ok := response.Schema.Attributes[name]
		if !ok {
			t.Errorf("provider schema is missing attribute %q", name)
			continue
		}
		if !attribute.IsOptional() {
			t.Errorf("attribute %q is not optional", name)
		}
		if attribute.IsRequired() || attribute.IsComputed() {
			t.Errorf("attribute %q required/computed = %t/%t, want false/false", name, attribute.IsRequired(), attribute.IsComputed())
		}
		if got := attribute.IsSensitive(); got != expected.sensitive {
			t.Errorf("attribute %q sensitive = %t, want %t", name, got, expected.sensitive)
		}
		if got := attribute.GetDescription(); got != expected.description {
			t.Errorf("attribute %q description = %q, want %q", name, got, expected.description)
		}
		if got := reflect.TypeOf(attribute).Name(); got != expected.typeName {
			t.Errorf("attribute %q type = %q, want %q", name, got, expected.typeName)
		}
	}

}

func TestProviderSchemaValidators(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	server, err := providerserver.NewProtocol6WithError(New("test")())()
	if err != nil {
		t.Fatalf("create Protocol 6 server: %v", err)
	}
	schemaResponse, err := server.GetProviderSchema(ctx, &tfprotov6.GetProviderSchemaRequest{})
	if err != nil {
		t.Fatalf("GetProviderSchema() error = %v", err)
	}
	if diagnosticsHaveErrors(schemaResponse.Diagnostics) {
		t.Fatalf("GetProviderSchema() diagnostics = %v", schemaResponse.Diagnostics)
	}

	nullValues := map[string]tftypes.Value{}
	unknownValues := map[string]tftypes.Value{
		"endpoint":                tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
		"username":                tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
		"password":                tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
		"api_key":                 tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
		"insecure":                tftypes.NewValue(tftypes.Bool, tftypes.UnknownValue),
		"ca_certificate":          tftypes.NewValue(tftypes.String, tftypes.UnknownValue),
		"request_timeout_seconds": tftypes.NewValue(tftypes.Number, tftypes.UnknownValue),
	}

	tests := []struct {
		name          string
		values        map[string]tftypes.Value
		wantErrorPath string
	}{
		{name: "null values allow environment fallback", values: nullValues},
		{name: "unknown values defer to Configure", values: unknownValues},
		{name: "API key one byte", values: map[string]tftypes.Value{"api_key": tftypes.NewValue(tftypes.String, "a")}},
		{name: "API key 4096 bytes", values: map[string]tftypes.Value{"api_key": tftypes.NewValue(tftypes.String, strings.Repeat("a", 4096))}},
		{name: "API key zero bytes", values: map[string]tftypes.Value{"api_key": tftypes.NewValue(tftypes.String, "")}, wantErrorPath: "api_key"},
		{name: "API key 4097 bytes", values: map[string]tftypes.Value{"api_key": tftypes.NewValue(tftypes.String, strings.Repeat("a", 4097))}, wantErrorPath: "api_key"},
		{name: "timeout lower boundary", values: map[string]tftypes.Value{"request_timeout_seconds": tftypes.NewValue(tftypes.Number, 1)}, wantErrorPath: ""},
		{name: "timeout upper boundary", values: map[string]tftypes.Value{"request_timeout_seconds": tftypes.NewValue(tftypes.Number, 600)}, wantErrorPath: ""},
		{name: "timeout below boundary", values: map[string]tftypes.Value{"request_timeout_seconds": tftypes.NewValue(tftypes.Number, 0)}, wantErrorPath: "request_timeout_seconds"},
		{name: "timeout above boundary", values: map[string]tftypes.Value{"request_timeout_seconds": tftypes.NewValue(tftypes.Number, 601)}, wantErrorPath: "request_timeout_seconds"},
	}
	for _, name := range []string{"endpoint", "username", "password", "ca_certificate"} {
		tests = append(tests, struct {
			name          string
			values        map[string]tftypes.Value
			wantErrorPath string
		}{
			name:          name + " rejects empty string",
			values:        map[string]tftypes.Value{name: tftypes.NewValue(tftypes.String, "")},
			wantErrorPath: name,
		})
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			config := protocolProviderConfigValues(t, schemaResponse.Provider, test.values)
			response, err := server.ValidateProviderConfig(ctx, &tfprotov6.ValidateProviderConfigRequest{Config: &config})
			if err != nil {
				t.Fatalf("ValidateProviderConfig() error = %v", err)
			}
			if test.wantErrorPath == "" {
				if diagnosticsHaveErrors(response.Diagnostics) {
					t.Fatalf("ValidateProviderConfig() diagnostics = %v", response.Diagnostics)
				}
				return
			}

			wantPath := tftypes.NewAttributePath().WithAttributeName(test.wantErrorPath)
			var errorCount int
			for _, diagnostic := range response.Diagnostics {
				if diagnostic.Severity != tfprotov6.DiagnosticSeverityError {
					continue
				}
				errorCount++
				if diagnostic.Attribute == nil || !diagnostic.Attribute.Equal(wantPath) {
					t.Errorf("validator diagnostic path = %v, want %s", diagnostic.Attribute, test.wantErrorPath)
				}
			}
			if errorCount != 1 {
				t.Fatalf("validator error diagnostic count = %d, want 1", errorCount)
			}
		})
	}
}

func TestProviderRegistersNoTypes(t *testing.T) {
	t.Parallel()

	configured := New("test")()
	if got := len(configured.Resources(context.Background())); got != 0 {
		t.Fatalf("resource count = %d, want 0", got)
	}
	if got := len(configured.DataSources(context.Background())); got != 0 {
		t.Fatalf("data source count = %d, want 0", got)
	}
}
