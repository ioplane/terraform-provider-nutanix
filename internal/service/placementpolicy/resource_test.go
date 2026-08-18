package placementpolicy

import (
	"context"
	"testing"

	"github.com/ioplane/terraform-provider-nutanix/internal/nutanix/vmm"
	"github.com/ioplane/terraform-provider-nutanix/internal/service/testkit"
)

func TestResourceContract(t *testing.T) {
	testkit.AssertResourceContract(t, NewResource(), "nutanix_image_placement_policy", map[string]testkit.AttributeFlags{
		"id": testkit.Computed("StringAttribute"), "ext_id": testkit.Computed("StringAttribute"), "name": testkit.Required("StringAttribute"),
		"description": testkit.OptionalComputed("StringAttribute"), "placement_type": testkit.Required("StringAttribute"),
		"image_entity_filter": filterContract(), "cluster_entity_filter": filterContract(),
		"enforcement_state": testkit.Computed("StringAttribute"), "create_time": testkit.Computed("StringAttribute"),
		"last_update_time": testkit.Computed("StringAttribute"), "owner_ext_id": testkit.Computed("StringAttribute"), "owner_name": testkit.Computed("StringAttribute"),
	})
}

func filterContract() testkit.AttributeFlags {
	return testkit.AttributeFlags{TypeName: "SingleNestedAttribute", Required: true, Nested: map[string]testkit.AttributeFlags{
		"type":             testkit.Required("StringAttribute"),
		"category_ext_ids": testkit.Required("ListAttribute"),
	}}
}

func TestStateFromRemoteRejectsIncompletePolicy(t *testing.T) {
	if _, err := stateFromRemote(context.Background(), resourceModel{}, vmm.PlacementPolicy{}); err == nil {
		t.Fatal("stateFromRemote() error = nil, want invalid placement policy")
	}
}
