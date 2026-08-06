// Package cluster implements the nutanix_clusters_v2 data source.
package cluster

import (
	"context"
	"net/url"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/ioplane/terraform-provider-nutanix/internal/nutanix/clustermgmt"
	"github.com/ioplane/terraform-provider-nutanix/internal/nutanix/odata"
	"github.com/ioplane/terraform-provider-nutanix/internal/service/listquery"
	"github.com/ioplane/terraform-provider-nutanix/internal/service/queryid"
)

const terraformTypeName = "nutanix_clusters_v2"

var clusterObjectType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"ext_id":                   types.StringType,
	"name":                     types.StringType,
	"categories":               types.ListType{ElemType: types.StringType},
	"vm_count":                 types.Int64Type,
	"inefficient_vm_count":     types.Int64Type,
	"container_name":           types.StringType,
	"cluster_profile_ext_id":   types.StringType,
	"backup_eligibility_score": types.Int64Type,
}}

// Reader is the Cluster Management capability consumed by this data source.
type Reader interface {
	ListClusters(context.Context, odata.ListOptions) ([]clustermgmt.Cluster, url.Values, error)
}

type providerData interface {
	ClusterReader() Reader
}

type dataSource struct {
	reader Reader
}

type dataSourceModel struct {
	Page            types.Int64  `tfsdk:"page"`
	Limit           types.Int64  `tfsdk:"limit"`
	Filter          types.String `tfsdk:"filter"`
	OrderBy         types.String `tfsdk:"order_by"`
	Select          types.String `tfsdk:"select"`
	Expand          types.String `tfsdk:"expand"`
	ID              types.String `tfsdk:"id"`
	ClusterEntities types.List   `tfsdk:"cluster_entities"`
}

type clusterModel struct {
	ExtID                  types.String `tfsdk:"ext_id"`
	Name                   types.String `tfsdk:"name"`
	Categories             types.List   `tfsdk:"categories"`
	VMCount                types.Int64  `tfsdk:"vm_count"`
	InefficientVMCount     types.Int64  `tfsdk:"inefficient_vm_count"`
	ContainerName          types.String `tfsdk:"container_name"`
	ClusterProfileExtID    types.String `tfsdk:"cluster_profile_ext_id"`
	BackupEligibilityScore types.Int64  `tfsdk:"backup_eligibility_score"`
}

var (
	_ datasource.DataSource              = (*dataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*dataSource)(nil)
)

// NewDataSource returns a new nutanix_clusters_v2 data source.
func NewDataSource() datasource.DataSource {
	return &dataSource{}
}

func (*dataSource) Metadata(
	_ context.Context,
	request datasource.MetadataRequest,
	response *datasource.MetadataResponse,
) {
	response.TypeName = request.ProviderTypeName + "_clusters_v2"
}

func (*dataSource) Schema(
	_ context.Context,
	_ datasource.SchemaRequest,
	response *datasource.SchemaResponse,
) {
	response.Schema = schema.Schema{
		Description: "Lists Nutanix clusters through the Cluster Management v4.2 API.",
		Attributes: map[string]schema.Attribute{
			"page":     listquery.PageAttribute(),
			"limit":    listquery.LimitAttribute(),
			"filter":   listquery.StringAttribute("OData filter expression."),
			"order_by": listquery.StringAttribute("OData order-by expression."),
			"select": listquery.StringAttribute(
				"Comma-separated simple properties to request in addition to required state fields.",
			),
			"expand": listquery.StringAttribute("Comma-separated simple relationships to expand."),
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Deterministic identity of the caller-supplied list query.",
			},
			"cluster_entities": schema.ListNestedAttribute{
				Computed:    true,
				Description: "Clusters returned by Nutanix.",
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"ext_id": schema.StringAttribute{Computed: true},
					"name":   schema.StringAttribute{Computed: true},
					"categories": schema.ListAttribute{
						Computed:    true,
						ElementType: types.StringType,
					},
					"vm_count":                 schema.Int64Attribute{Computed: true},
					"inefficient_vm_count":     schema.Int64Attribute{Computed: true},
					"container_name":           schema.StringAttribute{Computed: true},
					"cluster_profile_ext_id":   schema.StringAttribute{Computed: true},
					"backup_eligibility_score": schema.Int64Attribute{Computed: true},
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
			"Unexpected Cluster Data Source Configure Type",
			"The provider supplied incompatible data to the cluster data source.",
		)
		return
	}
	d.reader = configured.ClusterReader()
	if d.reader == nil {
		response.Diagnostics.AddError(
			"Missing Cluster Reader",
			"The provider did not configure the cluster reader.",
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
			"Missing Cluster Reader",
			"The provider did not configure the cluster reader.",
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
	clusters, identity, err := d.reader.ListClusters(ctx, options)
	if err != nil {
		addReadError(&response.Diagnostics)
		return
	}
	id, err := queryid.New(terraformTypeName, identity)
	if err != nil {
		addReadError(&response.Diagnostics)
		return
	}
	state, diagnostics := stateFromClusters(ctx, config, id, clusters)
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
			Title:  "Invalid Nutanix Cluster Query",
			Detail: "The query value must be known and valid before the cluster request can be constructed.",
		},
	)
}

func addReadError(diagnostics *diag.Diagnostics) {
	diagnostics.AddError(
		"Unable to Read Nutanix Clusters",
		"The provider could not read and map the requested Nutanix clusters.",
	)
}

func stateFromClusters(
	ctx context.Context,
	config dataSourceModel,
	id string,
	clusters []clustermgmt.Cluster,
) (dataSourceModel, diag.Diagnostics) {
	var diagnostics diag.Diagnostics
	models := make([]clusterModel, len(clusters))
	for index := range clusters {
		categories := types.ListNull(types.StringType)
		if clusters[index].Categories != nil {
			var current diag.Diagnostics
			categories, current = types.ListValueFrom(
				ctx,
				types.StringType,
				*clusters[index].Categories,
			)
			diagnostics.Append(current...)
		}
		models[index] = clusterModel{
			ExtID:                  types.StringPointerValue(clusters[index].ExtID),
			Name:                   types.StringPointerValue(clusters[index].Name),
			Categories:             categories,
			VMCount:                types.Int64PointerValue(clusters[index].VMCount),
			InefficientVMCount:     types.Int64PointerValue(clusters[index].InefficientVMCount),
			ContainerName:          types.StringPointerValue(clusters[index].ContainerName),
			ClusterProfileExtID:    types.StringPointerValue(clusters[index].ClusterProfileExtID),
			BackupEligibilityScore: types.Int64PointerValue(clusters[index].BackupEligibilityScore),
		}
	}
	entities, current := types.ListValueFrom(ctx, clusterObjectType, models)
	diagnostics.Append(current...)
	config.ID = types.StringValue(id)
	config.ClusterEntities = entities
	return config, diagnostics
}
