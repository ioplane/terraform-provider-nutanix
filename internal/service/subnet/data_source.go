// Package subnet implements the nutanix_subnet_v2 data source.
package subnet

import (
	"context"
	"regexp"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/ioplane/terraform-provider-nutanix/internal/nutanix/networking"
)

var (
	uuidPattern = regexp.MustCompile(
		`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`,
	)
	addressObjectType = types.ObjectType{AttrTypes: map[string]attr.Type{
		"value":         types.StringType,
		"prefix_length": types.Int64Type,
	}}
	ipSubnetObjectType = types.ObjectType{AttrTypes: map[string]attr.Type{
		"ip":            types.ListType{ElemType: addressObjectType},
		"prefix_length": types.Int64Type,
	}}
	poolObjectType = types.ObjectType{AttrTypes: map[string]attr.Type{
		"start_ip": types.ListType{ElemType: addressObjectType},
		"end_ip":   types.ListType{ElemType: addressObjectType},
	}}
	protocolConfigObjectType = types.ObjectType{AttrTypes: map[string]attr.Type{
		"ip_subnet":           types.ListType{ElemType: ipSubnetObjectType},
		"default_gateway_ip":  types.ListType{ElemType: addressObjectType},
		"dhcp_server_address": types.ListType{ElemType: addressObjectType},
		"pool_list":           types.ListType{ElemType: poolObjectType},
	}}
	ipConfigObjectType = types.ObjectType{AttrTypes: map[string]attr.Type{
		"ipv4": types.ListType{ElemType: protocolConfigObjectType},
		"ipv6": types.ListType{ElemType: protocolConfigObjectType},
	}}
)

// Reader is the Networking subnet capability consumed by this data source.
type Reader interface {
	GetSubnetByID(context.Context, string) (networking.Subnet, error)
}

type providerData interface {
	SubnetReader() Reader
}

type dataSource struct {
	reader Reader
}

type dataSourceModel struct {
	ID                     types.String `tfsdk:"id"`
	ExtID                  types.String `tfsdk:"ext_id"`
	Name                   types.String `tfsdk:"name"`
	Description            types.String `tfsdk:"description"`
	SubnetType             types.String `tfsdk:"subnet_type"`
	NetworkID              types.Int64  `tfsdk:"network_id"`
	IPConfig               types.List   `tfsdk:"ip_config"`
	ClusterReference       types.String `tfsdk:"cluster_reference"`
	VirtualSwitchReference types.String `tfsdk:"virtual_switch_reference"`
	VPCReference           types.String `tfsdk:"vpc_reference"`
	IsNATEnabled           types.Bool   `tfsdk:"is_nat_enabled"`
	IsExternal             types.Bool   `tfsdk:"is_external"`
	BridgeName             types.String `tfsdk:"bridge_name"`
	IsAdvancedNetworking   types.Bool   `tfsdk:"is_advanced_networking"`
}

type ipConfigModel struct {
	IPv4 types.List `tfsdk:"ipv4"`
	IPv6 types.List `tfsdk:"ipv6"`
}

type protocolConfigModel struct {
	IPSubnet          types.List `tfsdk:"ip_subnet"`
	DefaultGatewayIP  types.List `tfsdk:"default_gateway_ip"`
	DHCPServerAddress types.List `tfsdk:"dhcp_server_address"`
	PoolList          types.List `tfsdk:"pool_list"`
}

type ipSubnetModel struct {
	IP           types.List  `tfsdk:"ip"`
	PrefixLength types.Int64 `tfsdk:"prefix_length"`
}

type addressModel struct {
	Value        types.String `tfsdk:"value"`
	PrefixLength types.Int64  `tfsdk:"prefix_length"`
}

type poolModel struct {
	StartIP types.List `tfsdk:"start_ip"`
	EndIP   types.List `tfsdk:"end_ip"`
}

var (
	_ datasource.DataSource              = (*dataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*dataSource)(nil)
)

// NewDataSource returns a new nutanix_subnet_v2 data source.
func NewDataSource() datasource.DataSource {
	return &dataSource{}
}

func (*dataSource) Metadata(
	_ context.Context,
	request datasource.MetadataRequest,
	response *datasource.MetadataResponse,
) {
	response.TypeName = request.ProviderTypeName + "_subnet_v2"
}

