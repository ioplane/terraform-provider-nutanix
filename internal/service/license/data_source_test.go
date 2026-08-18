package license

import (
	"context"
	"testing"

	"github.com/ioplane/terraform-provider-nutanix/internal/nutanix/licensing"
	"github.com/ioplane/terraform-provider-nutanix/internal/service/testkit"
)

func TestDataSourceContract(t *testing.T) {
	testkit.AssertDataSourceContract(t, NewDataSource(), "nutanix_licenses_v2", map[string]testkit.AttributeFlags{
		"page": testkit.Optional("Int64Attribute"), "limit": testkit.Optional("Int64Attribute"),
		"filter": testkit.Optional("StringAttribute"), "order_by": testkit.Optional("StringAttribute"),
		"select": testkit.Optional("StringAttribute"), "expand": testkit.Optional("StringAttribute"),
		"id": testkit.Computed("StringAttribute"), "license_entities": testkit.Computed("ListNestedAttribute"),
	})
}

func TestStateFromLicensesPreservesNullConsumptionDetails(t *testing.T) {
	state, diagnostics := stateFromLicenses(context.Background(), dataSourceModel{}, "query-id", []licensing.License{{}})
	if diagnostics.HasError() {
		t.Fatalf("stateFromLicenses() diagnostics = %v", diagnostics)
	}
	if state.ID.ValueString() != "query-id" || len(state.LicenseEntities.Elements()) != 1 {
		t.Fatalf("state = %#v, want query identity and one license", state)
	}
}
