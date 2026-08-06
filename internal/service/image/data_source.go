// Package image implements the nutanix_images_v2 data source.
package image

import (
	"context"
	"net/url"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/ioplane/terraform-provider-nutanix/internal/nutanix/odata"
	"github.com/ioplane/terraform-provider-nutanix/internal/nutanix/vmm"
	"github.com/ioplane/terraform-provider-nutanix/internal/service/listquery"
	"github.com/ioplane/terraform-provider-nutanix/internal/service/queryid"
)

const terraformTypeName = "nutanix_images_v2"

var (
	checksumObjectType = types.ObjectType{AttrTypes: map[string]attr.Type{
		"hex_digest": types.StringType,
	}}
	placementStatusObjectType = types.ObjectType{AttrTypes: map[string]attr.Type{
		"placement_policy_ext_id":    types.StringType,
		"compliance_status":          types.StringType,
		"enforcement_mode":           types.StringType,
		"policy_cluster_ext_ids":     types.ListType{ElemType: types.StringType},
		"enforced_cluster_ext_ids":   types.ListType{ElemType: types.StringType},
		"conflicting_policy_ext_ids": types.ListType{ElemType: types.StringType},
	}}
	imageObjectType = types.ObjectType{AttrTypes: map[string]attr.Type{
		"ext_id":                   types.StringType,
		"name":                     types.StringType,
		"description":              types.StringType,
		"type":                     types.StringType,
		"checksum":                 types.ListType{ElemType: checksumObjectType},
		"size_bytes":               types.Int64Type,
		"category_ext_ids":         types.ListType{ElemType: types.StringType},
		"cluster_location_ext_ids": types.ListType{ElemType: types.StringType},
		"create_time":              types.StringType,
		"last_update_time":         types.StringType,
		"owner_ext_id":             types.StringType,
		"owner_name":               types.StringType,
		"placement_policy_status":  types.ListType{ElemType: placementStatusObjectType},
	}}
)

// Reader is the VMM image capability consumed by this data source.
type Reader interface {
	ListImages(context.Context, odata.ListOptions) ([]vmm.Image, url.Values, error)
}

type providerData interface {
	ImageReader() Reader
}

type dataSource struct {
	reader Reader
}

type dataSourceModel struct {
	Page    types.Int64  `tfsdk:"page"`
	Limit   types.Int64  `tfsdk:"limit"`
	Filter  types.String `tfsdk:"filter"`
	OrderBy types.String `tfsdk:"order_by"`
	Select  types.String `tfsdk:"select"`
	ID      types.String `tfsdk:"id"`
	Images  types.List   `tfsdk:"images"`
}

type imageModel struct {
	ExtID                 types.String `tfsdk:"ext_id"`
	Name                  types.String `tfsdk:"name"`
	Description           types.String `tfsdk:"description"`
	Type                  types.String `tfsdk:"type"`
	Checksum              types.List   `tfsdk:"checksum"`
	SizeBytes             types.Int64  `tfsdk:"size_bytes"`
	CategoryExtIDs        types.List   `tfsdk:"category_ext_ids"`
	ClusterLocationExtIDs types.List   `tfsdk:"cluster_location_ext_ids"`
	CreateTime            types.String `tfsdk:"create_time"`
	LastUpdateTime        types.String `tfsdk:"last_update_time"`
	OwnerExtID            types.String `tfsdk:"owner_ext_id"`
	OwnerName             types.String `tfsdk:"owner_name"`
	PlacementPolicyStatus types.List   `tfsdk:"placement_policy_status"`
}

type checksumModel struct {
	HexDigest types.String `tfsdk:"hex_digest"`
}

type placementStatusModel struct {
	PlacementPolicyExtID    types.String `tfsdk:"placement_policy_ext_id"`
	ComplianceStatus        types.String `tfsdk:"compliance_status"`
	EnforcementMode         types.String `tfsdk:"enforcement_mode"`
	PolicyClusterExtIDs     types.List   `tfsdk:"policy_cluster_ext_ids"`
	EnforcedClusterExtIDs   types.List   `tfsdk:"enforced_cluster_ext_ids"`
	ConflictingPolicyExtIDs types.List   `tfsdk:"conflicting_policy_ext_ids"`
}

var (
	_ datasource.DataSource              = (*dataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*dataSource)(nil)
)

