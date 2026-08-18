// Package testkit contains reusable contract assertions for service tests.
package testkit

import (
	"context"
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	datasourceschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
)

// AttributeFlags describes the Terraform shape expected at a service boundary.
type AttributeFlags struct {
	TypeName string
	Optional bool
	Required bool
	Computed bool
	Nested   map[string]AttributeFlags
}

// Optional returns an optional attribute expectation with a concrete Framework type.
func Optional(typeName string) AttributeFlags {
	return AttributeFlags{TypeName: typeName, Optional: true}
}

// Required returns a required attribute expectation with a concrete Framework type.
func Required(typeName string) AttributeFlags {
	return AttributeFlags{TypeName: typeName, Required: true}
}

// Computed returns a computed attribute expectation with a concrete Framework type.
func Computed(typeName string) AttributeFlags {
	return AttributeFlags{TypeName: typeName, Computed: true}
}

// OptionalComputed returns an optional and computed attribute expectation.
func OptionalComputed(typeName string) AttributeFlags {
	return AttributeFlags{TypeName: typeName, Optional: true, Computed: true}
}

// NestedComputed returns a computed nested attribute with recursive expectations.
func NestedComputed(typeName string, nested map[string]AttributeFlags) AttributeFlags {
	return AttributeFlags{TypeName: typeName, Computed: true, Nested: nested}
}

// AssertDataSourceContract checks metadata, concrete schema types, and attribute modes.
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
		assertAttribute(t, "data source."+name, attribute, flags)
	}
}

// AssertResourceContract checks metadata, concrete schema types, and attribute modes.
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
		assertAttribute(t, "resource."+name, attribute, flags)
	}
}

func assertAttribute(t *testing.T, path string, attribute any, flags AttributeFlags) {
	t.Helper()
	frameworkAttribute, ok := attribute.(interface {
		IsOptional() bool
		IsRequired() bool
		IsComputed() bool
	})
	if !ok {
		t.Fatalf("%s has unsupported Framework attribute type %T", path, attribute)
	}
	actualType := reflect.TypeOf(attribute).Name()
	if flags.TypeName != "" && actualType != flags.TypeName {
		t.Errorf("%s type = %q, want %q", path, actualType, flags.TypeName)
	}
	if frameworkAttribute.IsOptional() != flags.Optional || frameworkAttribute.IsRequired() != flags.Required || frameworkAttribute.IsComputed() != flags.Computed {
		t.Errorf("%s flags = optional:%t required:%t computed:%t, want optional:%t required:%t computed:%t", path, frameworkAttribute.IsOptional(), frameworkAttribute.IsRequired(), frameworkAttribute.IsComputed(), flags.Optional, flags.Required, flags.Computed)
	}
	if flags.Nested == nil {
		return
	}
	children := nestedAttributes(attribute)
	if len(children) != len(flags.Nested) {
		t.Errorf("%s nested attribute count = %d, want %d", path, len(children), len(flags.Nested))
	}
	for name, childFlags := range flags.Nested {
		child, ok := children[name]
		if !ok {
			t.Errorf("%s missing nested attribute %q", path, name)
			continue
		}
		assertAttribute(t, path+"."+name, child, childFlags)
	}
}

func nestedAttributes(attribute any) map[string]any {
	result := map[string]any{}
	switch value := attribute.(type) {
	case datasourceschema.ListNestedAttribute:
		for name, child := range value.NestedObject.Attributes {
			result[name] = child
		}
	case datasourceschema.SingleNestedAttribute:
		for name, child := range value.Attributes {
			result[name] = child
		}
	case resourceschema.ListNestedAttribute:
		for name, child := range value.NestedObject.Attributes {
			result[name] = child
		}
	case resourceschema.SingleNestedAttribute:
		for name, child := range value.Attributes {
			result[name] = child
		}
	}
	return result
}
