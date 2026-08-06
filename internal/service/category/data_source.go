// Package category implements the nutanix_categories_v2 data source.
package category

import (
	"context"
	"net/url"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/ioplane/terraform-provider-nutanix/internal/nutanix/odata"
	"github.com/ioplane/terraform-provider-nutanix/internal/nutanix/prism"
	"github.com/ioplane/terraform-provider-nutanix/internal/service/listquery"
	"github.com/ioplane/terraform-provider-nutanix/internal/service/queryid"
)

const terraformTypeName = "nutanix_categories_v2"

var (
	associationSummaryObjectType = types.ObjectType{AttrTypes: map[string]attr.Type{
		"category_id":    types.StringType,
		"resource_type":  types.StringType,
		"resource_group": types.StringType,
		"count":          types.Int64Type,
	}}
	associationDetailObjectType = types.ObjectType{AttrTypes: map[string]attr.Type{
		"category_id":    types.StringType,
		"resource_type":  types.StringType,
		"resource_group": types.StringType,
		"resource_id":    types.StringType,
	}}
	categoryObjectType = types.ObjectType{AttrTypes: map[string]attr.Type{
		"ext_id":                types.StringType,
		"key":                   types.StringType,
		"value":                 types.StringType,
		"type":                  types.StringType,
		"description":           types.StringType,
		"owner_uuid":            types.StringType,
		"associations":          types.ListType{ElemType: associationSummaryObjectType},
		"detailed_associations": types.ListType{ElemType: associationDetailObjectType},
	}}
)

// Reader is the Prism category capability consumed by this data source.
type Reader interface {
	ListCategories(context.Context, odata.ListOptions) ([]prism.Category, url.Values, error)
}

type providerData interface {
	CategoryReader() Reader
}

type dataSource struct {
	reader Reader
}

type dataSourceModel struct {
	Page       types.Int64  `tfsdk:"page"`
	Limit      types.Int64  `tfsdk:"limit"`
	Filter     types.String `tfsdk:"filter"`
	OrderBy    types.String `tfsdk:"order_by"`
	Select     types.String `tfsdk:"select"`
	Expand     types.String `tfsdk:"expand"`
	ID         types.String `tfsdk:"id"`
	Categories types.List   `tfsdk:"categories"`
}

type categoryModel struct {
	ExtID                types.String `tfsdk:"ext_id"`
	Key                  types.String `tfsdk:"key"`
	Value                types.String `tfsdk:"value"`
	Type                 types.String `tfsdk:"type"`
	Description          types.String `tfsdk:"description"`
	OwnerUUID            types.String `tfsdk:"owner_uuid"`
	Associations         types.List   `tfsdk:"associations"`
	DetailedAssociations types.List   `tfsdk:"detailed_associations"`
}

type associationSummaryModel struct {
	CategoryID    types.String `tfsdk:"category_id"`
	ResourceType  types.String `tfsdk:"resource_type"`
	ResourceGroup types.String `tfsdk:"resource_group"`
	Count         types.Int64  `tfsdk:"count"`
}

type associationDetailModel struct {
	CategoryID    types.String `tfsdk:"category_id"`
	ResourceType  types.String `tfsdk:"resource_type"`
	ResourceGroup types.String `tfsdk:"resource_group"`
	ResourceID    types.String `tfsdk:"resource_id"`
}

var (
	_ datasource.DataSource              = (*dataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*dataSource)(nil)
)

// NewDataSource returns a new nutanix_categories_v2 data source.
func NewDataSource() datasource.DataSource {
	return &dataSource{}
}

func (*dataSource) Metadata(
	_ context.Context,
	request datasource.MetadataRequest,
	response *datasource.MetadataResponse,
) {
	response.TypeName = request.ProviderTypeName + "_categories_v2"
}

