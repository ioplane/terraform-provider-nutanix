package licensekey

import (
	"context"
	"testing"

	"github.com/ioplane/terraform-provider-nutanix/internal/nutanix/licensing"
	"github.com/ioplane/terraform-provider-nutanix/internal/service/testkit"
)

func TestDataSourceContract(t *testing.T) {
	optional := testkit.AttributeFlags{Optional: true}
	computed := testkit.AttributeFlags{Computed: true}
	testkit.AssertDataSourceContract(t, NewDataSource(), "nutanix_license_keys_v2", map[string]testkit.AttributeFlags{
		"page": optional, "limit": optional, "filter": optional, "order_by": optional, "select": optional, "expand": optional,
		"id": computed, "license_key_entities": computed,
	})
}

func TestStateFromLicenseKeysPreservesNullExpansions(t *testing.T) {
	state, diagnostics := stateFromLicenseKeys(context.Background(), dataSourceModel{}, "query-id", []licensing.LicenseKey{{}})
	if diagnostics.HasError() {
		t.Fatalf("stateFromLicenseKeys() diagnostics = %v", diagnostics)
	}
	if state.ID.ValueString() != "query-id" || len(state.LicenseKeyEntities.Elements()) != 1 {
		t.Fatalf("state = %#v, want query identity and one license key", state)
	}
}
