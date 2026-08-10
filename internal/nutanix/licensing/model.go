// Package licensing implements hand-written Licensing API operations.
package licensing

import "encoding/json"

// License is the reviewed Licensing v4.3 applied-license projection.
//
// The API exposes additional fields and relationships. This model deliberately
// keeps only the stable inventory fields selected by the first Terraform
// surface; consumption details are returned when the caller expands them.
type License struct {
	ExtID               *string        `json:"extId"`
	Category            *string        `json:"category"`
	ExpiryDate          *string        `json:"expiryDate"`
	Name                *string        `json:"name"`
	SubCategory         *string        `json:"subCategory"`
	Type                *string        `json:"type"`
	Meter               *string        `json:"meter"`
	Quantity            *float64       `json:"quantity"`
	Scope               *string        `json:"scope"`
	SalesforceLicenseID *string        `json:"salesforceLicenseId"`
	ConsumptionDetails  *[]Consumption `json:"consumptionDetails"`
}

// Consumption is the optional per-cluster usage expansion for a license.
type Consumption struct {
	ClusterExtID *string  `json:"clusterExtId"`
	QuantityUsed *float64 `json:"quantityUsed"`
}

// LicenseKey is the reviewed Licensing v4.3 license-key inventory projection.
// Assignment and association details are populated only when explicitly expanded.
type LicenseKey struct {
	ExtID                 *string                  `json:"extId"`
	TenantID              *string                  `json:"tenantId"`
	Key                   *string                  `json:"key"`
	ValidationDetail      *string                  `json:"validationDetail"`
	Type                  *string                  `json:"type"`
	Category              *string                  `json:"category"`
	SubCategory           *string                  `json:"subCategory"`
	EntitlementExpiryDate *string                  `json:"entitlementExpiryDate"`
	Meter                 *string                  `json:"meter"`
	Quantity              *float64                 `json:"quantity"`
	GroupID               *string                  `json:"groupId"`
	EnforcementPolicy     *string                  `json:"enforcementPolicy"`
	AssignmentDetails     *[]LicenseKeyMapping     `json:"assignmentDetails"`
	AssociationDetails    *[]LicenseKeyAssociation `json:"associationDetails"`
}

// LicenseKeyMapping is an expanded license-key assignment projection.
type LicenseKeyMapping struct {
	Key          *string  `json:"key"`
	QuantityUsed *float64 `json:"quantityUsed"`
	ClusterExtID *string  `json:"clusterExtId"`
}

// LicenseKeyAssociation is an expanded relationship between license keys.
type LicenseKeyAssociation struct {
	BaseKey         *string `json:"baseKey"`
	AssociatedKey   *string `json:"associatedKey"`
	AssociationType *string `json:"associationType"`
	ReclaimType     *string `json:"reclaimType"`
}

// Feature is the reviewed Licensing v4.3 feature inventory projection.
// Value is either a boolean or an integer according to the API contract.
type Feature struct {
	Name               *string         `json:"name"`
	ValueType          *string         `json:"valueType"`
	Value              json.RawMessage `json:"value"`
	LicenseType        *string         `json:"licenseType"`
	LicenseCategory    *string         `json:"licenseCategory"`
	LicenseSubCategory *string         `json:"licenseSubCategory"`
	Scope              *string         `json:"scope"`
}
