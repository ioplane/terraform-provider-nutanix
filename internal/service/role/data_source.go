// Package role implements the provisional nutanix_roles_v2 data source.
package role

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

var roleObjectType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"ext_id":                        types.StringType,
	"tenant_id":                     types.StringType,
	"display_name":                  types.StringType,
	"client_name":                   types.StringType,
	"description":                   types.StringType,
	"operations":                    types.ListType{ElemType: types.StringType},
	"accessible_clients":            types.ListType{ElemType: types.StringType},
	"accessible_entity_types":       types.ListType{ElemType: types.StringType},
	"accessible_clients_count":      types.Int64Type,
	"accessible_entity_types_count": types.Int64Type,
	"assigned_users_count":          types.Int64Type,
	"assigned_user_groups_count":    types.Int64Type,
	"created_time":                  types.StringType,
	"last_updated_time":             types.StringType,
	"created_by":                    types.StringType,
	"is_system_defined":             types.BoolType,
}}

// Reader is the IAM role capability consumed by this data source.
type Reader interface {
	ListRoles(context.Context, odata.ListOptions) ([]iam.Role, url.Values, error)
}

type providerData interface {
	RoleReader() Reader
}

type dataSourceModel struct {
	Page         types.Int64  `tfsdk:"page"`
	Limit        types.Int64  `tfsdk:"limit"`
	Filter       types.String `tfsdk:"filter"`
	OrderBy      types.String `tfsdk:"order_by"`
	Select       types.String `tfsdk:"select"`
	ID           types.String `tfsdk:"id"`
	RoleEntities types.List   `tfsdk:"role_entities"`
}

type roleModel struct {
	ExtID                      types.String `tfsdk:"ext_id"`
	TenantID                   types.String `tfsdk:"tenant_id"`
	DisplayName                types.String `tfsdk:"display_name"`
	ClientName                 types.String `tfsdk:"client_name"`
	Description                types.String `tfsdk:"description"`
	Operations                 types.List   `tfsdk:"operations"`
	AccessibleClients          types.List   `tfsdk:"accessible_clients"`
	AccessibleEntityTypes      types.List   `tfsdk:"accessible_entity_types"`
	AccessibleClientsCount     types.Int64  `tfsdk:"accessible_clients_count"`
	AccessibleEntityTypesCount types.Int64  `tfsdk:"accessible_entity_types_count"`
	AssignedUsersCount         types.Int64  `tfsdk:"assigned_users_count"`
	AssignedUserGroupsCount    types.Int64  `tfsdk:"assigned_user_groups_count"`
	CreatedTime                types.String `tfsdk:"created_time"`
	LastUpdatedTime            types.String `tfsdk:"last_updated_time"`
	CreatedBy                  types.String `tfsdk:"created_by"`
	IsSystemDefined            types.Bool   `tfsdk:"is_system_defined"`
}

// NewDataSource returns a new nutanix_roles_v2 data source.
func NewDataSource() datasource.DataSource {
	return listdata.New(iamDataSourceDescriptor{}.DataSource())
}

type iamDataSourceDescriptor struct{}

func (iamDataSourceDescriptor) Schema() schema.Schema {
	return schema.Schema{
		Description: "Lists Nutanix IAM roles through the provisional IAM v4.0 API surface.",
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
			"role_entities": schema.ListNestedAttribute{
				Computed:    true,
				Description: "IAM roles returned by Nutanix.",
				NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
					"ext_id":                        schema.StringAttribute{Computed: true},
					"tenant_id":                     schema.StringAttribute{Computed: true},
					"display_name":                  schema.StringAttribute{Computed: true},
					"client_name":                   schema.StringAttribute{Computed: true},
					"description":                   schema.StringAttribute{Computed: true},
					"operations":                    schema.ListAttribute{Computed: true, ElementType: types.StringType},
					"accessible_clients":            schema.ListAttribute{Computed: true, ElementType: types.StringType},
					"accessible_entity_types":       schema.ListAttribute{Computed: true, ElementType: types.StringType},
					"accessible_clients_count":      schema.Int64Attribute{Computed: true},
					"accessible_entity_types_count": schema.Int64Attribute{Computed: true},
					"assigned_users_count":          schema.Int64Attribute{Computed: true},
					"assigned_user_groups_count":    schema.Int64Attribute{Computed: true},
					"created_time":                  schema.StringAttribute{Computed: true},
					"last_updated_time":             schema.StringAttribute{Computed: true},
					"created_by":                    schema.StringAttribute{Computed: true},
					"is_system_defined":             schema.BoolAttribute{Computed: true},
				}},
			},
		},
	}
}

