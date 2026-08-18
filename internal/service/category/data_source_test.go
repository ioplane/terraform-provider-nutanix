package category

import (
	"context"
	"testing"

	"github.com/ioplane/terraform-provider-nutanix/internal/nutanix/prism"
	"github.com/ioplane/terraform-provider-nutanix/internal/service/testkit"
)

func TestDataSourceContract(t *testing.T) {
	optional := testkit.AttributeFlags{Optional: true}
	computed := testkit.AttributeFlags{Computed: true}
	testkit.AssertDataSourceContract(t, NewDataSource(), "nutanix_categories_v2", map[string]testkit.AttributeFlags{
		"page": optional, "limit": optional, "filter": optional, "order_by": optional, "select": optional, "expand": optional,
		"id": computed, "categories": computed,
	})
}

func TestStateFromCategoriesPreservesNullAssociations(t *testing.T) {
	state, diagnostics := stateFromCategories(context.Background(), dataSourceModel{}, "query-id", []prism.Category{{}})
	if diagnostics.HasError() {
		t.Fatalf("stateFromCategories() diagnostics = %v", diagnostics)
	}
	if state.ID.ValueString() != "query-id" || len(state.Categories.Elements()) != 1 {
		t.Fatalf("state = %#v, want query identity and one category", state)
	}
}
