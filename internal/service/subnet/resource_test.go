package subnet

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/ioplane/terraform-provider-nutanix/internal/nutanix/networking"
	"github.com/ioplane/terraform-provider-nutanix/internal/service/testkit"
)

func TestResourceContract(t *testing.T) {
	required := testkit.AttributeFlags{Required: true}
	optionalComputed := testkit.AttributeFlags{Optional: true, Computed: true}
	computed := testkit.AttributeFlags{Computed: true}
	testkit.AssertResourceContract(t, NewResource(), "nutanix_subnet", map[string]testkit.AttributeFlags{
		"id": computed, "ext_id": computed, "name": required, "description": optionalComputed, "subnet_type": required,
		"network_id": optionalComputed, "cluster_reference": optionalComputed, "virtual_switch_reference": optionalComputed,
		"vpc_reference": optionalComputed, "is_nat_enabled": optionalComputed, "is_external": optionalComputed,
		"bridge_name": optionalComputed, "is_advanced_networking": optionalComputed,
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
