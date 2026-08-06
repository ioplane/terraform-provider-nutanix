// Package subnet implements the nutanix_subnet managed resource.
package subnet

import (
	"context"
	"errors"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/ioplane/terraform-provider-nutanix/internal/nutanix/networking"
	"github.com/ioplane/terraform-provider-nutanix/internal/task"
)

var subnetUUIDPattern = regexp.MustCompile(
	`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`,
)

// Writer is the Networking subnet capability consumed by this resource.
type Writer interface {
	CreateSubnet(context.Context, networking.SubnetSpec) (networking.AsyncOperation, error)
	GetSubnetByIDWithETag(context.Context, string) (networking.SubnetRead, error)
	UpdateSubnetByID(context.Context, string, networking.SubnetSpec, string) (networking.AsyncOperation, error)
	DeleteSubnetByID(context.Context, string) (networking.AsyncOperation, error)
	WaitTask(context.Context, networking.AsyncOperation) (task.Snapshot, error)
}

type resourceProviderData interface {
	SubnetWriter() Writer
}

type resourceModel struct {
	ID                     types.String `tfsdk:"id"`
	ExtID                  types.String `tfsdk:"ext_id"`
	Name                   types.String `tfsdk:"name"`
	Description            types.String `tfsdk:"description"`
	SubnetType             types.String `tfsdk:"subnet_type"`
	NetworkID              types.Int64  `tfsdk:"network_id"`
	ClusterReference       types.String `tfsdk:"cluster_reference"`
	VirtualSwitchReference types.String `tfsdk:"virtual_switch_reference"`
	VPCReference           types.String `tfsdk:"vpc_reference"`
	IsNATEnabled           types.Bool   `tfsdk:"is_nat_enabled"`
	IsExternal             types.Bool   `tfsdk:"is_external"`
	BridgeName             types.String `tfsdk:"bridge_name"`
	IsAdvancedNetworking   types.Bool   `tfsdk:"is_advanced_networking"`
}

type resourceImpl struct {
	writer Writer
}

var (
	_ resource.Resource                = (*resourceImpl)(nil)
	_ resource.ResourceWithConfigure   = (*resourceImpl)(nil)
	_ resource.ResourceWithImportState = (*resourceImpl)(nil)
)

// NewResource returns the Networking subnet resource.
func NewResource() resource.Resource {
	return &resourceImpl{}
}

func (*resourceImpl) Metadata(
	_ context.Context,
	request resource.MetadataRequest,
	response *resource.MetadataResponse,
) {
	response.TypeName = request.ProviderTypeName + "_subnet"
}

func (*resourceImpl) Schema(
	_ context.Context,
	_ resource.SchemaRequest,
	response *resource.SchemaResponse,
) {
	response.Schema = schema.Schema{
		MarkdownDescription: "Manages a Nutanix Networking v4.3 subnet.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Terraform identity, equal to the Networking subnet `extId`.",
			},
			"ext_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Networking subnet external UUID.",
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Subnet name.",
				Validators:          []validator.String{stringvalidator.LengthBetween(1, 128)},
			},
			"description": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Subnet description.",
				Validators:          []validator.String{stringvalidator.LengthAtMost(1000)},
			},
			"subnet_type": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Subnet type: `VLAN` or `OVERLAY`.",
				Validators:          []validator.String{stringvalidator.OneOf("VLAN", "OVERLAY")},
			},
			"network_id": schema.Int64Attribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "VLAN ID or overlay VNI assigned by Networking.",
				Validators:          []validator.Int64{int64validator.Between(0, 16777215)},
			},
			"cluster_reference":        referenceAttribute("Cluster UUID."),
			"virtual_switch_reference": referenceAttribute("Virtual switch UUID."),
			"vpc_reference":            referenceAttribute("VPC UUID."),
			"is_nat_enabled": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Whether NAT is enabled for the subnet.",
			},
			"is_external": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Whether the subnet is used for external connectivity.",
			},
			"bridge_name": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Host bridge name.",
				Validators:          []validator.String{stringvalidator.LengthAtMost(128)},
			},
			"is_advanced_networking": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Whether advanced networking is enabled.",
			},
		},
	}
}

func referenceAttribute(description string) schema.StringAttribute {
	return schema.StringAttribute{
		Optional:            true,
		Computed:            true,
		MarkdownDescription: description,
		Validators: []validator.String{stringvalidator.RegexMatches(
			subnetUUIDPattern,
			"must be a UUID",
		)},
	}
}

