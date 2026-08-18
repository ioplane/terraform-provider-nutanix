package category

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/ioplane/terraform-provider-nutanix/internal/nutanix/prism"
	"github.com/ioplane/terraform-provider-nutanix/internal/service/testkit"
)

func TestDataSourceContract(t *testing.T) {
	testkit.AssertDataSourceContract(t, NewDataSource(), "nutanix_categories_v2", map[string]testkit.AttributeFlags{
		"page": testkit.Optional("Int64Attribute"), "limit": testkit.Optional("Int64Attribute"),
		"filter": testkit.Optional("StringAttribute"), "order_by": testkit.Optional("StringAttribute"),
		"select": testkit.Optional("StringAttribute"), "expand": testkit.Optional("StringAttribute"),
		"id": testkit.Computed("StringAttribute"), "categories": testkit.NestedComputed("ListNestedAttribute", map[string]testkit.AttributeFlags{
			"ext_id": testkit.Computed("StringAttribute"), "key": testkit.Computed("StringAttribute"), "value": testkit.Computed("StringAttribute"),
			"type": testkit.Computed("StringAttribute"), "description": testkit.Computed("StringAttribute"), "owner_uuid": testkit.Computed("StringAttribute"),
			"associations": testkit.NestedComputed("ListNestedAttribute", map[string]testkit.AttributeFlags{
				"category_id": testkit.Computed("StringAttribute"), "resource_type": testkit.Computed("StringAttribute"),
				"resource_group": testkit.Computed("StringAttribute"), "count": testkit.Computed("Int64Attribute"),
			}),
			"detailed_associations": testkit.NestedComputed("ListNestedAttribute", map[string]testkit.AttributeFlags{
				"category_id": testkit.Computed("StringAttribute"), "resource_type": testkit.Computed("StringAttribute"),
				"resource_group": testkit.Computed("StringAttribute"), "resource_id": testkit.Computed("StringAttribute"),
			}),
		}),
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
	category := state.Categories.Elements()[0].(types.Object)
	if !category.Attributes()["associations"].(types.List).IsNull() || !category.Attributes()["detailed_associations"].(types.List).IsNull() {
		t.Fatal("category association fields are not null for nil API pointers")
	}
}
