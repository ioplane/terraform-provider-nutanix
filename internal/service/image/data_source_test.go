package image

import (
	"context"
	"testing"

	"github.com/ioplane/terraform-provider-nutanix/internal/nutanix/vmm"
	"github.com/ioplane/terraform-provider-nutanix/internal/service/testkit"
)

func TestDataSourceContract(t *testing.T) {
	testkit.AssertDataSourceContract(t, NewDataSource(), "nutanix_images_v2", map[string]testkit.AttributeFlags{
		"page": testkit.Optional("Int64Attribute"), "limit": testkit.Optional("Int64Attribute"),
		"filter": testkit.Optional("StringAttribute"), "order_by": testkit.Optional("StringAttribute"),
		"select": testkit.Optional("StringAttribute"), "id": testkit.Computed("StringAttribute"),
		"images": testkit.Computed("ListNestedAttribute"),
	})
}

func TestStateFromImagesPreservesNullOptionalProjection(t *testing.T) {
	state, diagnostics := stateFromImages(context.Background(), dataSourceModel{}, "query-id", []vmm.Image{{}})
	if diagnostics.HasError() {
		t.Fatalf("stateFromImages() diagnostics = %v", diagnostics)
	}
	if state.ID.ValueString() != "query-id" || len(state.Images.Elements()) != 1 {
		t.Fatalf("state = %#v, want query identity and one image", state)
	}
}
