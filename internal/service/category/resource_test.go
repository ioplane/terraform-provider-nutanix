package category

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/ioplane/terraform-provider-nutanix/internal/nutanix/prism"
	"github.com/ioplane/terraform-provider-nutanix/internal/service/testkit"
)

func TestResourceContract(t *testing.T) {
	required := testkit.AttributeFlags{Required: true}
	optionalComputed := testkit.AttributeFlags{Optional: true, Computed: true}
	computed := testkit.AttributeFlags{Computed: true}
	testkit.AssertResourceContract(t, NewResource(), "nutanix_category", map[string]testkit.AttributeFlags{
		"id": computed, "key": required, "value": required, "description": optionalComputed, "owner_uuid": optionalComputed, "type": computed,
	})
}

func TestStateFromRemoteRejectsIncompleteIdentity(t *testing.T) {
	if _, err := stateFromRemote(resourceModel{}, prism.Category{}); err == nil {
		t.Fatal("stateFromRemote() error = nil, want invalid category")
	}
}

func TestCategorySpecPreservesOptionalNulls(t *testing.T) {
	spec := categorySpec(resourceModel{Key: types.StringValue("owner"), Value: types.StringValue("value")})
	if spec.Description != nil || spec.OwnerUUID != nil {
		t.Fatalf("categorySpec() optional pointers = %#v/%#v, want nil", spec.Description, spec.OwnerUUID)
	}
}
