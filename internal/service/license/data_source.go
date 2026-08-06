// Package license implements the nutanix_licenses_v2 data source.
package license

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

var consumptionObjectType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"cluster_ext_id": types.StringType,
	"quantity_used":  types.Float64Type,
}}

var licenseObjectType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"ext_id":                types.StringType,
	"category":              types.StringType,
	"expiry_date":           types.StringType,
	"name":                  types.StringType,
	"sub_category":          types.StringType,
	"type":                  types.StringType,
	"meter":                 types.StringType,
	"quantity":              types.Float64Type,
	"scope":                 types.StringType,
	"salesforce_license_id": types.StringType,
	"consumption_details":   types.ListType{ElemType: consumptionObjectType},
}}

// Reader is the Licensing capability consumed by this data source.
type Reader interface {
	ListLicenses(context.Context, odata.ListOptions) ([]licensing.License, url.Values, error)
}

type providerData interface {
	LicenseReader() Reader
}

type dataSourceModel struct {
	Page            types.Int64  `tfsdk:"page"`
	Limit           types.Int64  `tfsdk:"limit"`
	Filter          types.String `tfsdk:"filter"`
	OrderBy         types.String `tfsdk:"order_by"`
	Select          types.String `tfsdk:"select"`
	Expand          types.String `tfsdk:"expand"`
	ID              types.String `tfsdk:"id"`
	LicenseEntities types.List   `tfsdk:"license_entities"`
}

type consumptionModel struct {
	ClusterExtID types.String  `tfsdk:"cluster_ext_id"`
	QuantityUsed types.Float64 `tfsdk:"quantity_used"`
}

type licenseModel struct {
	ExtID               types.String  `tfsdk:"ext_id"`
	Category            types.String  `tfsdk:"category"`
	ExpiryDate          types.String  `tfsdk:"expiry_date"`
	Name                types.String  `tfsdk:"name"`
	SubCategory         types.String  `tfsdk:"sub_category"`
	Type                types.String  `tfsdk:"type"`
	Meter               types.String  `tfsdk:"meter"`
	Quantity            types.Float64 `tfsdk:"quantity"`
	Scope               types.String  `tfsdk:"scope"`
	SalesforceLicenseID types.String  `tfsdk:"salesforce_license_id"`
	ConsumptionDetails  types.List    `tfsdk:"consumption_details"`
}

// NewDataSource returns a new nutanix_licenses_v2 data source.
func NewDataSource() datasource.DataSource {
	return listdata.New(licenseDataSourceDescriptor{}.DataSource())
}

type licenseDataSourceDescriptor struct{}

func (licenseDataSourceDescriptor) Schema() schema.Schema {
	attributes := listquery.QueryAttributes()
	attributes["expand"] = listquery.StringAttribute("Comma-separated relationships to expand, such as consumptionDetails.")
	licenseAttributes := map[string]schema.Attribute{
		"ext_id":                listquery.ComputedString(),
		"name":                  listquery.ComputedString(),
		"expiry_date":           listquery.ComputedString(),
		"category":              listquery.ComputedString(),
		"sub_category":          listquery.ComputedString(),
		"type":                  listquery.ComputedString(),
		"meter":                 listquery.ComputedString(),
		"quantity":              schema.Float64Attribute{Computed: true},
		"scope":                 listquery.ComputedString(),
		"salesforce_license_id": listquery.ComputedString(),
		"consumption_details": schema.ListNestedAttribute{
			Computed: true,
			NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
				"cluster_ext_id": listquery.ComputedString(),
				"quantity_used":  schema.Float64Attribute{Computed: true},
			}},
		},
	}
	attributes["id"] = listquery.ComputedID()
	attributes["license_entities"] = schema.ListNestedAttribute{
		Computed:     true,
		Description:  "Applied licenses returned by Nutanix.",
		NestedObject: schema.NestedAttributeObject{Attributes: licenseAttributes},
	}
	return schema.Schema{
		Description: "Lists applied Nutanix licenses through the Licensing v4.3 API.",
		Attributes:  attributes,
	}
}

