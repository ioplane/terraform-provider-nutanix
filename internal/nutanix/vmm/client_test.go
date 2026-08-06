package vmm

import (
	"testing"

	"github.com/ioplane/terraform-provider-nutanix/internal/task"
)

func TestPlacementPolicyIDFromTaskSelectsExactRelation(t *testing.T) {
	got, err := PlacementPolicyIDFromTask(task.Snapshot{EntitiesAffected: []task.EntityReference{
		{ExtID: "7ccae44f-d067-4d49-a4d0-7409e2455894", Rel: placementPolicyEntityRel},
		{ExtID: "1cf2e1a8-7a9c-4c99-8ff8-cb3b0d4b3b78", Rel: "vmm:content:image"},
	}})
	if err != nil {
		t.Fatalf("PlacementPolicyIDFromTask() error = %v", err)
	}
	if got != "7ccae44f-d067-4d49-a4d0-7409e2455894" {
		t.Fatalf("PlacementPolicyIDFromTask() = %q", got)
	}
}

func TestPlacementPolicyIDFromTaskFailsClosed(t *testing.T) {
	tests := map[string]task.Snapshot{
		"missing":      {EntitiesAffected: []task.EntityReference{{ExtID: "7ccae44f-d067-4d49-a4d0-7409e2455894", Rel: "vmm:content:image"}}},
		"invalid uuid": {EntitiesAffected: []task.EntityReference{{ExtID: "not-a-uuid", Rel: placementPolicyEntityRel}}},
		"duplicate": {EntitiesAffected: []task.EntityReference{
			{ExtID: "7ccae44f-d067-4d49-a4d0-7409e2455894", Rel: placementPolicyEntityRel},
			{ExtID: "1cf2e1a8-7a9c-4c99-8ff8-cb3b0d4b3b78", Rel: placementPolicyEntityRel},
		}},
	}
	for name, snapshot := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := PlacementPolicyIDFromTask(snapshot); err != ErrPlacementPolicyIdentityMissing {
				t.Fatalf("PlacementPolicyIDFromTask() error = %v", err)
			}
		})
	}
}

func TestValidatePlacementPolicySpec(t *testing.T) {
	valid := PlacementPolicySpec{
		Name:          "images",
		PlacementType: "SOFT",
		ImageEntityFilter: CategoryFilter{
			Type: "CATEGORIES_MATCH_ALL", CategoryExtIDs: []string{"7ccae44f-d067-4d49-a4d0-7409e2455894"},
		},
		ClusterEntityFilter: CategoryFilter{
			Type: "CATEGORIES_MATCH_ANY", CategoryExtIDs: []string{"1cf2e1a8-7a9c-4c99-8ff8-cb3b0d4b3b78"},
		},
	}
	if err := validatePlacementPolicySpec(valid); err != nil {
		t.Fatalf("validatePlacementPolicySpec(valid) error = %v", err)
	}
	valid.ImageEntityFilter.CategoryExtIDs = []string{"not-a-uuid"}
	if err := validatePlacementPolicySpec(valid); err != ErrInvalidPlacementPolicySpec {
		t.Fatalf("validatePlacementPolicySpec(invalid) error = %v", err)
	}
}
