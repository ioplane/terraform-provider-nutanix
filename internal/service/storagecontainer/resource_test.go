package storagecontainer

import (
	"testing"

	"github.com/ioplane/terraform-provider-nutanix/internal/nutanix/clustermgmt"
	"github.com/ioplane/terraform-provider-nutanix/internal/service/testkit"
)

func TestResourceContract(t *testing.T) {
	required := testkit.AttributeFlags{Required: true}
	optional := testkit.AttributeFlags{Optional: true}
	optionalComputed := testkit.AttributeFlags{Optional: true, Computed: true}
	computed := testkit.AttributeFlags{Computed: true}
	testkit.AssertResourceContract(t, NewResource(), "nutanix_storage_container", map[string]testkit.AttributeFlags{
		"id": computed, "ext_id": computed, "cluster_ext_id": required, "container_ext_id": computed, "owner_ext_id": computed,
		"name": required, "storage_pool_ext_id": computed, "is_marked_for_removal": computed, "max_capacity_bytes": computed,
		"logical_explicit_reserved_capacity_bytes": optionalComputed, "logical_implicit_reserved_capacity_bytes": computed,
		"logical_advertised_capacity_bytes": optionalComputed, "replication_factor": optionalComputed, "is_nfs_whitelist_inherited": computed,
		"erasure_code": optionalComputed, "is_inline_ec_enabled": optionalComputed, "has_higher_ec_fault_domain_preference": optionalComputed,
		"erasure_code_delay_secs": optionalComputed, "cache_deduplication": optionalComputed, "on_disk_dedup": optionalComputed,
		"is_compression_enabled": optionalComputed, "compression_delay_secs": optionalComputed, "is_internal": computed,
		"is_software_encryption_enabled": optionalComputed, "is_encrypted": computed, "affinity_host_ext_id": optionalComputed,
		"cluster_name": computed, "is_shared": optionalComputed, "external_storage_ext_id": computed, "ignore_small_files": optional,
	})
}

func TestStateFromRemoteRejectsMissingIdentity(t *testing.T) {
	if _, err := stateFromRemote(resourceModel{}, clustermgmt.StorageContainer{}); err == nil {
		t.Fatal("stateFromRemote() error = nil, want invalid storage container")
	}
}
