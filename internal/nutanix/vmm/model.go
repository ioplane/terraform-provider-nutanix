package vmm

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
