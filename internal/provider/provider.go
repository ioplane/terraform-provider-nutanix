// Package provider implements the Terraform provider boundary.
package provider

import (
	"context"
	"errors"
	"os"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	frameworkprovider "github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/ioplane/terraform-provider-nutanix/internal/auth"
	"github.com/ioplane/terraform-provider-nutanix/internal/transport"
)

const typeName = "nutanix"

type nutanixProvider struct {
	version string
}

type configuredProviderData struct {
	client *transport.Client
}

// New returns a factory for the Nutanix provider.
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

// Schema returns the M1 provider configuration schema.
func (p *nutanixProvider) Schema(
	_ context.Context,
	_ frameworkprovider.SchemaRequest,
	response *frameworkprovider.SchemaResponse,
) {
	response.Schema = schema.Schema{
		Description: "Nutanix infrastructure provider.",
		Attributes: map[string]schema.Attribute{
			"endpoint": schema.StringAttribute{
				Optional:    true,
				Description: "HTTPS origin of one Nutanix control plane. May be set with NUTANIX_ENDPOINT.",
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"username": schema.StringAttribute{
				Optional:    true,
				Description: "Basic authentication username. May be set with NUTANIX_USERNAME.",
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"password": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "Basic authentication password. May be set with NUTANIX_PASSWORD.",
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"api_key": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "Nutanix API key. May be set with NUTANIX_API_KEY.",
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 4096),
				},
			},
			"insecure": schema.BoolAttribute{
				Optional:    true,
				Description: "Disable TLS certificate verification. May be set with NUTANIX_INSECURE; defaults to false.",
			},
			"ca_certificate": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "PEM CA bundle appended to system roots. May be set with NUTANIX_CA_CERTIFICATE.",
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"request_timeout_seconds": schema.Int64Attribute{
				Optional:    true,
				Description: "Per-attempt HTTP timeout in seconds. May be set with NUTANIX_REQUEST_TIMEOUT_SECONDS; defaults to 60.",
				Validators: []validator.Int64{
					int64validator.Between(1, 600),
				},
			},
		},
	}
}

// Configure resolves provider values without contacting a Nutanix endpoint.
func (p *nutanixProvider) Configure(
	ctx context.Context,
	request frameworkprovider.ConfigureRequest,
	response *frameworkprovider.ConfigureResponse,
) {
	var model providerConfig
	response.Diagnostics.Append(request.Config.Get(ctx, &model)...)
	if response.Diagnostics.HasError() {
		return
	}

	configured, err := resolveConfig(model, os.LookupEnv)
	if err != nil {
		addConfigurationDiagnostic(response, err)
		return
	}

	configuredData, err := composeProviderData(configured, p.version, request.TerraformVersion)
	if err != nil {
		addConfigurationDiagnostic(response, err)
		return
	}

	response.ResourceData = configuredData
	response.DataSourceData = configuredData
}

func composeProviderData(
	configured resolvedConfig,
	providerVersion string,
	terraformVersion string,
) (configuredProviderData, error) {
	origin, err := transport.ParseOrigin(configured.endpoint.String())
	if err != nil {
		return configuredProviderData{}, newConfigurationError(
			"endpoint",
			ConfigurationErrorEndpoint,
			"endpoint must be an HTTPS origin",
		)
	}
	tlsConfig, err := transport.NewTLSConfig(origin, configured.insecure, configured.caCertificate)
	if err != nil {
		return configuredProviderData{}, newConfigurationErrorWithCause(
			"ca_certificate",
			ConfigurationErrorTLS,
			"TLS trust configuration could not be constructed",
			err,
		)
	}

	var client *transport.Client
	if configured.apiKey != "" {
		client, err = transport.NewClient(
			origin,
			auth.NewAPIKey(configured.apiKey),
			tlsConfig,
			configured.requestTimeout,
			providerVersion,
			terraformVersion,
		)
	} else {
		client, err = transport.NewClient(
			origin,
			auth.NewBasic(configured.username, configured.password),
			tlsConfig,
			configured.requestTimeout,
			providerVersion,
			terraformVersion,
		)
	}
	if err != nil {
		return configuredProviderData{}, mapClientConfigurationError(err)
	}
	return configuredProviderData{client: client}, nil
}

func mapClientConfigurationError(err error) *ConfigurationError {
	switch {
	case errors.Is(err, transport.ErrInvalidClientOrigin):
		return newConfigurationErrorWithCause(
			"endpoint",
			ConfigurationErrorEndpoint,
			"endpoint could not be bound to an HTTP client",
			err,
		)
	case errors.Is(err, transport.ErrMissingAuthorizer),
		errors.Is(err, transport.ErrInvalidAuthorizationState):
		return newConfigurationErrorWithCause(
			"username",
			ConfigurationErrorAuthentication,
			"authentication could not be bound to an HTTP client",
			err,
		)
	case errors.Is(err, transport.ErrInvalidClientTimeout):
		return newConfigurationErrorWithCause(
			"request_timeout_seconds",
			ConfigurationErrorTimeout,
			"request timeout could not be bound to an HTTP client",
			err,
		)
	case errors.Is(err, transport.ErrMissingTLSConfig),
		errors.Is(err, transport.ErrInvalidTLSConfig),
		errors.Is(err, transport.ErrMissingRoundTripper):
		return newConfigurationErrorWithCause(
			"ca_certificate",
			ConfigurationErrorTLS,
			"HTTP client security dependencies could not be constructed",
			err,
		)
	default:
		return newConfigurationError(
			"ca_certificate",
			ConfigurationErrorTLS,
			"HTTP client security dependencies could not be constructed",
		)
	}
}

func addConfigurationDiagnostic(response *frameworkprovider.ConfigureResponse, err error) {
	var configurationError *ConfigurationError
	if errors.As(err, &configurationError) {
		response.Diagnostics.AddAttributeError(
			path.Root(configurationError.Attribute),
			"Invalid Nutanix Provider Configuration",
			configurationError.Error(),
		)
		return
	}
	response.Diagnostics.AddError(
		"Invalid Nutanix Provider Configuration",
		"Provider configuration could not be resolved.",
	)
}

// Resources returns the empty M1 managed-resource registry.
func (p *nutanixProvider) Resources(context.Context) []func() resource.Resource {
	return nil
}

// DataSources returns the empty M1 data-source registry.
func (p *nutanixProvider) DataSources(context.Context) []func() datasource.DataSource {
	return nil
}