// NewDataSource returns a new nutanix_images_v2 data source.
func NewDataSource() datasource.DataSource {
	return &dataSource{}
}

func (*dataSource) Metadata(
	_ context.Context,
	request datasource.MetadataRequest,
	response *datasource.MetadataResponse,
) {
	response.TypeName = request.ProviderTypeName + "_images_v2"
}

func (*dataSource) Schema(
	_ context.Context,
	_ datasource.SchemaRequest,
	response *datasource.SchemaResponse,
) {
	response.Schema = schema.Schema{
		Description: "Lists Nutanix images through the VMM v4.2 API.",
		Attributes: map[string]schema.Attribute{
			"page":     listquery.PageAttribute(),
			"limit":    listquery.LimitAttribute(),
			"filter":   listquery.StringAttribute("OData filter expression."),
			"order_by": listquery.StringAttribute("OData order-by expression."),
			"select": listquery.StringAttribute(
				"Comma-separated simple properties to request in addition to required state fields.",
			),
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Deterministic identity of the caller-supplied list query.",
			},
			"images": schema.ListNestedAttribute{
				Computed:    true,
				Description: "Images returned by Nutanix.",
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"ext_id":      schema.StringAttribute{Computed: true},
					"name":        schema.StringAttribute{Computed: true},
					"description": schema.StringAttribute{Computed: true},
					"type":        schema.StringAttribute{Computed: true},
					"checksum": schema.ListNestedAttribute{
						Computed: true,
						NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
							"hex_digest": schema.StringAttribute{Computed: true},
						}},
					},
					"size_bytes": schema.Int64Attribute{Computed: true},
					"category_ext_ids": schema.ListAttribute{
						Computed:    true,
						ElementType: types.StringType,
					},
					"cluster_location_ext_ids": schema.ListAttribute{
						Computed:    true,
						ElementType: types.StringType,
					},
					"create_time":      schema.StringAttribute{Computed: true},
					"last_update_time": schema.StringAttribute{Computed: true},
					"owner_ext_id":     schema.StringAttribute{Computed: true},
					"owner_name":       schema.StringAttribute{Computed: true},
					"placement_policy_status": schema.ListNestedAttribute{
						Computed: true,
						NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
							"placement_policy_ext_id": schema.StringAttribute{Computed: true},
							"compliance_status":       schema.StringAttribute{Computed: true},
							"enforcement_mode":        schema.StringAttribute{Computed: true},
							"policy_cluster_ext_ids": schema.ListAttribute{
								Computed:    true,
								ElementType: types.StringType,
							},
							"enforced_cluster_ext_ids": schema.ListAttribute{
								Computed:    true,
								ElementType: types.StringType,
							},
							"conflicting_policy_ext_ids": schema.ListAttribute{
								Computed:    true,
								ElementType: types.StringType,
							},
						}},
					},
				}},
			},
		},
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
			"Unexpected Image Data Source Configure Type",
			"The provider supplied incompatible data to the image data source.",
		)
		return
	}
	d.reader = configured.ImageReader()
	if d.reader == nil {
		response.Diagnostics.AddError(
			"Missing Image Reader",
			"The provider did not configure the image reader.",
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
			"Missing Image Reader",
			"The provider did not configure the image reader.",
		)
		return
	}
	var config dataSourceModel
	response.Diagnostics.Append(request.Config.Get(ctx, &config)...)
	if response.Diagnostics.HasError() {
		return
	}
	options, diagnostics := optionsFromModel(config)
	response.Diagnostics.Append(diagnostics...)
	if response.Diagnostics.HasError() {
		return
	}
	images, identity, err := d.reader.ListImages(ctx, options)
	if err != nil {
		addReadError(&response.Diagnostics)
		return
	}
	id, err := queryid.New(terraformTypeName, identity)
	if err != nil {
		addReadError(&response.Diagnostics)
		return
	}
	state, diagnostics := stateFromImages(ctx, config, id, images)
	if len(diagnostics) != 0 {
		addReadError(&response.Diagnostics)
		return
	}
	response.Diagnostics.Append(response.State.Set(ctx, &state)...)
}

func optionsFromModel(model dataSourceModel) (odata.ListOptions, diag.Diagnostics) {
	return listquery.Options(
		listquery.Values{
			Page:    model.Page,
			Limit:   model.Limit,
			Filter:  model.Filter,
			OrderBy: model.OrderBy,
			Select:  model.Select,
			Expand:  types.StringNull(),
		},
		listquery.DiagnosticText{
			Title:  "Invalid Nutanix Image Query",
			Detail: "The query value must be known and valid before the image request can be constructed.",
		},
	)
}

