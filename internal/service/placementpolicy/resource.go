// Package placementpolicy implements the nutanix_image_placement_policy resource.
package placementpolicy

import (
	"context"
	"errors"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/ioplane/terraform-provider-nutanix/internal/nutanix/vmm"
	"github.com/ioplane/terraform-provider-nutanix/internal/task"
)

// Writer is the VMM placement-policy capability consumed by this resource.
type Writer interface {
	CreatePlacementPolicy(context.Context, vmm.PlacementPolicySpec) (vmm.AsyncOperation, error)
	GetPlacementPolicyByID(context.Context, string) (vmm.PlacementPolicyRead, error)
	UpdatePlacementPolicyByID(context.Context, string, vmm.PlacementPolicySpec, string) (vmm.AsyncOperation, error)
	DeletePlacementPolicyByID(context.Context, string) (vmm.AsyncOperation, error)
	WaitPlacementPolicyTask(context.Context, vmm.AsyncOperation) (task.Snapshot, error)
}

type resourceProviderData interface{ PlacementPolicyWriter() Writer }

type filterModel struct {
	Type           types.String `tfsdk:"type"`
	CategoryExtIDs types.List   `tfsdk:"category_ext_ids"`
}

type resourceModel struct {
	ID                  types.String `tfsdk:"id"`
	ExtID               types.String `tfsdk:"ext_id"`
	Name                types.String `tfsdk:"name"`
	Description         types.String `tfsdk:"description"`
	PlacementType       types.String `tfsdk:"placement_type"`
	ImageEntityFilter   filterModel  `tfsdk:"image_entity_filter"`
	ClusterEntityFilter filterModel  `tfsdk:"cluster_entity_filter"`
	EnforcementState    types.String `tfsdk:"enforcement_state"`
	CreateTime          types.String `tfsdk:"create_time"`
	LastUpdateTime      types.String `tfsdk:"last_update_time"`
	OwnerExtID          types.String `tfsdk:"owner_ext_id"`
	OwnerName           types.String `tfsdk:"owner_name"`
}

type resourceImpl struct{ writer Writer }

var (
	_ resource.Resource                = (*resourceImpl)(nil)
	_ resource.ResourceWithConfigure   = (*resourceImpl)(nil)
	_ resource.ResourceWithImportState = (*resourceImpl)(nil)
)

// NewResource returns the image placement-policy resource.
func NewResource() resource.Resource { return &resourceImpl{} }

func (*resourceImpl) Metadata(_ context.Context, request resource.MetadataRequest, response *resource.MetadataResponse) {
	response.TypeName = request.ProviderTypeName + "_image_placement_policy"
}

func (*resourceImpl) Schema(_ context.Context, _ resource.SchemaRequest, response *resource.SchemaResponse) {
	response.Schema = schema.Schema{
		MarkdownDescription: "Manages a Nutanix VMM v4.2 image placement policy.",
		Attributes: map[string]schema.Attribute{
			"id":                    schema.StringAttribute{Computed: true, MarkdownDescription: "Terraform identity, equal to placement-policy `extId`."},
			"ext_id":                schema.StringAttribute{Computed: true, MarkdownDescription: "Placement-policy external UUID."},
			"name":                  schema.StringAttribute{Required: true, MarkdownDescription: "Placement-policy name.", Validators: []validator.String{stringvalidator.LengthBetween(1, 256)}},
			"description":           schema.StringAttribute{Optional: true, Computed: true, MarkdownDescription: "Placement-policy description.", Validators: []validator.String{stringvalidator.LengthAtMost(1000)}},
			"placement_type":        schema.StringAttribute{Required: true, MarkdownDescription: "Placement type: `SOFT` or `HARD`.", Validators: []validator.String{stringvalidator.OneOf("SOFT", "HARD")}},
			"image_entity_filter":   filterAttribute("Categories attached to images selected by this policy."),
			"cluster_entity_filter": filterAttribute("Categories attached to clusters selected by this policy."),
			"enforcement_state":     schema.StringAttribute{Computed: true, MarkdownDescription: "Current enforcement state."},
			"create_time":           schema.StringAttribute{Computed: true, MarkdownDescription: "Policy creation time."},
			"last_update_time":      schema.StringAttribute{Computed: true, MarkdownDescription: "Last policy update time."},
			"owner_ext_id":          schema.StringAttribute{Computed: true, MarkdownDescription: "Policy owner UUID."},
			"owner_name":            schema.StringAttribute{Computed: true, MarkdownDescription: "Policy owner name."},
		},
	}
}