func iamDataSourceDescriptorReader(value any) (listdata.ListFunc[iam.Role], bool) {
	configured, ok := value.(providerData)
	if !ok {
		return nil, ok
	}
	reader := configured.RoleReader()
	if reader == nil {
		return nil, true
	}
	return reader.ListRoles, true
}

func (iamDataSourceDescriptor) DataSource() listdata.Descriptor[iam.Role, dataSourceModel] {
	return listdata.Descriptor[iam.Role, dataSourceModel]{
		TypeName:            "roles_v2",
		IdentityTypeName:    "nutanix_roles_v2",
		Schema:              iamDataSourceDescriptor{}.Schema(),
		Reader:              iamDataSourceDescriptorReader,
		Query:               queryValues,
		SetState:            setStateFromRoles,
		ConfigureTitle:      "Unexpected IAM Role Data Source Configure Type",
		ConfigureDetail:     "The provider supplied incompatible data to the IAM role data source.",
		MissingReaderTitle:  "Missing IAM Role Reader",
		MissingReaderDetail: "The provider did not configure the IAM role reader.",
		ReadErrorTitle:      "Unable to Read Nutanix IAM Roles",
		ReadErrorDetail:     "The provider could not read and map the requested Nutanix IAM roles.",
	}
}

func setStateFromRoles(
	ctx context.Context,
	config *dataSourceModel,
	id string,
	roles []iam.Role,
) diag.Diagnostics {
	state, diagnostics := stateFromRoles(ctx, *config, id, roles)
	*config = state
	return diagnostics
}

func queryValues(model dataSourceModel) listquery.Values {
	return listquery.Values{Page: model.Page, Limit: model.Limit, Filter: model.Filter, OrderBy: model.OrderBy, Select: model.Select}
}

func stateFromRoles(
	ctx context.Context,
	config dataSourceModel,
	id string,
	roles []iam.Role,
) (dataSourceModel, diag.Diagnostics) {
	var diagnostics diag.Diagnostics
	models := make([]roleModel, len(roles))
	for index := range roles {
		operations, current := stringListValue(ctx, roles[index].Operations)
		diagnostics.Append(current...)
		accessibleClients, current := stringListValue(ctx, roles[index].AccessibleClients)
		diagnostics.Append(current...)
		accessibleEntityTypes, current := stringListValue(ctx, roles[index].AccessibleEntityTypes)
		diagnostics.Append(current...)
		models[index] = roleModel{
			ExtID:                      types.StringPointerValue(roles[index].ExtID),
			TenantID:                   types.StringPointerValue(roles[index].TenantID),
			DisplayName:                types.StringPointerValue(roles[index].DisplayName),
			ClientName:                 types.StringPointerValue(roles[index].ClientName),
			Description:                types.StringPointerValue(roles[index].Description),
			Operations:                 operations,
			AccessibleClients:          accessibleClients,
			AccessibleEntityTypes:      accessibleEntityTypes,
			AccessibleClientsCount:     types.Int64PointerValue(roles[index].AccessibleClientsCount),
			AccessibleEntityTypesCount: types.Int64PointerValue(roles[index].AccessibleEntityTypesCount),
			AssignedUsersCount:         types.Int64PointerValue(roles[index].AssignedUsersCount),
			AssignedUserGroupsCount:    types.Int64PointerValue(roles[index].AssignedUserGroupsCount),
			CreatedTime:                types.StringPointerValue(roles[index].CreatedTime),
			LastUpdatedTime:            types.StringPointerValue(roles[index].LastUpdatedTime),
			CreatedBy:                  types.StringPointerValue(roles[index].CreatedBy),
			IsSystemDefined:            types.BoolPointerValue(roles[index].IsSystemDefined),
		}
	}
	entities, current := types.ListValueFrom(ctx, roleObjectType, models)
	diagnostics.Append(current...)
	config.ID = types.StringValue(id)
	config.RoleEntities = entities
	return config, diagnostics
}

func stringListValue(ctx context.Context, values *[]string) (types.List, diag.Diagnostics) {
	if values == nil {
		return types.ListNull(types.StringType), nil
	}
	return types.ListValueFrom(ctx, types.StringType, *values)
}
