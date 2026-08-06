package category

import (
	"context"
	"errors"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/ioplane/terraform-provider-nutanix/internal/nutanix/prism"
)

var (
	categoryValuePattern = regexp.MustCompile(`^[^$/,!"#%'()*+:;=?\\|\[\]^&][^/,!"#%'()*+:;=?\\|\[\]^&]{0,63}$`)
	categoryUUIDPattern  = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
)

// Writer is the Prism category capability consumed by this resource.
type Writer interface {
	CreateCategory(context.Context, prism.CategorySpec) (prism.Category, error)
	GetCategory(context.Context, string) (prism.Category, string, error)
	UpdateCategory(context.Context, string, string, prism.CategorySpec) error
	DeleteCategory(context.Context, string) error
}

type resourceProviderData interface {
	CategoryWriter() Writer
}

type resourceModel struct {
	ID          types.String `tfsdk:"id"`
	Key         types.String `tfsdk:"key"`
	Value       types.String `tfsdk:"value"`
	Description types.String `tfsdk:"description"`
	OwnerUUID   types.String `tfsdk:"owner_uuid"`
	Type        types.String `tfsdk:"type"`
}

type resourceImpl struct {
	writer Writer
}

var (
	_ resource.Resource                = (*resourceImpl)(nil)
	_ resource.ResourceWithConfigure   = (*resourceImpl)(nil)
	_ resource.ResourceWithImportState = (*resourceImpl)(nil)
)

// NewResource returns the Prism category resource.
func NewResource() resource.Resource {
	return &resourceImpl{}
}

func (*resourceImpl) Metadata(
	_ context.Context,
	request resource.MetadataRequest,
	response *resource.MetadataResponse,
) {
	response.TypeName = request.ProviderTypeName + "_category"
}

func (*resourceImpl) Schema(
	_ context.Context,
	_ resource.SchemaRequest,
	response *resource.SchemaResponse,
) {
	replace := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	response.Schema = schema.Schema{
		MarkdownDescription: "Manages a user-defined Nutanix Prism category through the Prism v4.3 API.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
				MarkdownDescription: "Prism category `extId`.",
			},
			"key": schema.StringAttribute{
				Required:      true,
				PlanModifiers: replace,
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 64),
					stringvalidator.RegexMatches(categoryValuePattern, "must satisfy the Prism category key character constraints"),
				},
				MarkdownDescription: "Immutable category key in the Prism `key:value` identity.",
			},
			"value": schema.StringAttribute{
				Required: true,
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 64),
					stringvalidator.RegexMatches(categoryValuePattern, "must satisfy the Prism category value character constraints"),
				},
				MarkdownDescription: "Category value. Updating it preserves the remote `extId`.",
			},
			"description": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Optional category description, limited to 512 characters by Prism.",
				Validators:          []validator.String{stringvalidator.LengthAtMost(512)},
			},
			"owner_uuid": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Optional owner UUID for an update; Prism assigns the creator on create.",
				Validators:          []validator.String{stringvalidator.RegexMatches(categoryUUIDPattern, "must be a UUID")},
			},
			"type": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Prism category type. User-defined resources report `USER`.",
			},
		},
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
			"Unexpected Category Resource Configure Type",
			"The provider supplied incompatible data to the category resource.",
		)
		return
	}
	r.writer = configured.CategoryWriter()
	if r.writer == nil {
		response.Diagnostics.AddError(
			"Missing Category Writer",
			"The provider did not configure the category writer.",
		)
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
		response.Diagnostics.AddError("Missing Category Writer", "The provider did not configure the category writer.")
		return
	}
	category, err := r.writer.CreateCategory(ctx, categorySpec(plan))
	if err != nil {
		response.Diagnostics.AddError("Unable to create Nutanix category", "The Prism category create operation failed.")
		return
	}
	state, err := stateFromRemote(plan, category)
	if err != nil {
		response.Diagnostics.AddError("Unable to map created Nutanix category", "The Prism response did not contain a valid category identity.")
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
		response.Diagnostics.AddError("Missing Category Writer", "The provider did not configure the category writer.")
		return
	}
	category, _, err := r.writer.GetCategory(ctx, state.ID.ValueString())
	if err != nil {
		if errors.Is(err, prism.ErrCategoryNotFound) {
			response.State.RemoveResource(ctx)
			return
		}
		response.Diagnostics.AddError("Unable to read Nutanix category", "The Prism category read operation failed.")
		return
	}
	state, err = stateFromRemote(state, category)
	if err != nil {
		response.Diagnostics.AddError("Unable to map Nutanix category", "The Prism response did not contain a valid category identity.")
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
		response.Diagnostics.AddError("Missing Category Writer", "The provider did not configure the category writer.")
		return
	}
	_, etag, err := r.writer.GetCategory(ctx, plan.ID.ValueString())
	if err != nil {
		response.Diagnostics.AddError("Unable to read Nutanix category before update", "The Prism category ETag could not be obtained.")
		return
	}
	if err := r.writer.UpdateCategory(ctx, plan.ID.ValueString(), etag, categorySpec(plan)); err != nil {
		response.Diagnostics.AddError("Unable to update Nutanix category", "The conditional Prism category update failed.")
		return
	}
	category, _, err := r.writer.GetCategory(ctx, plan.ID.ValueString())
	if err != nil {
		response.Diagnostics.AddError("Unable to read updated Nutanix category", "The category update completed but its state could not be refreshed.")
		return
	}
	state, err := stateFromRemote(plan, category)
	if err != nil {
		response.Diagnostics.AddError("Unable to map updated Nutanix category", "The Prism response did not contain a valid category identity.")
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
		response.Diagnostics.AddError("Missing Category Writer", "The provider did not configure the category writer.")
		return
	}
	if err := r.writer.DeleteCategory(ctx, state.ID.ValueString()); err != nil && !errors.Is(err, prism.ErrCategoryNotFound) {
		response.Diagnostics.AddError("Unable to delete Nutanix category", "The Prism category delete operation failed.")
	}
}

func (r *resourceImpl) ImportState(
	ctx context.Context,
	request resource.ImportStateRequest,
	response *resource.ImportStateResponse,
) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), request, response)
}

func categorySpec(model resourceModel) prism.CategorySpec {
	return prism.CategorySpec{
		Key:         model.Key.ValueString(),
		Value:       model.Value.ValueString(),
		Description: knownStringPointer(model.Description),
		OwnerUUID:   knownStringPointer(model.OwnerUUID),
	}
}

func knownStringPointer(value types.String) *string {
	if value.IsNull() || value.IsUnknown() {
		return nil
	}
	text := value.ValueString()
	return &text
}

func stateFromRemote(prior resourceModel, category prism.Category) (resourceModel, error) {
	if category.ExtID == nil || category.Key == nil || category.Value == nil {
		return resourceModel{}, prism.ErrInvalidCategory
	}
	prior.ID = types.StringPointerValue(category.ExtID)
	prior.Key = types.StringPointerValue(category.Key)
	prior.Value = types.StringPointerValue(category.Value)
	prior.Description = types.StringPointerValue(category.Description)
	prior.OwnerUUID = types.StringPointerValue(category.OwnerUUID)
	prior.Type = types.StringPointerValue(category.Type)
	return prior, nil
}
