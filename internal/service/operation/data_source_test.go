package operation

import (
	"context"
	"testing"

	"github.com/ioplane/terraform-provider-nutanix/internal/nutanix/iam"
	"github.com/ioplane/terraform-provider-nutanix/internal/service/testkit"
)

func TestDataSourceContract(t *testing.T) {
	testkit.AssertDataSourceContract(t, NewDataSource(), "nutanix_operations_v2", map[string]testkit.AttributeFlags{
		"page": testkit.Optional("Int64Attribute"), "limit": testkit.Optional("Int64Attribute"),
		"filter": testkit.Optional("StringAttribute"), "order_by": testkit.Optional("StringAttribute"),
		"select": testkit.Optional("StringAttribute"), "id": testkit.Computed("StringAttribute"),
		"operation_entities": testkit.Computed("ListNestedAttribute"),
	})
}

func TestStateFromOperationsPreservesNullCollections(t *testing.T) {
	state, diagnostics := stateFromOperations(context.Background(), dataSourceModel{}, "query-id", []iam.Operation{{}})
	if diagnostics.HasError() {
		t.Fatalf("stateFromOperations() diagnostics = %v", diagnostics)
	}
	if state.ID.ValueString() != "query-id" || len(state.OperationEntities.Elements()) != 1 {
		t.Fatalf("state = %#v, want query identity and one operation", state)
	}
}