func (*dataSource) Schema(
	_ context.Context,
	_ datasource.SchemaRequest,
	response *datasource.SchemaResponse,
) {
	response.Schema = schema.Schema{
		Description: "Reads one Nutanix subnet through the Networking v4.3 API.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Terraform identity, equal to the validated caller-supplied ext_id.",
			},
			"ext_id": schema.StringAttribute{
				Required:    true,
				Description: "External UUID of the Nutanix subnet.",
				Validators: []validator.String{stringvalidator.RegexMatches(
					uuidPattern,
					"must be a canonical UUID",
				)},
			},
			"name":                     schema.StringAttribute{Computed: true},
			"description":              schema.StringAttribute{Computed: true},
			"subnet_type":              schema.StringAttribute{Computed: true},
			"network_id":               schema.Int64Attribute{Computed: true},
			"ip_config":                ipConfigAttribute(),
			"cluster_reference":        schema.StringAttribute{Computed: true},
			"virtual_switch_reference": schema.StringAttribute{Computed: true},
			"vpc_reference":            schema.StringAttribute{Computed: true},
			"is_nat_enabled":           schema.BoolAttribute{Computed: true},
			"is_external":              schema.BoolAttribute{Computed: true},
			"bridge_name":              schema.StringAttribute{Computed: true},
			"is_advanced_networking":   schema.BoolAttribute{Computed: true},
		},
	}
}

func ipConfigAttribute() schema.ListNestedAttribute {
	return schema.ListNestedAttribute{
		Computed:    true,
		Description: "Dual-stack address configuration returned by Nutanix.",
		NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
			"ipv4": protocolConfigAttribute(),
			"ipv6": protocolConfigAttribute(),
		}},
	}
}

func protocolConfigAttribute() schema.ListNestedAttribute {
	return schema.ListNestedAttribute{
		Computed: true,
		NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
			"ip_subnet": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"ip":            addressAttribute(),
					"prefix_length": schema.Int64Attribute{Computed: true},
				}},
			},
			"default_gateway_ip":  addressAttribute(),
			"dhcp_server_address": addressAttribute(),
			"pool_list": schema.ListNestedAttribute{
				Computed: true,
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"start_ip": addressAttribute(),
					"end_ip":   addressAttribute(),
				}},
			},
		}},
	}
}

func addressAttribute() schema.ListNestedAttribute {
	return schema.ListNestedAttribute{
		Computed: true,
		NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
			"value":         schema.StringAttribute{Computed: true},
			"prefix_length": schema.Int64Attribute{Computed: true},
		}},
	}
}

func (d *dataSource) Configure(
	_ context.Context,
	request datasource.ConfigureRequest,
	response *datasource.ConfigureResponse,
) {
	if request.ProviderData == nil {
		return
	}
	configured, ok := request.ProviderData.(providerData)
	if !ok {
		response.Diagnostics.AddError(
			"Unexpected Subnet Data Source Configure Type",
			"The provider supplied incompatible data to the subnet data source.",
		)
		return
	}
	d.reader = configured.SubnetReader()
	if d.reader == nil {
		response.Diagnostics.AddError(
			"Missing Subnet Reader",
			"The provider did not configure the subnet reader.",
		)
	}
}

func (d *dataSource) Read(
	ctx context.Context,
	request datasource.ReadRequest,
	response *datasource.ReadResponse,
) {
	if d.reader == nil {
		response.Diagnostics.AddError(
			"Missing Subnet Reader",
			"The provider did not configure the subnet reader.",
		)
		return
	}
	var config dataSourceModel
	response.Diagnostics.Append(request.Config.Get(ctx, &config)...)
	if response.Diagnostics.HasError() {
		return
	}
	extID, diagnostics := extIDFromModel(config.ExtID)
	response.Diagnostics.Append(diagnostics...)
	if response.Diagnostics.HasError() {
		return
	}
	current, err := d.reader.GetSubnetByID(ctx, extID)
	if err != nil {
		addReadError(&response.Diagnostics)
		return
	}
	state, diagnostics := stateFromSubnet(ctx, config, extID, current)
	if len(diagnostics) != 0 {
		addReadError(&response.Diagnostics)
		return
	}
	response.Diagnostics.Append(response.State.Set(ctx, &state)...)
}