func filterAttribute(description string) schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		Required:            true,
		MarkdownDescription: description,
		Attributes: map[string]schema.Attribute{
			"type":             schema.StringAttribute{Required: true, MarkdownDescription: "Filter match type.", Validators: []validator.String{stringvalidator.OneOf("CATEGORIES_MATCH_ALL", "CATEGORIES_MATCH_ANY")}},
			"category_ext_ids": schema.ListAttribute{Required: true, ElementType: types.StringType, MarkdownDescription: "One to 100 category UUIDs.", Validators: []validator.List{listvalidator.SizeBetween(1, 100)}},
		},
	}
}

func (r *resourceImpl) Configure(_ context.Context, request resource.ConfigureRequest, response *resource.ConfigureResponse) {
	if request.ProviderData == nil {
		return
	}
	configured, ok := request.ProviderData.(resourceProviderData)
	if !ok {
		response.Diagnostics.AddError("Unexpected Placement Policy Resource Configure Type", "The provider supplied incompatible data to the placement-policy resource.")
		return
	}
	r.writer = configured.PlacementPolicyWriter()
	if r.writer == nil {
		response.Diagnostics.AddError("Missing Placement Policy Writer", "The provider did not configure the placement-policy writer.")
	}
}

func (r *resourceImpl) Create(ctx context.Context, request resource.CreateRequest, response *resource.CreateResponse) {
	var plan resourceModel
	response.Diagnostics.Append(request.Plan.Get(ctx, &plan)...)
	if response.Diagnostics.HasError() || !r.requireWriter(&response.Diagnostics) {
		return
	}
	spec, err := specFromModel(ctx, plan)
	if err != nil {
		addError(&response.Diagnostics, "validate create plan")
		return
	}
	operation, err := r.writer.CreatePlacementPolicy(ctx, spec)
	if err != nil {
		addError(&response.Diagnostics, "create")
		return
	}
	snapshot, err := r.writer.WaitPlacementPolicyTask(ctx, operation)
	if err != nil {
		addError(&response.Diagnostics, "wait for create")
		return
	}
	extID, err := vmm.PlacementPolicyIDFromTask(snapshot)
	if err != nil {
		response.Diagnostics.AddError("Unable to identify created Nutanix placement policy", "The completed task did not contain exactly one valid placement-policy entity.")
		return
	}
	read, err := r.writer.GetPlacementPolicyByID(ctx, extID)
	if err != nil {
		addError(&response.Diagnostics, "read created policy")
		return
	}
	state, err := stateFromRemote(ctx, plan, read.Policy)
	if err != nil {
		addError(&response.Diagnostics, "map created policy")
		return
	}
	response.Diagnostics.Append(response.State.Set(ctx, &state)...)
}

func (r *resourceImpl) Read(ctx context.Context, request resource.ReadRequest, response *resource.ReadResponse) {
	var state resourceModel
	response.Diagnostics.Append(request.State.Get(ctx, &state)...)
	if response.Diagnostics.HasError() || !r.requireWriter(&response.Diagnostics) {
		return
	}
	read, err := r.writer.GetPlacementPolicyByID(ctx, state.ID.ValueString())
	if err != nil {
		if errors.Is(err, vmm.ErrPlacementPolicyNotFound) {
			response.State.RemoveResource(ctx)
			return
		}
		addError(&response.Diagnostics, "read")
		return
	}
	state, err = stateFromRemote(ctx, state, read.Policy)
	if err != nil {
		addError(&response.Diagnostics, "map policy")
		return
	}
	response.Diagnostics.Append(response.State.Set(ctx, &state)...)
}

func (r *resourceImpl) Update(ctx context.Context, request resource.UpdateRequest, response *resource.UpdateResponse) {
	var plan resourceModel
	response.Diagnostics.Append(request.Plan.Get(ctx, &plan)...)
	if response.Diagnostics.HasError() || !r.requireWriter(&response.Diagnostics) {
		return
	}
	spec, err := specFromModel(ctx, plan)
	if err != nil {
		addError(&response.Diagnostics, "validate update plan")
		return
	}
	current, err := r.writer.GetPlacementPolicyByID(ctx, plan.ID.ValueString())
	if err != nil {
		addError(&response.Diagnostics, "read policy before update")
		return
	}
	operation, err := r.writer.UpdatePlacementPolicyByID(ctx, plan.ID.ValueString(), spec, current.ETag)
	if err != nil {
		addError(&response.Diagnostics, "update")
		return
	}
	if _, err := r.writer.WaitPlacementPolicyTask(ctx, operation); err != nil {
		addError(&response.Diagnostics, "wait for update")
		return
	}
	updated, err := r.writer.GetPlacementPolicyByID(ctx, plan.ID.ValueString())
	if err != nil {
		addError(&response.Diagnostics, "read updated policy")
		return
	}
	state, err := stateFromRemote(ctx, plan, updated.Policy)
	if err != nil {
		addError(&response.Diagnostics, "map updated policy")
		return
	}
	response.Diagnostics.Append(response.State.Set(ctx, &state)...)
}

