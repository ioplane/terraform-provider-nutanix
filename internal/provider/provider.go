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
	"github.com/ioplane/terraform-provider-nutanix/internal/capability"
	"github.com/ioplane/terraform-provider-nutanix/internal/nutanix/clustermgmt"
	"github.com/ioplane/terraform-provider-nutanix/internal/nutanix/iam"
	"github.com/ioplane/terraform-provider-nutanix/internal/nutanix/licensing"
	"github.com/ioplane/terraform-provider-nutanix/internal/nutanix/networking"
	"github.com/ioplane/terraform-provider-nutanix/internal/nutanix/prism"
	"github.com/ioplane/terraform-provider-nutanix/internal/nutanix/vmm"
	"github.com/ioplane/terraform-provider-nutanix/internal/service/category"
	"github.com/ioplane/terraform-provider-nutanix/internal/service/cluster"
	"github.com/ioplane/terraform-provider-nutanix/internal/service/image"
	license "github.com/ioplane/terraform-provider-nutanix/internal/service/license"
	licensekey "github.com/ioplane/terraform-provider-nutanix/internal/service/licensekey"
	"github.com/ioplane/terraform-provider-nutanix/internal/service/operation"
	"github.com/ioplane/terraform-provider-nutanix/internal/service/placementpolicy"
	"github.com/ioplane/terraform-provider-nutanix/internal/service/role"
	"github.com/ioplane/terraform-provider-nutanix/internal/service/storagecontainer"
	"github.com/ioplane/terraform-provider-nutanix/internal/service/subnet"
	ntnxtask "github.com/ioplane/terraform-provider-nutanix/internal/task"
	"github.com/ioplane/terraform-provider-nutanix/internal/transport"
)

const typeName = "nutanix"

type nutanixProvider struct {
	version string
}

type configuredProviderData struct {
	client         *transport.Client
	capabilities   *capability.Registry
	clusterClient  *clustermgmt.Client
	iamClient      *iam.Client
	categoryClient *prism.Client
	imageClient    *vmm.Client
	subnetClient   *networking.Client
	licenseClient  *licensing.Client
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

// Schema returns the provider configuration schema.
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
	categoryClient, err := prism.NewClient(client)
	if err != nil {
		return configuredProviderData{}, productClientConfigurationError()
	}
	taskReader, err := prism.NewReader(client)
	if err != nil {
		return configuredProviderData{}, productClientConfigurationError()
	}
	taskWaiter, err := ntnxtask.NewWaiter(taskReader)
	if err != nil {
		return configuredProviderData{}, productClientConfigurationError()
	}
	clusterClient, err := clustermgmt.NewClientWithTaskWaiter(client, taskWaiter)
	if err != nil {
		return configuredProviderData{}, productClientConfigurationError()
	}
	imageClient, err := vmm.NewClientWithTaskWaiter(client, taskWaiter)
	if err != nil {
		return configuredProviderData{}, productClientConfigurationError()
	}
	subnetClient, err := networking.NewClientWithTaskWaiter(client, taskWaiter)
	if err != nil {
		return configuredProviderData{}, productClientConfigurationError()
	}
	iamClient, err := iam.NewClient(client)
	if err != nil {
		return configuredProviderData{}, productClientConfigurationError()
	}
	licenseClient, err := licensing.NewClient(client)
	if err != nil {
		return configuredProviderData{}, productClientConfigurationError()
	}
	capabilities, err := capability.NewRegistry(nil)
	if err != nil {
		return configuredProviderData{}, newConfigurationError(
			"endpoint",
			ConfigurationErrorEndpoint,
			"capability registry could not be constructed",
		)
	}
	return configuredProviderData{
		client:         client,
		capabilities:   capabilities,
		clusterClient:  clusterClient,
		iamClient:      iamClient,
		categoryClient: categoryClient,
		imageClient:    imageClient,
		subnetClient:   subnetClient,
		licenseClient:  licenseClient,
	}, nil
}

func productClientConfigurationError() *ConfigurationError {
	return newConfigurationError(
		"endpoint",
		ConfigurationErrorEndpoint,
		"Nutanix product clients could not be constructed",
	)
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

// ClusterReader returns the configured Cluster Management read capability.
func (d configuredProviderData) ClusterReader() cluster.Reader {
	return d.clusterClient
}

// CategoryReader returns the configured Prism category read capability.
func (d configuredProviderData) CategoryReader() category.Reader {
	return d.categoryClient
}

// CategoryWriter returns the configured Prism category mutation capability.
func (d configuredProviderData) CategoryWriter() category.Writer {
	return d.categoryClient
}

// OperationReader returns the configured IAM operation read capability.
func (d configuredProviderData) OperationReader() operation.Reader {
	return d.iamClient
}

// RoleReader returns the configured IAM role read capability.
func (d configuredProviderData) RoleReader() role.Reader {
	return d.iamClient
}

// ImageReader returns the configured VMM image read capability.
func (d configuredProviderData) ImageReader() image.Reader {
	return d.imageClient
}

// SubnetReader returns the configured Networking subnet read capability.
func (d configuredProviderData) SubnetReader() subnet.Reader {
	return d.subnetClient
}

// SubnetWriter returns the configured Networking subnet mutation capability.
func (d configuredProviderData) SubnetWriter() subnet.Writer {
	return d.subnetClient
}

// LicenseReader returns the configured Licensing read capability.
func (d configuredProviderData) LicenseReader() license.Reader {
	return d.licenseClient
}

// LicenseKeyReader returns the configured Licensing key-inventory capability.
func (d configuredProviderData) LicenseKeyReader() licensekey.Reader {
	return d.licenseClient
}

// StorageContainerWriter returns the configured Cluster Management storage-container capability.
func (d configuredProviderData) StorageContainerWriter() storagecontainer.Writer {
	return d.clusterClient
}

// PlacementPolicyWriter returns the configured VMM placement-policy capability.
func (d configuredProviderData) PlacementPolicyWriter() placementpolicy.Writer {
	return d.imageClient
}

// Resources returns the managed-resource registry.
func (p *nutanixProvider) Resources(context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		category.NewResource,
		subnet.NewResource,
		storagecontainer.NewResource,
		placementpolicy.NewResource,
	}
}

// DataSources returns the implemented read-only data-source registry.
func (p *nutanixProvider) DataSources(context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		cluster.NewDataSource,
		category.NewDataSource,
		image.NewDataSource,
		subnet.NewDataSource,
		role.NewDataSource,
		operation.NewDataSource,
		license.NewDataSource,
		licensekey.NewDataSource,
	}
}
