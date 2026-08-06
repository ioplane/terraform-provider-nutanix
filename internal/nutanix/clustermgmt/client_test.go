package clustermgmt

import (
	"testing"

	"github.com/ioplane/terraform-provider-nutanix/internal/task"
)

func TestStorageContainerIDFromTaskSelectsExactRelation(t *testing.T) {
	got, err := StorageContainerIDFromTask(task.Snapshot{EntitiesAffected: []task.EntityReference{
		{ExtID: "7ccae44f-d067-4d49-a4d0-7409e2455894", Rel: storageContainerEntityRel},
		{ExtID: "1cf2e1a8-7a9c-4c99-8ff8-cb3b0d4b3b78", Rel: "clustermgmt:config:clusters"},
	}})
	if err != nil {
		t.Fatalf("StorageContainerIDFromTask() error = %v", err)
	}
	if got != "7ccae44f-d067-4d49-a4d0-7409e2455894" {
		t.Fatalf("StorageContainerIDFromTask() = %q", got)
	}
}

func TestStorageContainerIDFromTaskFailsClosed(t *testing.T) {
	tests := map[string]task.Snapshot{
		"missing":      {EntitiesAffected: []task.EntityReference{{ExtID: "7ccae44f-d067-4d49-a4d0-7409e2455894", Rel: "clustermgmt:config:clusters"}}},
		"invalid uuid": {EntitiesAffected: []task.EntityReference{{ExtID: "not-a-uuid", Rel: storageContainerEntityRel}}},
		"duplicate": {EntitiesAffected: []task.EntityReference{
			{ExtID: "7ccae44f-d067-4d49-a4d0-7409e2455894", Rel: storageContainerEntityRel},
			{ExtID: "1cf2e1a8-7a9c-4c99-8ff8-cb3b0d4b3b78", Rel: storageContainerEntityRel},
		}},
	}
	for name, snapshot := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := StorageContainerIDFromTask(snapshot); err != ErrStorageContainerIdentityMissing {
				t.Fatalf("StorageContainerIDFromTask() error = %v", err)
			}
		})
	}
}
