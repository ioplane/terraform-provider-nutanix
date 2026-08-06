// Package operation implements the provisional nutanix_operations_v2 data source.
package operation

import (
	"context"
	"net/url"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/ioplane/terraform-provider-nutanix/internal/nutanix/iam"
	"github.com/ioplane/terraform-provider-nutanix/internal/nutanix/odata"
	"github.com/ioplane/terraform-provider-nutanix/internal/service/listdata"
	"github.com/ioplane/terraform-provider-nutanix/internal/service/listquery"
)

var (
	endpointObjectType = types.ObjectType{AttrTypes: map[string]attr.Type{
		"api_version":  types.StringType,
		"endpoint_url": types.StringType,
		"http_method":  types.StringType,
	}}
	operationObjectType = types.ObjectType{AttrTypes: map[string]attr.Type{
		"ext_id":                   types.StringType,
		"tenant_id":                types.StringType,
		"display_name":             types.StringType,
		"description":              types.StringType,
		"entity_type":              types.StringType,
		"client_name":              types.StringType,
		"created_time":             types.StringType,
		"last_updated_time":        types.StringType,
		"operation_type":           types.StringType,
		"related_operation_list":   types.ListType{ElemType: types.StringType},
		"associated_endpoint_list": types.ListType{ElemType: endpointObjectType},
	}}
)

// Reader is the IAM operation capability consumed by this data source.
type Reader interface {
	ListOperations(context.Context, odata.ListOptions) ([]iam.Operation, url.Values, error)
}

type providerData interface {
	OperationReader() Reader
}

type dataSourceModel struct {
	Page              types.Int64  `tfsdk:"page"`
	Limit             types.Int64  `tfsdk:"limit"`
	Filter            types.String `tfsdk:"filter"`
	OrderBy           types.String `tfsdk:"order_by"`
	Select            types.String `tfsdk:"select"`
	ID                types.String `tfsdk:"id"`
	OperationEntities types.List   `tfsdk:"operation_entities"`
}

type endpointModel struct {
	APIVersion  types.String `tfsdk:"api_version"`
	EndpointURL types.String `tfsdk:"endpoint_url"`
	HTTPMethod  types.String `tfsdk:"http_method"`
}

type operationModel struct {
	ExtID                  types.String `tfsdk:"ext_id"`
	TenantID               types.String `tfsdk:"tenant_id"`
	DisplayName            types.String `tfsdk:"display_name"`
	Description            types.String `tfsdk:"description"`
	EntityType             types.String `tfsdk:"entity_type"`
	ClientName             types.String `tfsdk:"client_name"`
	CreatedTime            types.String `tfsdk:"created_time"`
	LastUpdatedTime        types.String `tfsdk:"last_updated_time"`
	OperationType          types.String `tfsdk:"operation_type"`
	RelatedOperationList   types.List   `tfsdk:"related_operation_list"`
	AssociatedEndpointList types.List   `tfsdk:"associated_endpoint_list"`
}

// NewDataSource returns a new nutanix_operations_v2 data source.
func NewDataSource() datasource.DataSource {
	return listdata.New(iamDataSourceDescriptor{}.DataSource())
}

type iamDataSourceDescriptor struct{}

func (iamDataSourceDescriptor) Schema() schema.Schema {
	return schema.Schema{
		Description: "Lists Nutanix IAM operations through the provisional IAM v4.0 API surface.",
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
			"operation_entities": schema.ListNestedAttribute{
				Computed:    true,
				Description: "IAM operations returned by Nutanix.",
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"ext_id":                 schema.StringAttribute{Computed: true},
					"tenant_id":              schema.StringAttribute{Computed: true},
					"display_name":           schema.StringAttribute{Computed: true},
					"description":            schema.StringAttribute{Computed: true},
					"entity_type":            schema.StringAttribute{Computed: true},
					"client_name":            schema.StringAttribute{Computed: true},
					"created_time":           schema.StringAttribute{Computed: true},
					"last_updated_time":      schema.StringAttribute{Computed: true},
					"operation_type":         schema.StringAttribute{Computed: true},
					"related_operation_list": schema.ListAttribute{Computed: true, ElementType: types.StringType},
					"associated_endpoint_list": schema.ListNestedAttribute{
						Computed: true,
						NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
							"api_version":  schema.StringAttribute{Computed: true},
							"endpoint_url": schema.StringAttribute{Computed: true},
							"http_method":  schema.StringAttribute{Computed: true},
						}},
					},
				}},
			},
		},
	}
}

