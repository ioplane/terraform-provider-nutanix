// Package licensekey implements the nutanix_license_keys_v2 data source.
package licensekey

import (
	"context"
	"net/url"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/ioplane/terraform-provider-nutanix/internal/nutanix/licensing"
	"github.com/ioplane/terraform-provider-nutanix/internal/nutanix/odata"
	"github.com/ioplane/terraform-provider-nutanix/internal/service/listdata"
	"github.com/ioplane/terraform-provider-nutanix/internal/service/listquery"
)

var licenseKeyMappingObjectType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"key":            types.StringType,
	"quantity_used":  types.Float64Type,
	"cluster_ext_id": types.StringType,
}}

var licenseKeyAssociationObjectType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"base_key":         types.StringType,
	"associated_key":   types.StringType,
	"association_type": types.StringType,
	"reclaim_type":     types.StringType,
}}

var licenseKeyObjectType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"ext_id":                  types.StringType,
	"tenant_id":               types.StringType,
	"key":                     types.StringType,
	"validation_detail":       types.StringType,
	"type":                    types.StringType,
	"category":                types.StringType,
	"sub_category":            types.StringType,
	"entitlement_expiry_date": types.StringType,
	"meter":                   types.StringType,
	"quantity":                types.Float64Type,
	"group_id":                types.StringType,
	"enforcement_policy":      types.StringType,
	"assignment_details":      types.ListType{ElemType: licenseKeyMappingObjectType},
	"association_details":     types.ListType{ElemType: licenseKeyAssociationObjectType},
}}

// Reader is the Licensing capability consumed by this data source.
type Reader interface {
	ListLicenseKeys(context.Context, odata.ListOptions) ([]licensing.LicenseKey, url.Values, error)
}

type providerData interface {
	LicenseKeyReader() Reader
}

type dataSourceModel struct {
	Page               types.Int64  `tfsdk:"page"`
	Limit              types.Int64  `tfsdk:"limit"`
	Filter             types.String `tfsdk:"filter"`
	OrderBy            types.String `tfsdk:"order_by"`
	Select             types.String `tfsdk:"select"`
	Expand             types.String `tfsdk:"expand"`
	ID                 types.String `tfsdk:"id"`
	LicenseKeyEntities types.List   `tfsdk:"license_key_entities"`
}

type mappingModel struct {
	Key          types.String  `tfsdk:"key"`
	QuantityUsed types.Float64 `tfsdk:"quantity_used"`
	ClusterExtID types.String  `tfsdk:"cluster_ext_id"`
}

type associationModel struct {
	BaseKey         types.String `tfsdk:"base_key"`
	AssociatedKey   types.String `tfsdk:"associated_key"`
	AssociationType types.String `tfsdk:"association_type"`
	ReclaimType     types.String `tfsdk:"reclaim_type"`
}

type licenseKeyModel struct {
	ExtID                 types.String  `tfsdk:"ext_id"`
	TenantID              types.String  `tfsdk:"tenant_id"`
	Key                   types.String  `tfsdk:"key"`
	ValidationDetail      types.String  `tfsdk:"validation_detail"`
	Type                  types.String  `tfsdk:"type"`
	Category              types.String  `tfsdk:"category"`
	SubCategory           types.String  `tfsdk:"sub_category"`
	EntitlementExpiryDate types.String  `tfsdk:"entitlement_expiry_date"`
	Meter                 types.String  `tfsdk:"meter"`
	Quantity              types.Float64 `tfsdk:"quantity"`
	GroupID               types.String  `tfsdk:"group_id"`
	EnforcementPolicy     types.String  `tfsdk:"enforcement_policy"`
	AssignmentDetails     types.List    `tfsdk:"assignment_details"`
	AssociationDetails    types.List    `tfsdk:"association_details"`
}

// NewDataSource returns a new nutanix_license_keys_v2 data source.
func NewDataSource() datasource.DataSource {
	return listdata.New(licenseKeyDataSourceDescriptor{}.DataSource())
}

type licenseKeyDataSourceDescriptor struct{}

func (licenseKeyDataSourceDescriptor) Schema() schema.Schema {
	attributes := listquery.QueryAttributes()
	attributes["expand"] = listquery.StringAttribute("Comma-separated relationships to expand, such as assignmentDetails or associationDetails.")
	keyAttributes := map[string]schema.Attribute{
		"ext_id":                  listquery.ComputedString(),
		"tenant_id":               listquery.ComputedString(),
		"key":                     listquery.ComputedString(),
		"validation_detail":       listquery.ComputedString(),
		"type":                    listquery.ComputedString(),
		"category":                listquery.ComputedString(),
		"sub_category":            listquery.ComputedString(),
		"entitlement_expiry_date": listquery.ComputedString(),
		"meter":                   listquery.ComputedString(),
		"quantity":                schema.Float64Attribute{Computed: true},
		"group_id":                listquery.ComputedString(),
		"enforcement_policy":      listquery.ComputedString(),
		"assignment_details": schema.ListNestedAttribute{
			Computed: true,
			NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
				"key":            listquery.ComputedString(),
				"quantity_used":  schema.Float64Attribute{Computed: true},
				"cluster_ext_id": listquery.ComputedString(),
			}},
		},
		"association_details": schema.ListNestedAttribute{
			Computed: true,
			NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
				"base_key":         listquery.ComputedString(),
				"associated_key":   listquery.ComputedString(),
				"association_type": listquery.ComputedString(),
				"reclaim_type":     listquery.ComputedString(),
			}},
		},
	}
	attributes["id"] = listquery.ComputedID()
	attributes["license_key_entities"] = schema.ListNestedAttribute{
		Computed:     true,
		Description:  "License keys returned by Nutanix.",
		NestedObject: schema.NestedAttributeObject{Attributes: keyAttributes},
	}
	return schema.Schema{
		Description: "Lists Nutanix license keys through the Licensing v4.3 API.",
		Attributes:  attributes,
	}
}

