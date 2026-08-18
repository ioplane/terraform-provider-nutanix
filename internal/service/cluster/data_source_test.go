package cluster

import (
	"context"
	"testing"

	"github.com/ioplane/terraform-provider-nutanix/internal/nutanix/clustermgmt"
	"github.com/ioplane/terraform-provider-nutanix/internal/service/testkit"
)

func TestDataSourceContract(t *testing.T) {
	optional := testkit.AttributeFlags{Optional: true}
	computed := testkit.AttributeFlags{Computed: true}
	testkit.AssertDataSourceContract(t, NewDataSource(), "nutanix_clusters_v2", map[string]testkit.AttributeFlags{
		"page": optional, "limit": optional, "filter": optional, "order_by": optional, "select": optional, "expand": optional,
		"id": computed, "cluster_entities": computed,
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
