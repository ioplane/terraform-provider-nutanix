package vmm

import (
	"context"

	"github.com/ioplane/terraform-provider-nutanix/internal/task"
)

// Image is the reviewed listImages projection used by Terraform state mapping.
type Image struct {
	ExtID                 *string                 `json:"extId"`
	Name                  *string                 `json:"name"`
	Description           *string                 `json:"description"`
	Type                  *string                 `json:"type"`
	Checksum              *Checksum               `json:"checksum"`
	SizeBytes             *int64                  `json:"sizeBytes"`
	CategoryExtIDs        *[]string               `json:"categoryExtIds"`
	ClusterLocationExtIDs *[]string               `json:"clusterLocationExtIds"`
	CreateTime            *string                 `json:"createTime"`
	LastUpdateTime        *string                 `json:"lastUpdateTime"`
	OwnerExtID            *string                 `json:"ownerExtId"`
	OwnerName             *string                 `json:"ownerName"`
	PlacementPolicyStatus *[]ImagePlacementStatus `json:"placementPolicyStatus"`
}

// Checksum is the common state-safe projection of either supported image checksum type.
type Checksum struct {
	HexDigest *string `json:"hexDigest"`
}

// ImagePlacementStatus is the reviewed image placement projection.
type ImagePlacementStatus struct {
	PlacementPolicyExtID    *string   `json:"placementPolicyExtId"`
	ComplianceStatus        *string   `json:"complianceStatus"`
	EnforcementMode         *string   `json:"enforcementMode"`
	PolicyClusterExtIDs     *[]string `json:"policyClusterExtIds"`
	EnforcedClusterExtIDs   *[]string `json:"enforcedClusterExtIds"`
	ConflictingPolicyExtIDs *[]string `json:"conflictingPolicyExtIds"`
}

// PlacementPolicy is the reviewed VMM v4.2 image placement policy projection.
type PlacementPolicy struct {
	ExtID               *string         `json:"extId"`
	Name                *string         `json:"name"`
	Description         *string         `json:"description"`
	PlacementType       *string         `json:"placementType"`
	ImageEntityFilter   *CategoryFilter `json:"imageEntityFilter"`
	ClusterEntityFilter *CategoryFilter `json:"clusterEntityFilter"`
	EnforcementState    *string         `json:"enforcementState"`
	CreateTime          *string         `json:"createTime"`
	LastUpdateTime      *string         `json:"lastUpdateTime"`
	OwnerExtID          *string         `json:"ownerExtId"`
	OwnerName           *string         `json:"ownerName"`
}

// CategoryFilter is the bounded category-based entity filter in the VMM API.
type CategoryFilter struct {
	Type           string   `json:"type"`
	CategoryExtIDs []string `json:"categoryExtIds"`
}

// PlacementPolicySpec contains the mutable placement-policy request fields.
type PlacementPolicySpec struct {
	Name                string         `json:"name"`
	Description         *string        `json:"description,omitempty"`
	PlacementType       string         `json:"placementType"`
	ImageEntityFilter   CategoryFilter `json:"imageEntityFilter"`
	ClusterEntityFilter CategoryFilter `json:"clusterEntityFilter"`
}

// PlacementPolicyRead is a validated projection plus the opaque response ETag.
type PlacementPolicyRead struct {
	Policy PlacementPolicy
	ETag   string
}

// AsyncOperation aliases the shared Prism task operation identity.
type AsyncOperation = task.AsyncOperation

// TaskWaiter is the shared asynchronous task port.
type TaskWaiter interface {
	Wait(context.Context, string) (task.Snapshot, error)
}
