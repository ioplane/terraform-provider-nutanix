// Package testkit contains reusable contract assertions for service tests.
package testkit

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

// AttributeFlags describes the Terraform shape expected at a service boundary.
type AttributeFlags struct {
	Optional bool
	Required bool
	Computed bool
}

// AssertDataSourceContract checks metadata, schema names, and attribute modes.
func AssertDataSourceContract(t *testing.T, dataSource datasource.DataSource, typeName string, want map[string]AttributeFlags) {
	t.Helper()
	ctx := context.Background()
	metadata := &datasource.MetadataResponse{}
	dataSource.Metadata(ctx, datasource.MetadataRequest{ProviderTypeName: "nutanix"}, metadata)
	if metadata.TypeName != typeName {
		t.Fatalf("data source type name = %q, want %q", metadata.TypeName, typeName)
	}

	schemaResponse := &datasource.SchemaResponse{}
	dataSource.Schema(ctx, datasource.SchemaRequest{}, schemaResponse)
	if schemaResponse.Diagnostics.HasError() {
		t.Fatalf("data source schema diagnostics = %v", schemaResponse.Diagnostics)
	}
	if len(schemaResponse.Schema.Attributes) != len(want) {
		t.Fatalf("data source attribute count = %d, want %d", len(schemaResponse.Schema.Attributes), len(want))
	}
	for name, flags := range want {
		attribute, ok := schemaResponse.Schema.Attributes[name]
		if !ok {
			t.Errorf("data source schema missing %q", name)
			continue
		}
		if attribute.IsOptional() != flags.Optional || attribute.IsRequired() != flags.Required || attribute.IsComputed() != flags.Computed {
			t.Errorf("data source attribute %q flags = optional:%t required:%t computed:%t, want optional:%t required:%t computed:%t", name, attribute.IsOptional(), attribute.IsRequired(), attribute.IsComputed(), flags.Optional, flags.Required, flags.Computed)
		}
	}
}

// AssertResourceContract checks metadata, schema names, and attribute modes.
func AssertResourceContract(t *testing.T, resourceImpl resource.Resource, typeName string, want map[string]AttributeFlags) {
	t.Helper()
	ctx := context.Background()
	metadata := &resource.MetadataResponse{}
	resourceImpl.Metadata(ctx, resource.MetadataRequest{ProviderTypeName: "nutanix"}, metadata)
	if metadata.TypeName != typeName {
		t.Fatalf("resource type name = %q, want %q", metadata.TypeName, typeName)
	}

	schemaResponse := &resource.SchemaResponse{}
	resourceImpl.Schema(ctx, resource.SchemaRequest{}, schemaResponse)
	if schemaResponse.Diagnostics.HasError() {
		t.Fatalf("resource schema diagnostics = %v", schemaResponse.Diagnostics)
	}
	if len(schemaResponse.Schema.Attributes) != len(want) {
		t.Fatalf("resource attribute count = %d, want %d", len(schemaResponse.Schema.Attributes), len(want))
	}
	for name, flags := range want {
		attribute, ok := schemaResponse.Schema.Attributes[name]
		if !ok {
			t.Errorf("resource schema missing %q", name)
			continue
		}
		if attribute.IsOptional() != flags.Optional || attribute.IsRequired() != flags.Required || attribute.IsComputed() != flags.Computed {
			t.Errorf("resource attribute %q flags = optional:%t required:%t computed:%t, want optional:%t required:%t computed:%t", name, attribute.IsOptional(), attribute.IsRequired(), attribute.IsComputed(), flags.Optional, flags.Required, flags.Computed)
		}
	}
}