func (r *resourceImpl) Configure(
	_ context.Context,
	request resource.ConfigureRequest,
	response *resource.ConfigureResponse,
) {
	if request.ProviderData == nil {
		return
	}
	configured, ok := request.ProviderData.(resourceProviderData)
	if !ok {
		response.Diagnostics.AddError(
			"Unexpected Subnet Resource Configure Type",
			"The provider supplied incompatible data to the subnet resource.",
		)
		return
	}
	r.writer = configured.SubnetWriter()
	if r.writer == nil {
		response.Diagnostics.AddError("Missing Subnet Writer", "The provider did not configure the subnet writer.")
	}
}

func (r *resourceImpl) Create(
	ctx context.Context,
	request resource.CreateRequest,
	response *resource.CreateResponse,
) {
	var plan resourceModel
	response.Diagnostics.Append(request.Plan.Get(ctx, &plan)...)
	if response.Diagnostics.HasError() {
		return
	}
	if r.writer == nil {
		response.Diagnostics.AddError("Missing Subnet Writer", "The provider did not configure the subnet writer.")
		return
	}
	operation, err := r.writer.CreateSubnet(ctx, specFromModel(plan))
	if err != nil {
		addMutationError(&response.Diagnostics, "create")
		return
	}
	snapshot, err := r.writer.WaitTask(ctx, operation)
	if err != nil {
		addMutationError(&response.Diagnostics, "wait for create")
		return
	}
	extID, err := networking.SubnetIDFromTask(snapshot)
	if err != nil {
		response.Diagnostics.AddError("Unable to identify created Nutanix subnet", "The completed task did not contain exactly one valid subnet entity.")
		return
	}
	read, err := r.writer.GetSubnetByIDWithETag(ctx, extID)
	if err != nil {
		addMutationError(&response.Diagnostics, "read created subnet")
		return
	}
	state, err := stateFromRemote(plan, read.Subnet)
	if err != nil {
		response.Diagnostics.AddError("Unable to map created Nutanix subnet", "The Networking response did not contain a valid subnet identity.")
		return
	}
	response.Diagnostics.Append(response.State.Set(ctx, &state)...)
}

func (r *resourceImpl) Read(
	ctx context.Context,
	request resource.ReadRequest,
	response *resource.ReadResponse,
) {
	var state resourceModel
	response.Diagnostics.Append(request.State.Get(ctx, &state)...)
	if response.Diagnostics.HasError() {
		return
	}
	if r.writer == nil {
		response.Diagnostics.AddError("Missing Subnet Writer", "The provider did not configure the subnet writer.")
		return
	}
	read, err := r.writer.GetSubnetByIDWithETag(ctx, state.ID.ValueString())
	if err != nil {
		if errors.Is(err, networking.ErrSubnetNotFound) {
			response.State.RemoveResource(ctx)
			return
		}
		addMutationError(&response.Diagnostics, "read")
		return
	}
	state, err = stateFromRemote(state, read.Subnet)
	if err != nil {
		response.Diagnostics.AddError("Unable to map Nutanix subnet", "The Networking response did not contain a valid subnet identity.")
		return
	}
	response.Diagnostics.Append(response.State.Set(ctx, &state)...)
}

func (r *resourceImpl) Update(
	ctx context.Context,
	request resource.UpdateRequest,
	response *resource.UpdateResponse,
) {
	var plan resourceModel
	response.Diagnostics.Append(request.Plan.Get(ctx, &plan)...)
	if response.Diagnostics.HasError() {
		return
	}
	if r.writer == nil {
		response.Diagnostics.AddError("Missing Subnet Writer", "The provider did not configure the subnet writer.")
		return
	}
	current, err := r.writer.GetSubnetByIDWithETag(ctx, plan.ID.ValueString())
	if err != nil {
		addMutationError(&response.Diagnostics, "read subnet before update")
		return
	}
	operation, err := r.writer.UpdateSubnetByID(ctx, plan.ID.ValueString(), specFromModel(plan), current.ETag)
	if err != nil {
		addMutationError(&response.Diagnostics, "update")
		return
	}
	if _, err := r.writer.WaitTask(ctx, operation); err != nil {
		addMutationError(&response.Diagnostics, "wait for update")
		return
	}
	updated, err := r.writer.GetSubnetByIDWithETag(ctx, plan.ID.ValueString())
	if err != nil {
		addMutationError(&response.Diagnostics, "read updated subnet")
		return
	}
	state, err := stateFromRemote(plan, updated.Subnet)
	if err != nil {
		response.Diagnostics.AddError("Unable to map updated Nutanix subnet", "The Networking response did not contain a valid subnet identity.")
		return
	}
	response.Diagnostics.Append(response.State.Set(ctx, &state)...)
}