func licenseDataSourceDescriptorReader(value any) (listdata.ListFunc[licensing.License], bool) {
	configured, ok := value.(providerData)
	if !ok {
		return nil, false
	}
	reader := configured.LicenseReader()
	if reader == nil {
		return nil, true
	}
	return reader.ListLicenses, true
}

func (licenseDataSourceDescriptor) DataSource() listdata.Descriptor[licensing.License, dataSourceModel] {
	return listdata.Descriptor[licensing.License, dataSourceModel]{
		TypeName:            "licenses_v2",
		IdentityTypeName:    "nutanix_licenses_v2",
		Schema:              licenseDataSourceDescriptor{}.Schema(),
		Reader:              licenseDataSourceDescriptorReader,
		Query:               queryValues,
		SetState:            setStateFromLicenses,
		ConfigureTitle:      "Unexpected Licensing Data Source Configure Type",
		ConfigureDetail:     "The provider supplied incompatible data to the licensing data source.",
		MissingReaderTitle:  "Missing Licensing Reader",
		MissingReaderDetail: "The provider did not configure the licensing reader.",
		ReadErrorTitle:      "Unable to Read Nutanix Licenses",
		ReadErrorDetail:     "The provider could not read and map the requested Nutanix licenses.",
	}
}

func queryValues(model dataSourceModel) listquery.Values {
	return listquery.Values{Page: model.Page, Limit: model.Limit, Filter: model.Filter, OrderBy: model.OrderBy, Select: model.Select, Expand: model.Expand}
}

func setStateFromLicenses(ctx context.Context, config *dataSourceModel, id string, licenses []licensing.License) diag.Diagnostics {
	state, diagnostics := stateFromLicenses(ctx, *config, id, licenses)
	*config = state
	return diagnostics
}

func stateFromLicenses(ctx context.Context, config dataSourceModel, id string, licenses []licensing.License) (dataSourceModel, diag.Diagnostics) {
	var diagnostics diag.Diagnostics
	models := make([]licenseModel, len(licenses))
	for index := range licenses {
		consumption, current := consumptionListValue(ctx, licenses[index].ConsumptionDetails)
		diagnostics.Append(current...)
		models[index] = licenseModel{
			ExtID:               types.StringPointerValue(licenses[index].ExtID),
			Category:            types.StringPointerValue(licenses[index].Category),
			ExpiryDate:          types.StringPointerValue(licenses[index].ExpiryDate),
			Name:                types.StringPointerValue(licenses[index].Name),
			SubCategory:         types.StringPointerValue(licenses[index].SubCategory),
			Type:                types.StringPointerValue(licenses[index].Type),
			Meter:               types.StringPointerValue(licenses[index].Meter),
			Quantity:            types.Float64PointerValue(licenses[index].Quantity),
			Scope:               types.StringPointerValue(licenses[index].Scope),
			SalesforceLicenseID: types.StringPointerValue(licenses[index].SalesforceLicenseID),
			ConsumptionDetails:  consumption,
		}
	}
	entities, current := types.ListValueFrom(ctx, licenseObjectType, models)
	diagnostics.Append(current...)
	config.ID = types.StringValue(id)
	config.LicenseEntities = entities
	return config, diagnostics
}

func consumptionListValue(ctx context.Context, values *[]licensing.Consumption) (types.List, diag.Diagnostics) {
	if values == nil {
		return types.ListNull(consumptionObjectType), nil
	}
	models := make([]consumptionModel, len(*values))
	for index := range *values {
		models[index] = consumptionModel{
			ClusterExtID: types.StringPointerValue((*values)[index].ClusterExtID),
			QuantityUsed: types.Float64PointerValue((*values)[index].QuantityUsed),
		}
	}
	return types.ListValueFrom(ctx, consumptionObjectType, models)
}
