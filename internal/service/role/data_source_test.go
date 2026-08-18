package role

import (
	"context"
	"testing"

	"github.com/ioplane/terraform-provider-nutanix/internal/nutanix/iam"
	"github.com/ioplane/terraform-provider-nutanix/internal/service/testkit"
)

func TestDataSourceContract(t *testing.T) {
	optional := testkit.AttributeFlags{Optional: true}
	computed := testkit.AttributeFlags{Computed: true}
	testkit.AssertDataSourceContract(t, NewDataSource(), "nutanix_roles_v2", map[string]testkit.AttributeFlags{
		"page": optional, "limit": optional, "filter": optional, "order_by": optional, "select": optional,
		"id": computed, "role_entities": computed,
	})
}

func TestStateFromRolesPreservesNullCollections(t *testing.T) {
	state, diagnostics := stateFromRoles(context.Background(), dataSourceModel{}, "query-id", []iam.Role{{}})
	if diagnostics.HasError() {
		t.Fatalf("stateFromRoles() diagnostics = %v", diagnostics)
	}
	if state.ID.ValueString() != "query-id" || len(state.RoleEntities.Elements()) != 1 {
		t.Fatalf("state = %#v, want query identity and one role", state)
	}
}