func (r *resourceImpl) Delete(
	ctx context.Context,
	request resource.DeleteRequest,
	response *resource.DeleteResponse,
) {
	var state resourceModel
	response.Diagnostics.Append(request.State.Get(ctx, &state)...)
	if response.Diagnostics.HasError() {
		return
	}
	if r.writer == nil {
		response.Diagnostics.AddError("Missing Subnet Writer", "The provider did not configure the subnet writer.")
		return
	}
	operation, err := r.writer.DeleteSubnetByID(ctx, state.ID.ValueString())
	if err != nil {
		if errors.Is(err, networking.ErrSubnetNotFound) {
			return
		}
		addMutationError(&response.Diagnostics, "delete")
		return
	}
	if _, err := r.writer.WaitTask(ctx, operation); err != nil && !errors.Is(err, networking.ErrSubnetNotFound) {
		addMutationError(&response.Diagnostics, "wait for delete")
	}
}

func (r *resourceImpl) ImportState(
	ctx context.Context,
	request resource.ImportStateRequest,
	response *resource.ImportStateResponse,
) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), request, response)
}

func specFromModel(model resourceModel) networking.SubnetSpec {
	var networkID *int64
	if model.SubnetType.ValueString() != "OVERLAY" {
		networkID = knownInt64Pointer(model.NetworkID)
	}
	return networking.SubnetSpec{
		Name:                   model.Name.ValueString(),
		Description:            knownStringPointer(model.Description),
		SubnetType:             model.SubnetType.ValueString(),
		NetworkID:              networkID,
		ClusterReference:       knownStringPointer(model.ClusterReference),
		VirtualSwitchReference: knownStringPointer(model.VirtualSwitchReference),
		VPCReference:           knownStringPointer(model.VPCReference),
		IsNATEnabled:           knownBoolPointer(model.IsNATEnabled),
		IsExternal:             knownBoolPointer(model.IsExternal),
		BridgeName:             knownStringPointer(model.BridgeName),
		IsAdvancedNetworking:   knownBoolPointer(model.IsAdvancedNetworking),
	}
}

func stateFromRemote(prior resourceModel, subnet networking.Subnet) (resourceModel, error) {
	if subnet.ExtID == nil {
		return resourceModel{}, networking.ErrInvalidSubnet
	}
	prior.ID = types.StringPointerValue(subnet.ExtID)
	prior.ExtID = types.StringPointerValue(subnet.ExtID)
	prior.Name = types.StringPointerValue(subnet.Name)
	prior.Description = types.StringPointerValue(subnet.Description)
	prior.SubnetType = types.StringPointerValue(subnet.SubnetType)
	prior.NetworkID = types.Int64PointerValue(subnet.NetworkID)
	prior.ClusterReference = types.StringPointerValue(subnet.ClusterReference)
	prior.VirtualSwitchReference = types.StringPointerValue(subnet.VirtualSwitchReference)
	prior.VPCReference = types.StringPointerValue(subnet.VPCReference)
	prior.IsNATEnabled = types.BoolPointerValue(subnet.IsNATEnabled)
	prior.IsExternal = types.BoolPointerValue(subnet.IsExternal)
	prior.BridgeName = types.StringPointerValue(subnet.BridgeName)
	prior.IsAdvancedNetworking = types.BoolPointerValue(subnet.IsAdvancedNetworking)
	return prior, nil
}

func knownStringPointer(value types.String) *string {
	if value.IsNull() || value.IsUnknown() {
		return nil
	}
	text := value.ValueString()
	return &text
}

func knownInt64Pointer(value types.Int64) *int64 {
	if value.IsNull() || value.IsUnknown() {
		return nil
	}
	number := value.ValueInt64()
	return &number
}

func knownBoolPointer(value types.Bool) *bool {
	if value.IsNull() || value.IsUnknown() {
		return nil
	}
	boolean := value.ValueBool()
	return &boolean
}

func addMutationError(diagnostics *diag.Diagnostics, operation string) {
	diagnostics.AddError(
		"Unable to "+operation+" Nutanix subnet",
		"The Networking v4.3 subnet operation failed or returned an invalid response.",
	)
}