func (r *resourceImpl) Delete(ctx context.Context, request resource.DeleteRequest, response *resource.DeleteResponse) {
	var state resourceModel
	response.Diagnostics.Append(request.State.Get(ctx, &state)...)
	if response.Diagnostics.HasError() || !r.requireWriter(&response.Diagnostics) {
		return
	}
	operation, err := r.writer.DeletePlacementPolicyByID(ctx, state.ID.ValueString())
	if err != nil {
		if errors.Is(err, vmm.ErrPlacementPolicyNotFound) {
			return
		}
		addError(&response.Diagnostics, "delete")
		return
	}
	if _, err := r.writer.WaitPlacementPolicyTask(ctx, operation); err != nil && !errors.Is(err, vmm.ErrPlacementPolicyNotFound) {
		addError(&response.Diagnostics, "wait for delete")
	}
}

func (r *resourceImpl) ImportState(ctx context.Context, request resource.ImportStateRequest, response *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), request, response)
}

func (r *resourceImpl) requireWriter(diagnostics *diag.Diagnostics) bool {
	if r.writer != nil {
		return true
	}
	diagnostics.AddError("Missing Placement Policy Writer", "The provider did not configure the placement-policy writer.")
	return false
}

func specFromModel(ctx context.Context, model resourceModel) (vmm.PlacementPolicySpec, error) {
	imageFilter, err := filterFromModel(ctx, model.ImageEntityFilter)
	if err != nil {
		return vmm.PlacementPolicySpec{}, err
	}
	clusterFilter, err := filterFromModel(ctx, model.ClusterEntityFilter)
	if err != nil {
		return vmm.PlacementPolicySpec{}, err
	}
	return vmm.PlacementPolicySpec{
		Name: model.Name.ValueString(), Description: knownString(model.Description), PlacementType: model.PlacementType.ValueString(),
		ImageEntityFilter: imageFilter, ClusterEntityFilter: clusterFilter,
	}, nil
}

func filterFromModel(ctx context.Context, model filterModel) (vmm.CategoryFilter, error) {
	var ids []string
	if diagnostics := model.CategoryExtIDs.ElementsAs(ctx, &ids, false); diagnostics.HasError() {
		return vmm.CategoryFilter{}, errors.New("category filter list is invalid")
	}
	return vmm.CategoryFilter{Type: model.Type.ValueString(), CategoryExtIDs: ids}, nil
}

func stateFromRemote(ctx context.Context, prior resourceModel, policy vmm.PlacementPolicy) (resourceModel, error) {
	if policy.ExtID == nil || policy.Name == nil || policy.PlacementType == nil || policy.ImageEntityFilter == nil || policy.ClusterEntityFilter == nil {
		return resourceModel{}, vmm.ErrInvalidPlacementPolicy
	}
	prior.ID = types.StringPointerValue(policy.ExtID)
	prior.ExtID = types.StringPointerValue(policy.ExtID)
	prior.Name = types.StringPointerValue(policy.Name)
	prior.Description = types.StringPointerValue(policy.Description)
	prior.PlacementType = types.StringPointerValue(policy.PlacementType)
	var err error
	prior.ImageEntityFilter, err = stateFilter(ctx, *policy.ImageEntityFilter)
	if err != nil {
		return resourceModel{}, err
	}
	prior.ClusterEntityFilter, err = stateFilter(ctx, *policy.ClusterEntityFilter)
	if err != nil {
		return resourceModel{}, err
	}
	prior.EnforcementState = types.StringPointerValue(policy.EnforcementState)
	prior.CreateTime = types.StringPointerValue(policy.CreateTime)
	prior.LastUpdateTime = types.StringPointerValue(policy.LastUpdateTime)
	prior.OwnerExtID = types.StringPointerValue(policy.OwnerExtID)
	prior.OwnerName = types.StringPointerValue(policy.OwnerName)
	return prior, nil
}

func stateFilter(ctx context.Context, filter vmm.CategoryFilter) (filterModel, error) {
	values, diagnostics := types.ListValueFrom(ctx, types.StringType, filter.CategoryExtIDs)
	if diagnostics.HasError() {
		return filterModel{}, errors.New("category filter state is invalid")
	}
	return filterModel{Type: types.StringValue(filter.Type), CategoryExtIDs: values}, nil
}

func knownString(value types.String) *string {
	if value.IsNull() || value.IsUnknown() {
		return nil
	}
	selected := value.ValueString()
	return &selected
}

func addError(diagnostics *diag.Diagnostics, operation string) {
	diagnostics.AddError("Nutanix placement-policy operation failed", "The placement-policy "+operation+" operation could not be completed.")
}
