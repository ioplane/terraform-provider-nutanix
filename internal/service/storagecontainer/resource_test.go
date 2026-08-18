package storagecontainer

import (
	"testing"

	"github.com/ioplane/terraform-provider-nutanix/internal/nutanix/clustermgmt"
	"github.com/ioplane/terraform-provider-nutanix/internal/service/testkit"
)

func TestResourceContract(t *testing.T) {
	testkit.AssertResourceContract(t, NewResource(), "nutanix_storage_container", map[string]testkit.AttributeFlags{
		"id": testkit.Computed("StringAttribute"), "ext_id": testkit.Computed("StringAttribute"), "cluster_ext_id": testkit.Required("StringAttribute"),
		"container_ext_id": testkit.Computed("StringAttribute"), "owner_ext_id": testkit.Computed("StringAttribute"), "name": testkit.Required("StringAttribute"),
		"storage_pool_ext_id": testkit.Computed("StringAttribute"), "is_marked_for_removal": testkit.Computed("BoolAttribute"), "max_capacity_bytes": testkit.Computed("Int64Attribute"),
		"logical_explicit_reserved_capacity_bytes": testkit.OptionalComputed("Int64Attribute"), "logical_implicit_reserved_capacity_bytes": testkit.Computed("Int64Attribute"),
		"logical_advertised_capacity_bytes": testkit.OptionalComputed("Int64Attribute"), "replication_factor": testkit.OptionalComputed("Int32Attribute"), "is_nfs_whitelist_inherited": testkit.Computed("BoolAttribute"),
		"erasure_code": testkit.OptionalComputed("StringAttribute"), "is_inline_ec_enabled": testkit.OptionalComputed("BoolAttribute"), "has_higher_ec_fault_domain_preference": testkit.OptionalComputed("BoolAttribute"),
		"erasure_code_delay_secs": testkit.OptionalComputed("Int32Attribute"), "cache_deduplication": testkit.OptionalComputed("StringAttribute"), "on_disk_dedup": testkit.OptionalComputed("StringAttribute"),
		"is_compression_enabled": testkit.OptionalComputed("BoolAttribute"), "compression_delay_secs": testkit.OptionalComputed("Int32Attribute"), "is_internal": testkit.Computed("BoolAttribute"),
		"is_software_encryption_enabled": testkit.OptionalComputed("BoolAttribute"), "is_encrypted": testkit.Computed("BoolAttribute"), "affinity_host_ext_id": testkit.OptionalComputed("StringAttribute"),
		"cluster_name": testkit.Computed("StringAttribute"), "is_shared": testkit.OptionalComputed("BoolAttribute"), "external_storage_ext_id": testkit.Computed("StringAttribute"),
		"ignore_small_files": testkit.Optional("BoolAttribute"),
	})
}

func TestStateFromRemoteRejectsMissingIdentity(t *testing.T) {
	if _, err := stateFromRemote(resourceModel{}, clustermgmt.StorageContainer{}); err == nil {
		t.Fatal("stateFromRemote() error = nil, want invalid storage container")
	}
}
