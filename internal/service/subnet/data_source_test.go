package subnet

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/ioplane/terraform-provider-nutanix/internal/nutanix/networking"
	"github.com/ioplane/terraform-provider-nutanix/internal/service/testkit"
)

func TestDataSourceContract(t *testing.T) {
	required := testkit.AttributeFlags{Required: true}
	computed := testkit.AttributeFlags{Computed: true}
	testkit.AssertDataSourceContract(t, NewDataSource(), "nutanix_subnet_v2", map[string]testkit.AttributeFlags{
		"id": computed, "ext_id": required, "name": computed, "description": computed, "subnet_type": computed,
		"network_id": computed, "ip_config": computed, "cluster_reference": computed, "virtual_switch_reference": computed,
		"vpc_reference": computed, "is_nat_enabled": computed, "is_external": computed, "bridge_name": computed,
		"is_advanced_networking": computed,
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