func addReadError(diagnostics *diag.Diagnostics) {
	diagnostics.AddError(
		"Unable to Read Nutanix Images",
		"The provider could not read and map the requested Nutanix images.",
	)
}

func stateFromImages(
	ctx context.Context,
	config dataSourceModel,
	id string,
	images []vmm.Image,
) (dataSourceModel, diag.Diagnostics) {
	var diagnostics diag.Diagnostics
	models := make([]imageModel, len(images))
	for index := range images {
		checksum, current := checksumValue(ctx, images[index].Checksum)
		diagnostics.Append(current...)
		categoryIDs, current := stringList(ctx, images[index].CategoryExtIDs)
		diagnostics.Append(current...)
		clusterIDs, current := stringList(ctx, images[index].ClusterLocationExtIDs)
		diagnostics.Append(current...)
		placement, current := placementStatuses(ctx, images[index].PlacementPolicyStatus)
		diagnostics.Append(current...)
		models[index] = imageModel{
			ExtID:                 types.StringPointerValue(images[index].ExtID),
			Name:                  types.StringPointerValue(images[index].Name),
			Description:           types.StringPointerValue(images[index].Description),
			Type:                  types.StringPointerValue(images[index].Type),
			Checksum:              checksum,
			SizeBytes:             types.Int64PointerValue(images[index].SizeBytes),
			CategoryExtIDs:        categoryIDs,
			ClusterLocationExtIDs: clusterIDs,
			CreateTime:            types.StringPointerValue(images[index].CreateTime),
			LastUpdateTime:        types.StringPointerValue(images[index].LastUpdateTime),
			OwnerExtID:            types.StringPointerValue(images[index].OwnerExtID),
			OwnerName:             types.StringPointerValue(images[index].OwnerName),
			PlacementPolicyStatus: placement,
		}
	}
	values, current := types.ListValueFrom(ctx, imageObjectType, models)
	diagnostics.Append(current...)
	config.ID = types.StringValue(id)
	config.Images = values
	return config, diagnostics
}

func checksumValue(ctx context.Context, checksum *vmm.Checksum) (types.List, diag.Diagnostics) {
	if checksum == nil {
		return types.ListNull(checksumObjectType), nil
	}
	return types.ListValueFrom(ctx, checksumObjectType, []checksumModel{{
		HexDigest: types.StringPointerValue(checksum.HexDigest),
	}})
}

func placementStatuses(
	ctx context.Context,
	statuses *[]vmm.ImagePlacementStatus,
) (types.List, diag.Diagnostics) {
	if statuses == nil {
		return types.ListNull(placementStatusObjectType), nil
	}
	var diagnostics diag.Diagnostics
	models := make([]placementStatusModel, len(*statuses))
	for index := range *statuses {
		policyIDs, current := stringList(ctx, (*statuses)[index].PolicyClusterExtIDs)
		diagnostics.Append(current...)
		enforcedIDs, current := stringList(ctx, (*statuses)[index].EnforcedClusterExtIDs)
		diagnostics.Append(current...)
		conflictingIDs, current := stringList(ctx, (*statuses)[index].ConflictingPolicyExtIDs)
		diagnostics.Append(current...)
		models[index] = placementStatusModel{
			PlacementPolicyExtID:    types.StringPointerValue((*statuses)[index].PlacementPolicyExtID),
			ComplianceStatus:        types.StringPointerValue((*statuses)[index].ComplianceStatus),
			EnforcementMode:         types.StringPointerValue((*statuses)[index].EnforcementMode),
			PolicyClusterExtIDs:     policyIDs,
			EnforcedClusterExtIDs:   enforcedIDs,
			ConflictingPolicyExtIDs: conflictingIDs,
		}
	}
	values, current := types.ListValueFrom(ctx, placementStatusObjectType, models)
	diagnostics.Append(current...)
	return values, diagnostics
}

func stringList(ctx context.Context, values *[]string) (types.List, diag.Diagnostics) {
	if values == nil {
		return types.ListNull(types.StringType), nil
	}
	return types.ListValueFrom(ctx, types.StringType, *values)
}
