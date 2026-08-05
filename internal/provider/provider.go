// Package provider implements the Terraform provider boundary.
package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	frameworkprovider "github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

const typeName = "nutanix"

type nutanixProvider struct {
	version string
}

// New returns a factory for an intentionally empty Nutanix provider.
func New(version string) func() frameworkprovider.Provider {
	return func() frameworkprovider.Provider {
		return &nutanixProvider{version: version}
	}
}

// Metadata sets the stable provider type and injected build version.
func (p *nutanixProvider) Metadata(
	_ context.Context,
	_ frameworkprovider.MetadataRequest,
	response *frameworkprovider.MetadataResponse,
) {
	response.TypeName = typeName
	response.Version = p.version
}

// Schema returns the intentionally empty M0 provider configuration schema.
func (p *nutanixProvider) Schema(
	_ context.Context,
	_ frameworkprovider.SchemaRequest,
	response *frameworkprovider.SchemaResponse,
) {
	response.Schema = schema.Schema{
		Description: "Nutanix infrastructure provider foundation.",
	}
}

// Configure accepts the intentionally empty M0 provider configuration.
func (p *nutanixProvider) Configure(
	_ context.Context,
	_ frameworkprovider.ConfigureRequest,
	_ *frameworkprovider.ConfigureResponse,
) {
}

// Resources returns the empty M0 managed-resource registry.
func (p *nutanixProvider) Resources(context.Context) []func() resource.Resource {
	return nil
}

// DataSources returns the empty M0 data-source registry.
func (p *nutanixProvider) DataSources(context.Context) []func() datasource.DataSource {
	return nil
}
