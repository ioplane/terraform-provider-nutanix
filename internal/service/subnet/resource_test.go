package subnet

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/ioplane/terraform-provider-nutanix/internal/nutanix/networking"
	"github.com/ioplane/terraform-provider-nutanix/internal/service/testkit"
)

func TestResourceContract(t *testing.T) {
	testkit.AssertResourceContract(t, NewResource(), "nutanix_subnet", map[string]testkit.AttributeFlags{
		"id": testkit.Computed("StringAttribute"), "ext_id": testkit.Computed("StringAttribute"),
		"name": testkit.Required("StringAttribute"), "description": testkit.OptionalComputed("StringAttribute"),
		"subnet_type": testkit.Required("StringAttribute"), "network_id": testkit.OptionalComputed("Int64Attribute"),
		"cluster_reference": testkit.OptionalComputed("StringAttribute"), "virtual_switch_reference": testkit.OptionalComputed("StringAttribute"),
		"vpc_reference": testkit.OptionalComputed("StringAttribute"), "is_nat_enabled": testkit.OptionalComputed("BoolAttribute"),
		"is_external": testkit.OptionalComputed("BoolAttribute"), "bridge_name": testkit.OptionalComputed("StringAttribute"),
		"is_advanced_networking": testkit.OptionalComputed("BoolAttribute"),
	})
}

func TestStateFromRemoteRejectsMissingIdentity(t *testing.T) {
	if _, err := stateFromRemote(resourceModel{}, networking.Subnet{}); err == nil {
		t.Fatal("stateFromRemote() error = nil, want invalid subnet")
	}
}

func TestSpecFromModelOmitsUnknownOptionalValues(t *testing.T) {
	spec := specFromModel(resourceModel{SubnetType: types.StringValue("OVERLAY")})
	if spec.NetworkID != nil {
		t.Fatalf("specFromModel() network ID = %v, want nil for overlay", *spec.NetworkID)
	}
}
