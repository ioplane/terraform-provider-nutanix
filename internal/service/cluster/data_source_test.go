package cluster

import (
	"context"
	"testing"

	"github.com/ioplane/terraform-provider-nutanix/internal/nutanix/clustermgmt"
	"github.com/ioplane/terraform-provider-nutanix/internal/service/testkit"
)

func TestDataSourceContract(t *testing.T) {
	testkit.AssertDataSourceContract(t, NewDataSource(), "nutanix_clusters_v2", map[string]testkit.AttributeFlags{
		"page": testkit.Optional("Int64Attribute"), "limit": testkit.Optional("Int64Attribute"),
		"filter": testkit.Optional("StringAttribute"), "order_by": testkit.Optional("StringAttribute"),
		"select": testkit.Optional("StringAttribute"), "expand": testkit.Optional("StringAttribute"),
		"id": testkit.Computed("StringAttribute"), "cluster_entities": testkit.Computed("ListNestedAttribute"),
	})
}

func TestStateFromClustersPreservesNullOptionalProjection(t *testing.T) {
	state, diagnostics := stateFromClusters(context.Background(), dataSourceModel{}, "query-id", []clustermgmt.Cluster{{}})
	if diagnostics.HasError() {
		t.Fatalf("stateFromClusters() diagnostics = %v", diagnostics)
	}
	if state.ID.ValueString() != "query-id" || len(state.ClusterEntities.Elements()) != 1 {
		t.Fatalf("state = %#v, want query identity and one cluster", state)
	}
}
