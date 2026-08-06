// Package iam implements hand-written Identity and Access Management API operations.
package iam

// Role is the bounded IAM v4.0 role projection used by the read-only provider surface.
type Role struct {
	ExtID                      *string   `json:"extId"`
	TenantID                   *string   `json:"tenantId"`
	DisplayName                *string   `json:"displayName"`
	ClientName                 *string   `json:"clientName"`
	Description                *string   `json:"description"`
	Operations                 *[]string `json:"operations"`
	AccessibleClients          *[]string `json:"accessibleClients"`
	AccessibleEntityTypes      *[]string `json:"accessibleEntityTypes"`
	AccessibleClientsCount     *int64    `json:"accessibleClientsCount"`
	AccessibleEntityTypesCount *int64    `json:"accessibleEntityTypesCount"`
	AssignedUsersCount         *int64    `json:"assignedUsersCount"`
	AssignedUserGroupsCount    *int64    `json:"assignedUserGroupsCount"`
	CreatedTime                *string   `json:"createdTime"`
	LastUpdatedTime            *string   `json:"lastUpdatedTime"`
	CreatedBy                  *string   `json:"createdBy"`
	IsSystemDefined            *bool     `json:"isSystemDefined"`
}

// OperationEndpoint describes one endpoint associated with an IAM operation.
type OperationEndpoint struct {
	APIVersion  *string `json:"apiVersion"`
	EndpointURL *string `json:"endpointUrl"`
	HTTPMethod  *string `json:"httpMethod"`
}

// Operation is the bounded IAM v4.0 operation projection used by the read-only provider surface.
type Operation struct {
	ExtID                  *string              `json:"extId"`
	TenantID               *string              `json:"tenantId"`
	DisplayName            *string              `json:"displayName"`
	Description            *string              `json:"description"`
	EntityType             *string              `json:"entityType"`
	ClientName             *string              `json:"clientName"`
	CreatedTime            *string              `json:"createdTime"`
	LastUpdatedTime        *string              `json:"lastUpdatedTime"`
	OperationType          *string              `json:"operationType"`
	RelatedOperationList   *[]string            `json:"relatedOperationList"`
	AssociatedEndpointList *[]OperationEndpoint `json:"associatedEndpointList"`
}