func extIDFromModel(value types.String) (string, diag.Diagnostics) {
	var diagnostics diag.Diagnostics
	if value.IsUnknown() || value.IsNull() {
		diagnostics.AddAttributeError(
			path.Root("ext_id"),
			"Invalid Nutanix Subnet Query",
			"The subnet ext_id must be known before the request can be constructed.",
		)
		return "", diagnostics
	}
	extID := value.ValueString()
	if _, err := uuid.Parse(extID); err != nil {
		diagnostics.AddAttributeError(
			path.Root("ext_id"),
			"Invalid Nutanix Subnet Query",
			"The subnet ext_id must be a valid UUID.",
		)
		return "", diagnostics
	}
	return extID, diagnostics
}

func addReadError(diagnostics *diag.Diagnostics) {
	diagnostics.AddError(
		"Unable to Read Nutanix Subnet",
		"The provider could not read and map the requested Nutanix subnet.",
	)
}

func stateFromSubnet(
	ctx context.Context,
	config dataSourceModel,
	extID string,
	subnet networking.Subnet,
) (dataSourceModel, diag.Diagnostics) {
	ipConfig, diagnostics := ipConfigValues(ctx, subnet.IPConfig)
	config.ID = types.StringValue(extID)
	config.ExtID = types.StringValue(extID)
	config.Name = types.StringPointerValue(subnet.Name)
	config.Description = types.StringPointerValue(subnet.Description)
	config.SubnetType = types.StringPointerValue(subnet.SubnetType)
	config.NetworkID = types.Int64PointerValue(subnet.NetworkID)
	config.IPConfig = ipConfig
	config.ClusterReference = types.StringPointerValue(subnet.ClusterReference)
	config.VirtualSwitchReference = types.StringPointerValue(subnet.VirtualSwitchReference)
	config.VPCReference = types.StringPointerValue(subnet.VPCReference)
	config.IsNATEnabled = types.BoolPointerValue(subnet.IsNATEnabled)
	config.IsExternal = types.BoolPointerValue(subnet.IsExternal)
	config.BridgeName = types.StringPointerValue(subnet.BridgeName)
	config.IsAdvancedNetworking = types.BoolPointerValue(subnet.IsAdvancedNetworking)
	return config, diagnostics
}

func ipConfigValues(
	ctx context.Context,
	configs *[]networking.IPConfig,
) (types.List, diag.Diagnostics) {
	if configs == nil {
		return types.ListNull(ipConfigObjectType), nil
	}
	var diagnostics diag.Diagnostics
	models := make([]ipConfigModel, len(*configs))
	for index := range *configs {
		ipv4, current := ipv4ConfigValue(ctx, (*configs)[index].IPv4)
		diagnostics.Append(current...)
		ipv6, current := ipv6ConfigValue(ctx, (*configs)[index].IPv6)
		diagnostics.Append(current...)
		models[index] = ipConfigModel{IPv4: ipv4, IPv6: ipv6}
	}
	values, current := types.ListValueFrom(ctx, ipConfigObjectType, models)
	diagnostics.Append(current...)
	return values, diagnostics
}

func ipv4ConfigValue(
	ctx context.Context,
	config *networking.IPv4Config,
) (types.List, diag.Diagnostics) {
	if config == nil {
		return types.ListNull(protocolConfigObjectType), nil
	}
	var diagnostics diag.Diagnostics
	ipSubnet, current := ipv4SubnetValue(ctx, config.IPSubnet)
	diagnostics.Append(current...)
	defaultGateway, current := ipv4AddressValue(ctx, config.DefaultGatewayIP)
	diagnostics.Append(current...)
	dhcpServer, current := ipv4AddressValue(ctx, config.DHCPServerAddress)
	diagnostics.Append(current...)
	pools, current := ipv4PoolValues(ctx, config.PoolList)
	diagnostics.Append(current...)
	values, current := types.ListValueFrom(ctx, protocolConfigObjectType, []protocolConfigModel{{
		IPSubnet:          ipSubnet,
		DefaultGatewayIP:  defaultGateway,
		DHCPServerAddress: dhcpServer,
		PoolList:          pools,
	}})
	diagnostics.Append(current...)
	return values, diagnostics
}