func licenseKeyDataSourceDescriptorReader(value any) (listdata.ListFunc[licensing.LicenseKey], bool) {
	configured, ok := value.(providerData)
	if !ok {
		return nil, false
	}
	reader := configured.LicenseKeyReader()
	if reader == nil {
		return nil, true
	}
	return reader.ListLicenseKeys, true
}

func (licenseKeyDataSourceDescriptor) DataSource() listdata.Descriptor[licensing.LicenseKey, dataSourceModel] {
	return listdata.Descriptor[licensing.LicenseKey, dataSourceModel]{
		TypeName:            "license_keys_v2",
		IdentityTypeName:    "nutanix_license_keys_v2",
		Schema:              licenseKeyDataSourceDescriptor{}.Schema(),
		Reader:              licenseKeyDataSourceDescriptorReader,
		Query:               queryValues,
		SetState:            setStateFromLicenseKeys,
		ConfigureTitle:      "Unexpected Licensing Key Data Source Configure Type",
		ConfigureDetail:     "The provider supplied incompatible data to the licensing key data source.",
		MissingReaderTitle:  "Missing Licensing Key Reader",
		MissingReaderDetail: "The provider did not configure the licensing key reader.",
		ReadErrorTitle:      "Unable to Read Nutanix License Keys",
		ReadErrorDetail:     "The provider could not read and map the requested Nutanix license keys.",
	}
}

func queryValues(model dataSourceModel) listquery.Values {
	return listquery.Values{Page: model.Page, Limit: model.Limit, Filter: model.Filter, OrderBy: model.OrderBy, Select: model.Select, Expand: model.Expand}
}

func setStateFromLicenseKeys(ctx context.Context, config *dataSourceModel, id string, keys []licensing.LicenseKey) diag.Diagnostics {
	state, diagnostics := stateFromLicenseKeys(ctx, *config, id, keys)
	*config = state
	return diagnostics
}

func stateFromLicenseKeys(ctx context.Context, config dataSourceModel, id string, keys []licensing.LicenseKey) (dataSourceModel, diag.Diagnostics) {
	var diagnostics diag.Diagnostics
	models := make([]licenseKeyModel, len(keys))
	for index := range keys {
		assignment, current := mappingListValue(ctx, keys[index].AssignmentDetails)
		diagnostics.Append(current...)
		association, current := associationListValue(ctx, keys[index].AssociationDetails)
		diagnostics.Append(current...)
		models[index] = licenseKeyModel{
			ExtID:                 types.StringPointerValue(keys[index].ExtID),
			TenantID:              types.StringPointerValue(keys[index].TenantID),
			Key:                   types.StringPointerValue(keys[index].Key),
			ValidationDetail:      types.StringPointerValue(keys[index].ValidationDetail),
			Type:                  types.StringPointerValue(keys[index].Type),
			Category:              types.StringPointerValue(keys[index].Category),
			SubCategory:           types.StringPointerValue(keys[index].SubCategory),
			EntitlementExpiryDate: types.StringPointerValue(keys[index].EntitlementExpiryDate),
			Meter:                 types.StringPointerValue(keys[index].Meter),
			Quantity:              types.Float64PointerValue(keys[index].Quantity),
			GroupID:               types.StringPointerValue(keys[index].GroupID),
			EnforcementPolicy:     types.StringPointerValue(keys[index].EnforcementPolicy),
			AssignmentDetails:     assignment,
			AssociationDetails:    association,
		}
	}
	entities, current := types.ListValueFrom(ctx, licenseKeyObjectType, models)
	diagnostics.Append(current...)
	config.ID = types.StringValue(id)
	config.LicenseKeyEntities = entities
	return config, diagnostics
}

func mappingListValue(ctx context.Context, values *[]licensing.LicenseKeyMapping) (types.List, diag.Diagnostics) {
	if values == nil {
		return types.ListNull(licenseKeyMappingObjectType), nil
	}
	models := make([]mappingModel, len(*values))
	for index := range *values {
		models[index] = mappingModel{
			Key:          types.StringPointerValue((*values)[index].Key),
			QuantityUsed: types.Float64PointerValue((*values)[index].QuantityUsed),
			ClusterExtID: types.StringPointerValue((*values)[index].ClusterExtID),
		}
	}
	return types.ListValueFrom(ctx, licenseKeyMappingObjectType, models)
}

func associationListValue(ctx context.Context, values *[]licensing.LicenseKeyAssociation) (types.List, diag.Diagnostics) {
	if values == nil {
		return types.ListNull(licenseKeyAssociationObjectType), nil
	}
	models := make([]associationModel, len(*values))
	for index := range *values {
		models[index] = associationModel{
			BaseKey:         types.StringPointerValue((*values)[index].BaseKey),
			AssociatedKey:   types.StringPointerValue((*values)[index].AssociatedKey),
			AssociationType: types.StringPointerValue((*values)[index].AssociationType),
			ReclaimType:     types.StringPointerValue((*values)[index].ReclaimType),
		}
	}
	return types.ListValueFrom(ctx, licenseKeyAssociationObjectType, models)
}