func (*dataSource) Schema(
	_ context.Context,
	_ datasource.SchemaRequest,
	response *datasource.SchemaResponse,
) {
	response.Schema = schema.Schema{
		Description: "Lists Nutanix categories through the Prism v4.3 API.",
		Attributes: map[string]schema.Attribute{
			"page":     listquery.PageAttribute(),
			"limit":    listquery.LimitAttribute(),
			"filter":   listquery.StringAttribute("OData filter expression."),
			"order_by": listquery.StringAttribute("OData order-by expression."),
			"select": listquery.StringAttribute(
				"Comma-separated simple properties to request in addition to required state fields.",
			),
			"expand": listquery.StringAttribute(
				"Comma-separated simple relationships to expand in addition to required associations.",
			),
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Deterministic identity of the caller-supplied list query.",
			},
			"categories": schema.ListNestedAttribute{
				Computed:    true,
				Description: "Categories returned by Nutanix.",
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"ext_id":      schema.StringAttribute{Computed: true},
					"key":         schema.StringAttribute{Computed: true},
					"value":       schema.StringAttribute{Computed: true},
					"type":        schema.StringAttribute{Computed: true},
					"description": schema.StringAttribute{Computed: true},
					"owner_uuid":  schema.StringAttribute{Computed: true},
					"associations": schema.ListNestedAttribute{
						Computed: true,
						NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
							"category_id":    schema.StringAttribute{Computed: true},
							"resource_type":  schema.StringAttribute{Computed: true},
							"resource_group": schema.StringAttribute{Computed: true},
							"count":          schema.Int64Attribute{Computed: true},
						}},
					},
					"detailed_associations": schema.ListNestedAttribute{
						Computed: true,
						NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
							"category_id":    schema.StringAttribute{Computed: true},
							"resource_type":  schema.StringAttribute{Computed: true},
							"resource_group": schema.StringAttribute{Computed: true},
							"resource_id":    schema.StringAttribute{Computed: true},
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
			"Unexpected Category Data Source Configure Type",
			"The provider supplied incompatible data to the category data source.",
		)
		return
	}
	d.reader = configured.CategoryReader()
	if d.reader == nil {
		response.Diagnostics.AddError(
			"Missing Category Reader",
			"The provider did not configure the category reader.",
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
			"Missing Category Reader",
			"The provider did not configure the category reader.",
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
	categories, identity, err := d.reader.ListCategories(ctx, options)
	if err != nil {
		addReadError(&response.Diagnostics)
		return
	}
	id, err := queryid.New(terraformTypeName, identity)
	if err != nil {
		addReadError(&response.Diagnostics)
		return
	}
	state, diagnostics := stateFromCategories(ctx, config, id, categories)
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
			Expand:  model.Expand,
		},
		listquery.DiagnosticText{
			Title:  "Invalid Nutanix Category Query",
			Detail: "The query value must be known and valid before the category request can be constructed.",
		},
	)
}

func addReadError(diagnostics *diag.Diagnostics) {
	diagnostics.AddError(
		"Unable to Read Nutanix Categories",
		"The provider could not read and map the requested Nutanix categories.",
	)
}

func stateFromCategories(
	ctx context.Context,
	config dataSourceModel,
	id string,
	categories []prism.Category,
) (dataSourceModel, diag.Diagnostics) {
	var diagnostics diag.Diagnostics
	models := make([]categoryModel, len(categories))
	for index := range categories {
		associations, current := associationSummaries(ctx, categories[index].Associations)
		diagnostics.Append(current...)
		details, current := associationDetails(ctx, categories[index].DetailedAssociations)
		diagnostics.Append(current...)
		models[index] = categoryModel{
			ExtID:                types.StringPointerValue(categories[index].ExtID),
			Key:                  types.StringPointerValue(categories[index].Key),
			Value:                types.StringPointerValue(categories[index].Value),
			Type:                 types.StringPointerValue(categories[index].Type),
			Description:          types.StringPointerValue(categories[index].Description),
			OwnerUUID:            types.StringPointerValue(categories[index].OwnerUUID),
			Associations:         associations,
			DetailedAssociations: details,
		}
	}
	values, current := types.ListValueFrom(ctx, categoryObjectType, models)
	diagnostics.Append(current...)
	config.ID = types.StringValue(id)
	config.Categories = values
	return config, diagnostics
}

func associationSummaries(
	ctx context.Context,
	associations *[]prism.AssociationSummary,
) (types.List, diag.Diagnostics) {
	if associations == nil {
		return types.ListNull(associationSummaryObjectType), nil
	}
	models := make([]associationSummaryModel, len(*associations))
	for index := range *associations {
		models[index] = associationSummaryModel{
			CategoryID:    types.StringPointerValue((*associations)[index].CategoryID),
			ResourceType:  types.StringPointerValue((*associations)[index].ResourceType),
			ResourceGroup: types.StringPointerValue((*associations)[index].ResourceGroup),
			Count:         types.Int64PointerValue((*associations)[index].Count),
		}
	}
	return types.ListValueFrom(ctx, associationSummaryObjectType, models)
}

func associationDetails(
	ctx context.Context,
	associations *[]prism.AssociationDetail,
) (types.List, diag.Diagnostics) {
	if associations == nil {
		return types.ListNull(associationDetailObjectType), nil
	}
	models := make([]associationDetailModel, len(*associations))
	for index := range *associations {
		models[index] = associationDetailModel{
			CategoryID:    types.StringPointerValue((*associations)[index].CategoryID),
			ResourceType:  types.StringPointerValue((*associations)[index].ResourceType),
			ResourceGroup: types.StringPointerValue((*associations)[index].ResourceGroup),
			ResourceID:    types.StringPointerValue((*associations)[index].ResourceID),
		}
	}
	return types.ListValueFrom(ctx, associationDetailObjectType, models)
}
