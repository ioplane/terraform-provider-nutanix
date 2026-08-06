package networking

import (
	"errors"
	"testing"

	"github.com/ioplane/terraform-provider-nutanix/internal/task"
)

func TestSubnetIDFromTaskSelectsValidatedSubnetEntity(t *testing.T) {
	got, err := SubnetIDFromTask(task.Snapshot{EntitiesAffected: []task.EntityReference{
		{ExtID: "d4f9d6a1-4bb4-4d84-ae31-5c0a32bb4a72", Rel: "networking:config:vpc"},
		{ExtID: "7ccae44f-d067-4d49-a4d0-7409e2455894", Rel: "networking:config:subnet"},
	}})
	if err != nil {
		t.Fatalf("SubnetIDFromTask() error = %v", err)
	}
	if got != "7ccae44f-d067-4d49-a4d0-7409e2455894" {
		t.Fatalf("SubnetIDFromTask() = %q", got)
	}
}

func TestSubnetIDFromTaskFailsClosed(t *testing.T) {
	t.Parallel()

	for name, snapshot := range map[string]task.Snapshot{
		"missing":      {EntitiesAffected: []task.EntityReference{{ExtID: "7ccae44f-d067-4d49-a4d0-7409e2455894", Rel: "networking:config:vpc"}}},
		"invalid uuid": {EntitiesAffected: []task.EntityReference{{ExtID: "not-a-uuid", Rel: "networking:config:subnet"}}},
		"duplicate": {EntitiesAffected: []task.EntityReference{
			{ExtID: "7ccae44f-d067-4d49-a4d0-7409e2455894", Rel: "networking:config:subnet"},
			{ExtID: "d4f9d6a1-4bb4-4d84-ae31-5c0a32bb4a72", Rel: "networking:config:subnet"},
		}},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := SubnetIDFromTask(snapshot)
			if !errors.Is(err, ErrSubnetIdentityMissing) {
				t.Fatalf("error = %v, want %v", err, ErrSubnetIdentityMissing)
			}
		})
	}
}