func iamDataSourceDescriptorReader(value any) (listdata.ListFunc[iam.Operation], bool) {
	configured, ok := value.(providerData)
	if !ok {
		return nil, ok
	}
	reader := configured.OperationReader()
	if reader == nil {
		return nil, true
	}
	return reader.ListOperations, true
}

func (iamDataSourceDescriptor) DataSource() listdata.Descriptor[iam.Operation, dataSourceModel] {
	return listdata.Descriptor[iam.Operation, dataSourceModel]{
		TypeName:            "operations_v2",
		IdentityTypeName:    "nutanix_operations_v2",
		Schema:              iamDataSourceDescriptor{}.Schema(),
		Reader:              iamDataSourceDescriptorReader,
		Query:               queryValues,
		SetState:            setStateFromOperations,
		ConfigureTitle:      "Unexpected IAM Operation Data Source Configure Type",
		ConfigureDetail:     "The provider supplied incompatible data to the IAM operation data source.",
		MissingReaderTitle:  "Missing IAM Operation Reader",
		MissingReaderDetail: "The provider did not configure the IAM operation reader.",
		ReadErrorTitle:      "Unable to Read Nutanix IAM Operations",
		ReadErrorDetail:     "The provider could not read and map the requested Nutanix IAM operations.",
	}
}

func setStateFromOperations(
	ctx context.Context,
	config *dataSourceModel,
	id string,
	operations []iam.Operation,
) diag.Diagnostics {
	state, diagnostics := stateFromOperations(ctx, *config, id, operations)
	*config = state
	return diagnostics
}

func queryValues(model dataSourceModel) listquery.Values {
	return listquery.Values{Page: model.Page, Limit: model.Limit, Filter: model.Filter, OrderBy: model.OrderBy, Select: model.Select}
}

func stateFromOperations(
	ctx context.Context,
	config dataSourceModel,
	id string,
	operations []iam.Operation,
) (dataSourceModel, diag.Diagnostics) {
	var diagnostics diag.Diagnostics
	models := make([]operationModel, len(operations))
	for index := range operations {
		related, current := stringListValue(ctx, operations[index].RelatedOperationList)
		diagnostics.Append(current...)
		endpoints, current := endpointListValue(ctx, operations[index].AssociatedEndpointList)
		diagnostics.Append(current...)
		models[index] = operationModel{
			ExtID:                  types.StringPointerValue(operations[index].ExtID),
			TenantID:               types.StringPointerValue(operations[index].TenantID),
			DisplayName:            types.StringPointerValue(operations[index].DisplayName),
			Description:            types.StringPointerValue(operations[index].Description),
			EntityType:             types.StringPointerValue(operations[index].EntityType),
			ClientName:             types.StringPointerValue(operations[index].ClientName),
			CreatedTime:            types.StringPointerValue(operations[index].CreatedTime),
			LastUpdatedTime:        types.StringPointerValue(operations[index].LastUpdatedTime),
			OperationType:          types.StringPointerValue(operations[index].OperationType),
			RelatedOperationList:   related,
			AssociatedEndpointList: endpoints,
		}
	}
	entities, current := types.ListValueFrom(ctx, operationObjectType, models)
	diagnostics.Append(current...)
	config.ID = types.StringValue(id)
	config.OperationEntities = entities
	return config, diagnostics
}

func stringListValue(ctx context.Context, values *[]string) (types.List, diag.Diagnostics) {
	if values == nil {
		return types.ListNull(types.StringType), nil
	}
	return types.ListValueFrom(ctx, types.StringType, *values)
}

func endpointListValue(
	ctx context.Context,
	values *[]iam.OperationEndpoint,
) (types.List, diag.Diagnostics) {
	if values == nil {
		return types.ListNull(endpointObjectType), nil
	}
	models := make([]endpointModel, len(*values))
	for index := range *values {
		models[index] = endpointModel{
			APIVersion:  types.StringPointerValue((*values)[index].APIVersion),
			EndpointURL: types.StringPointerValue((*values)[index].EndpointURL),
			HTTPMethod:  types.StringPointerValue((*values)[index].HTTPMethod),
		}
	}
	return types.ListValueFrom(ctx, endpointObjectType, models)
}
