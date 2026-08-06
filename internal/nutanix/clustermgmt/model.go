package clustermgmt

import (
	"context"

	"github.com/ioplane/terraform-provider-nutanix/internal/task"
)

// Cluster is the reviewed listClusters projection used by Terraform state mapping.
type Cluster struct {
	ExtID                  *string   `json:"extId"`
	Name                   *string   `json:"name"`
	Categories             *[]string `json:"categories"`
	VMCount                *int64    `json:"vmCount"`
	InefficientVMCount     *int64    `json:"inefficientVmCount"`
	ContainerName          *string   `json:"containerName"`
	ClusterProfileExtID    *string   `json:"clusterProfileExtId"`
	BackupEligibilityScore *int64    `json:"backupEligibilityScore"`
}

// StorageContainer is the reviewed clustermgmt v4.2 storage-container
// projection used by Terraform state mapping.
type StorageContainer struct {
	ExtID                                *string `json:"extId"`
	ContainerExtID                       *string `json:"containerExtId"`
	OwnerExtID                           *string `json:"ownerExtId"`
	Name                                 *string `json:"name"`
	ClusterExtID                         *string `json:"clusterExtId"`
	StoragePoolExtID                     *string `json:"storagePoolExtId"`
	IsMarkedForRemoval                   *bool   `json:"isMarkedForRemoval"`
	MaxCapacityBytes                     *int64  `json:"maxCapacityBytes"`
	LogicalExplicitReservedCapacityBytes *int64  `json:"logicalExplicitReservedCapacityBytes"`
	LogicalImplicitReservedCapacityBytes *int64  `json:"logicalImplicitReservedCapacityBytes"`
	LogicalAdvertisedCapacityBytes       *int64  `json:"logicalAdvertisedCapacityBytes"`
	ReplicationFactor                    *int32  `json:"replicationFactor"`
	IsNFSWhitelistInherited              *bool   `json:"isNfsWhitelistInherited"`
	ErasureCode                          *string `json:"erasureCode"`
	IsInlineECEnabled                    *bool   `json:"isInlineEcEnabled"`
	HasHigherECFaultDomainPreference     *bool   `json:"hasHigherEcFaultDomainPreference"`
	ErasureCodeDelaySecs                 *int32  `json:"erasureCodeDelaySecs"`
	CacheDeduplication                   *string `json:"cacheDeduplication"`
	OnDiskDedup                          *string `json:"onDiskDedup"`
	IsCompressionEnabled                 *bool   `json:"isCompressionEnabled"`
	CompressionDelaySecs                 *int32  `json:"compressionDelaySecs"`
	IsInternal                           *bool   `json:"isInternal"`
	IsSoftwareEncryptionEnabled          *bool   `json:"isSoftwareEncryptionEnabled"`
	IsEncrypted                          *bool   `json:"isEncrypted"`
	AffinityHostExtID                    *string `json:"affinityHostExtId"`
	ClusterName                          *string `json:"clusterName"`
	IsShared                             *bool   `json:"isShared"`
	ExternalStorageExtID                 *string `json:"externalStorageExtId"`
}

// StorageContainerSpec contains only the reviewed mutable request fields.
// Read-only API projections are intentionally not reused in requests.
type StorageContainerSpec struct {
	Name                                 string  `json:"name"`
	LogicalExplicitReservedCapacityBytes *int64  `json:"logicalExplicitReservedCapacityBytes,omitempty"`
	LogicalAdvertisedCapacityBytes       *int64  `json:"logicalAdvertisedCapacityBytes,omitempty"`
	ReplicationFactor                    *int32  `json:"replicationFactor,omitempty"`
	ErasureCode                          *string `json:"erasureCode,omitempty"`
	IsInlineECEnabled                    *bool   `json:"isInlineEcEnabled,omitempty"`
	HasHigherECFaultDomainPreference     *bool   `json:"hasHigherEcFaultDomainPreference,omitempty"`
	ErasureCodeDelaySecs                 *int32  `json:"erasureCodeDelaySecs,omitempty"`
	CacheDeduplication                   *string `json:"cacheDeduplication,omitempty"`
	OnDiskDedup                          *string `json:"onDiskDedup,omitempty"`
	IsCompressionEnabled                 *bool   `json:"isCompressionEnabled,omitempty"`
	CompressionDelaySecs                 *int32  `json:"compressionDelaySecs,omitempty"`
	IsSoftwareEncryptionEnabled          *bool   `json:"isSoftwareEncryptionEnabled,omitempty"`
	AffinityHostExtID                    *string `json:"affinityHostExtId,omitempty"`
	IsShared                             *bool   `json:"isShared,omitempty"`
}

// StorageContainerRead is a validated storage-container projection plus its
// opaque response ETag.
type StorageContainerRead struct {
	StorageContainer StorageContainer
	ETag             string
}

// StorageContainerAsyncOperation aliases the shared asynchronous operation contract.
type StorageContainerAsyncOperation = task.AsyncOperation

// StorageContainerTaskWaiter is the reusable asynchronous task port.
type StorageContainerTaskWaiter interface {
	Wait(context.Context, string) (task.Snapshot, error)
}
