package licensefeature

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/ioplane/terraform-provider-nutanix/internal/nutanix/licensing"
)

func TestStateFromFeaturesNormalizesUnionValue(t *testing.T) {
	name, valueType, licenseType := "dp_recovery", "BOOLEAN", "PRISM"
	features := []licensing.Feature{{
		Name:        &name,
		ValueType:   &valueType,
		Value:       true,
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
	if _, err := featureValue("unexpected"); err == nil {
		t.Fatal("featureValue() error = nil, want unsupported type error")
	}
}
