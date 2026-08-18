package licensefeature

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/ioplane/terraform-provider-nutanix/internal/nutanix/licensing"
	"github.com/ioplane/terraform-provider-nutanix/internal/service/testkit"
)

func TestDataSourceContract(t *testing.T) {
	testkit.AssertDataSourceContract(t, NewDataSource(), "nutanix_license_features_v2", map[string]testkit.AttributeFlags{
		"page": testkit.Optional("Int64Attribute"), "limit": testkit.Optional("Int64Attribute"),
		"filter": testkit.Optional("StringAttribute"), "order_by": testkit.Optional("StringAttribute"),
		"select": testkit.Optional("StringAttribute"), "id": testkit.Computed("StringAttribute"),
		"license_feature_entities": testkit.Computed("ListNestedAttribute"),
	})
}

func TestStateFromFeaturesNormalizesUnionValue(t *testing.T) {
	name, valueType, licenseType := "dp_recovery", "BOOLEAN", "PRISM"
	features := []licensing.Feature{{
		Name:        &name,
		ValueType:   &valueType,
		Value:       json.RawMessage("true"),
		LicenseType: &licenseType,
	}}

	state, diagnostics := stateFromFeatures(context.Background(), dataSourceModel{}, "query-id", features)
	if diagnostics.HasError() {
		t.Fatalf("stateFromFeatures() diagnostics = %v", diagnostics)
	}
	if state.ID != types.StringValue("query-id") {
		t.Fatalf("state ID = %v, want query-id", state.ID)
	}
	if state.LicenseFeatureEntities.IsNull() || len(state.LicenseFeatureEntities.Elements()) != 1 {
		t.Fatalf("feature entities = %v, want one entity", state.LicenseFeatureEntities)
	}
}

func TestFeatureValueRejectsUnsupportedType(t *testing.T) {
	if _, err := featureValue(json.RawMessage(`"unexpected"`)); err == nil {
		t.Fatal("featureValue() error = nil, want unsupported type error")
	}
}

func TestFeatureValuePreservesLargeInteger(t *testing.T) {
	value, err := featureValue(json.RawMessage("9007199254740993"))
	if err != nil {
		t.Fatalf("featureValue() error = %v", err)
	}
	if got := value.ValueString(); got != "9007199254740993" {
		t.Fatalf("featureValue() = %q, want exact integer", got)
	}
}
