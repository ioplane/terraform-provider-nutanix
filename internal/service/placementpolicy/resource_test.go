package placementpolicy

import (
	"context"
	"testing"

	"github.com/ioplane/terraform-provider-nutanix/internal/nutanix/vmm"
	"github.com/ioplane/terraform-provider-nutanix/internal/service/testkit"
)

func TestResourceContract(t *testing.T) {
	required := testkit.AttributeFlags{Required: true}
	optionalComputed := testkit.AttributeFlags{Optional: true, Computed: true}
	computed := testkit.AttributeFlags{Computed: true}
	testkit.AssertResourceContract(t, NewResource(), "nutanix_image_placement_policy", map[string]testkit.AttributeFlags{
		"id": computed, "ext_id": computed, "name": required, "description": optionalComputed, "placement_type": required,
		"image_entity_filter": required, "cluster_entity_filter": required, "enforcement_state": computed,
		"create_time": computed, "last_update_time": computed, "owner_ext_id": computed, "owner_name": computed,
	})
}

func TestStateFromRemoteRejectsIncompletePolicy(t *testing.T) {
	if _, err := stateFromRemote(context.Background(), resourceModel{}, vmm.PlacementPolicy{}); err == nil {
		t.Fatal("stateFromRemote() error = nil, want invalid placement policy")
	}
}
