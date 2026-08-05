package provider

import (
	"context"
	"testing"

	frameworkprovider "github.com/hashicorp/terraform-plugin-framework/provider"
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

func TestProviderSchemaIsIntentionallyEmpty(t *testing.T) {
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
	if got := len(response.Schema.Attributes); got != 0 {
		t.Fatalf("provider attribute count = %d, want 0", got)
	}
}

func TestProviderEmptyConfiguration(t *testing.T) {
	t.Parallel()

	response := &frameworkprovider.ConfigureResponse{}
	New("test")().Configure(
		context.Background(),
		frameworkprovider.ConfigureRequest{},
		response,
	)

	if response.Diagnostics.HasError() {
		t.Fatalf("provider configuration returned errors: %v", response.Diagnostics.Errors())
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