func ipv6ConfigValue(
	ctx context.Context,
	config *networking.IPv6Config,
) (types.List, diag.Diagnostics) {
	if config == nil {
		return types.ListNull(protocolConfigObjectType), nil
	}
	var diagnostics diag.Diagnostics
	ipSubnet, current := ipv6SubnetValue(ctx, config.IPSubnet)
	diagnostics.Append(current...)
	defaultGateway, current := ipv6AddressValue(ctx, config.DefaultGatewayIP)
	diagnostics.Append(current...)
	dhcpServer, current := ipv6AddressValue(ctx, config.DHCPServerAddress)
	diagnostics.Append(current...)
	pools, current := ipv6PoolValues(ctx, config.PoolList)
	diagnostics.Append(current...)
	values, current := types.ListValueFrom(ctx, protocolConfigObjectType, []protocolConfigModel{{
		IPSubnet:          ipSubnet,
		DefaultGatewayIP:  defaultGateway,
		DHCPServerAddress: dhcpServer,
		PoolList:          pools,
	}})
	diagnostics.Append(current...)
	return values, diagnostics
}

func ipv4SubnetValue(
	ctx context.Context,
	subnet *networking.IPv4Subnet,
) (types.List, diag.Diagnostics) {
	if subnet == nil {
		return types.ListNull(ipSubnetObjectType), nil
	}
	ip, diagnostics := ipv4AddressValue(ctx, subnet.IP)
	values, current := types.ListValueFrom(ctx, ipSubnetObjectType, []ipSubnetModel{{
		IP:           ip,
		PrefixLength: types.Int64PointerValue(subnet.PrefixLength),
	}})
	diagnostics.Append(current...)
	return values, diagnostics
}

func ipv6SubnetValue(
	ctx context.Context,
	subnet *networking.IPv6Subnet,
) (types.List, diag.Diagnostics) {
	if subnet == nil {
		return types.ListNull(ipSubnetObjectType), nil
	}
	ip, diagnostics := ipv6AddressValue(ctx, subnet.IP)
	values, current := types.ListValueFrom(ctx, ipSubnetObjectType, []ipSubnetModel{{
		IP:           ip,
		PrefixLength: types.Int64PointerValue(subnet.PrefixLength),
	}})
	diagnostics.Append(current...)
	return values, diagnostics
}

func ipv4AddressValue(
	ctx context.Context,
	address *networking.IPv4Address,
) (types.List, diag.Diagnostics) {
	if address == nil {
		return types.ListNull(addressObjectType), nil
	}
	return addressValue(ctx, address.Value, address.PrefixLength)
}

func ipv6AddressValue(
	ctx context.Context,
	address *networking.IPv6Address,
) (types.List, diag.Diagnostics) {
	if address == nil {
		return types.ListNull(addressObjectType), nil
	}
	return addressValue(ctx, address.Value, address.PrefixLength)
}

func addressValue(
	ctx context.Context,
	value *string,
	prefixLength *int64,
) (types.List, diag.Diagnostics) {
	return types.ListValueFrom(ctx, addressObjectType, []addressModel{{
		Value:        types.StringPointerValue(value),
		PrefixLength: types.Int64PointerValue(prefixLength),
	}})
}

func ipv4PoolValues(
	ctx context.Context,
	pools *[]networking.IPv4Pool,
) (types.List, diag.Diagnostics) {
	if pools == nil {
		return types.ListNull(poolObjectType), nil
	}
	var diagnostics diag.Diagnostics
	models := make([]poolModel, len(*pools))
	for index := range *pools {
		start, current := ipv4AddressValue(ctx, (*pools)[index].StartIP)
		diagnostics.Append(current...)
		end, current := ipv4AddressValue(ctx, (*pools)[index].EndIP)
		diagnostics.Append(current...)
		models[index] = poolModel{StartIP: start, EndIP: end}
	}
	values, current := types.ListValueFrom(ctx, poolObjectType, models)
	diagnostics.Append(current...)
	return values, diagnostics
}

func ipv6PoolValues(
	ctx context.Context,
	pools *[]networking.IPv6Pool,
) (types.List, diag.Diagnostics) {
	if pools == nil {
		return types.ListNull(poolObjectType), nil
	}
	var diagnostics diag.Diagnostics
	models := make([]poolModel, len(*pools))
	for index := range *pools {
		start, current := ipv6AddressValue(ctx, (*pools)[index].StartIP)
		diagnostics.Append(current...)
		end, current := ipv6AddressValue(ctx, (*pools)[index].EndIP)
		diagnostics.Append(current...)
		models[index] = poolModel{StartIP: start, EndIP: end}
	}
	values, current := types.ListValueFrom(ctx, poolObjectType, models)
	diagnostics.Append(current...)
	return values, diagnostics
}
