package subnet

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/ioplane/terraform-provider-nutanix/internal/nutanix/networking"
	"github.com/ioplane/terraform-provider-nutanix/internal/service/testkit"
)

func TestDataSourceContract(t *testing.T) {
	testkit.AssertDataSourceContract(t, NewDataSource(), "nutanix_subnet_v2", map[string]testkit.AttributeFlags{
		"id": testkit.Computed("StringAttribute"), "ext_id": testkit.Required("StringAttribute"),
		"name": testkit.Computed("StringAttribute"), "description": testkit.Computed("StringAttribute"), "subnet_type": testkit.Computed("StringAttribute"),
		"network_id": testkit.Computed("Int64Attribute"), "ip_config": testkit.Computed("ListNestedAttribute"),
		"cluster_reference": testkit.Computed("StringAttribute"), "virtual_switch_reference": testkit.Computed("StringAttribute"),
		"vpc_reference": testkit.Computed("StringAttribute"), "is_nat_enabled": testkit.Computed("BoolAttribute"),
		"is_external": testkit.Computed("BoolAttribute"), "bridge_name": testkit.Computed("StringAttribute"),
		"is_advanced_networking": testkit.Computed("BoolAttribute"),
	})
}

func TestStateFromSubnetPreservesNullIPConfig(t *testing.T) {
	extID := "2b4f1b3a-68d0-4ac5-9d16-0d4b7ad7e2bb"
	state, diagnostics := stateFromSubnet(context.Background(), dataSourceModel{}, extID, networking.Subnet{})
	if diagnostics.HasError() {
		t.Fatalf("stateFromSubnet() diagnostics = %v", diagnostics)
	}
	if state.ID != types.StringValue(extID) || !state.IPConfig.IsNull() {
		t.Fatalf("state identity/ip config = %v/%v, want %q/null", state.ID, state.IPConfig, extID)
	}
}

func TestSubnetQueryRejectsUnknownIdentity(t *testing.T) {
	_, diagnostics := extIDFromModel(types.StringUnknown())
	if !diagnostics.HasError() {
		t.Fatal("extIDFromModel() diagnostics = nil, want unknown identity error")
	}
}
