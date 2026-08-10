// Package licensefeature implements the nutanix_license_features_v2 data source.
package licensefeature

import (
	"context"
	"fmt"
	"net/url"
	"strconv"

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

var featureObjectType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"name":                 types.StringType,
	"value_type":           types.StringType,
	"value":                types.StringType,
	"license_type":         types.StringType,
	"license_category":     types.StringType,
	"license_sub_category": types.StringType,
	"scope":                types.StringType,
}}

// Reader is the Licensing capability consumed by this data source.
type Reader interface {
	ListFeatures(context.Context, odata.ListOptions) ([]licensing.Feature, url.Values, error)
}

type providerData interface {
	FeatureReader() Reader
}

type dataSourceModel struct {
	Page                   types.Int64  `tfsdk:"page"`
	Limit                  types.Int64  `tfsdk:"limit"`
	Filter                 types.String `tfsdk:"filter"`
	OrderBy                types.String `tfsdk:"order_by"`
	Select                 types.String `tfsdk:"select"`
	ID                     types.String `tfsdk:"id"`
	LicenseFeatureEntities types.List   `tfsdk:"license_feature_entities"`
}

type featureModel struct {
	Name               types.String `tfsdk:"name"`
	ValueType          types.String `tfsdk:"value_type"`
	Value              types.String `tfsdk:"value"`
	LicenseType        types.String `tfsdk:"license_type"`
	LicenseCategory    types.String `tfsdk:"license_category"`
	LicenseSubCategory types.String `tfsdk:"license_sub_category"`
	Scope              types.String `tfsdk:"scope"`
}

// NewDataSource returns a new nutanix_license_features_v2 data source.
func NewDataSource() datasource.DataSource {
	return listdata.New(licenseFeatureDataSourceDescriptor{}.DataSource())
}

type licenseFeatureDataSourceDescriptor struct{}

func (licenseFeatureDataSourceDescriptor) Schema() schema.Schema {
	attributes := listquery.QueryAttributes()
	attributes["id"] = listquery.ComputedID()
	attributes["license_feature_entities"] = schema.ListNestedAttribute{
		Computed:    true,
		Description: "License features returned by Nutanix.",
		NestedObject: schema.NestedAttributeObject{Attributes: map[string]schema.Attribute{
			"name":                 listquery.ComputedString(),
			"value_type":           listquery.ComputedString(),
			"value":                schema.StringAttribute{Computed: true, Description: "Feature value returned as a string; use value_type to interpret boolean or integer values."},
			"license_type":         listquery.ComputedString(),
			"license_category":     listquery.ComputedString(),
			"license_sub_category": listquery.ComputedString(),
			"scope":                listquery.ComputedString(),
		}},
	}
	return schema.Schema{
		Description: "Lists Nutanix license features through the Licensing v4.3 API.",
		Attributes:  attributes,
	}
}

func licenseFeatureDataSourceDescriptorReader(value any) (listdata.ListFunc[licensing.Feature], bool) {
	configured, ok := value.(providerData)
	if !ok {
		return nil, false
	}
	reader := configured.FeatureReader()
	if reader == nil {
		return nil, true
	}
	return reader.ListFeatures, true
}

func (licenseFeatureDataSourceDescriptor) DataSource() listdata.Descriptor[licensing.Feature, dataSourceModel] {
	return listdata.Descriptor[licensing.Feature, dataSourceModel]{
		TypeName:            "license_features_v2",
		IdentityTypeName:    "nutanix_license_features_v2",
		Schema:              licenseFeatureDataSourceDescriptor{}.Schema(),
		Reader:              licenseFeatureDataSourceDescriptorReader,
		Query:               queryValues,
		SetState:            setStateFromFeatures,
		ConfigureTitle:      "Unexpected Licensing Feature Data Source Configure Type",
		ConfigureDetail:     "The provider supplied incompatible data to the licensing feature data source.",
		MissingReaderTitle:  "Missing Licensing Feature Reader",
		MissingReaderDetail: "The provider did not configure the licensing feature reader.",
		ReadErrorTitle:      "Unable to Read Nutanix License Features",
		ReadErrorDetail:     "The provider could not read and map the requested Nutanix license features.",
	}
}

func queryValues(model dataSourceModel) listquery.Values {
	return listquery.Values{Page: model.Page, Limit: model.Limit, Filter: model.Filter, OrderBy: model.OrderBy, Select: model.Select}
}

func setStateFromFeatures(ctx context.Context, config *dataSourceModel, id string, features []licensing.Feature) diag.Diagnostics {
	state, diagnostics := stateFromFeatures(ctx, *config, id, features)
	*config = state
	return diagnostics
}

func stateFromFeatures(ctx context.Context, config dataSourceModel, id string, features []licensing.Feature) (dataSourceModel, diag.Diagnostics) {
	var diagnostics diag.Diagnostics
	models := make([]featureModel, len(features))
	for index := range features {
		value, err := featureValue(features[index].Value)
		if err != nil {
			diagnostics.AddError("Invalid Nutanix license feature value", "The Licensing API returned an unsupported value type.")
			continue
		}
		models[index] = featureModel{
			Name:               types.StringPointerValue(features[index].Name),
			ValueType:          types.StringPointerValue(features[index].ValueType),
			Value:              value,
			LicenseType:        types.StringPointerValue(features[index].LicenseType),
			LicenseCategory:    types.StringPointerValue(features[index].LicenseCategory),
			LicenseSubCategory: types.StringPointerValue(features[index].LicenseSubCategory),
			Scope:              types.StringPointerValue(features[index].Scope),
		}
	}
	entities, current := types.ListValueFrom(ctx, featureObjectType, models)
	diagnostics.Append(current...)
	config.ID = types.StringValue(id)
	config.LicenseFeatureEntities = entities
	return config, diagnostics
}

func featureValue(value any) (types.String, error) {
	switch value := value.(type) {
	case nil:
		return types.StringNull(), nil
	case bool:
		return types.StringValue(strconv.FormatBool(value)), nil
	case float64:
		return types.StringValue(strconv.FormatFloat(value, 'f', -1, 64)), nil
	default:
		return types.StringNull(), fmt.Errorf("unsupported license feature value type %T", value)
	}
}
