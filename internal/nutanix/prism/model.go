package prism

// Category is the reviewed listCategories projection used by Terraform state mapping.
type Category struct {
	ExtID                *string               `json:"extId"`
	Key                  *string               `json:"key"`
	Value                *string               `json:"value"`
	Type                 *string               `json:"type"`
	Description          *string               `json:"description"`
	OwnerUUID            *string               `json:"ownerUuid"`
	Associations         *[]AssociationSummary `json:"associations"`
	DetailedAssociations *[]AssociationDetail  `json:"detailedAssociations"`
}

// CategorySpec is the writable category projection accepted by Prism.
// Read-only identity and association fields are intentionally excluded.
type CategorySpec struct {
	Key         string  `json:"key"`
	Value       string  `json:"value"`
	Description *string `json:"description,omitempty"`
	OwnerUUID   *string `json:"ownerUuid,omitempty"`
}

// AssociationSummary is the state-safe category association count projection.
type AssociationSummary struct {
	CategoryID    *string `json:"categoryId"`
	ResourceType  *string `json:"resourceType"`
	ResourceGroup *string `json:"resourceGroup"`
	Count         *int64  `json:"count"`
}

// AssociationDetail is the state-safe detailed category association projection.
type AssociationDetail struct {
	CategoryID    *string `json:"categoryId"`
	ResourceType  *string `json:"resourceType"`
	ResourceGroup *string `json:"resourceGroup"`
	ResourceID    *string `json:"resourceId"`
}
