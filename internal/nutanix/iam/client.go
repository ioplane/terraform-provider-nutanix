package iam

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/google/uuid"

	"github.com/ioplane/terraform-provider-nutanix/internal/nutanix/apiresponse"
	"github.com/ioplane/terraform-provider-nutanix/internal/nutanix/odata"
	"github.com/ioplane/terraform-provider-nutanix/internal/transport"
)

const (
	listOperationsPath = "/api/iam/v4.0/authz/operations"
	listRolesPath      = "/api/iam/v4.0/authz/roles"
)

var (
	// ErrMissingClient identifies an absent transport dependency.
	ErrMissingClient = errors.New("iam client is missing")
	// ErrInvalidOperation identifies an operation response without a valid remote identity.
	ErrInvalidOperation = errors.New("iam operation response is invalid")
	// ErrInvalidRole identifies a role response without a valid remote identity.
	ErrInvalidRole = errors.New("iam role response is invalid")

	listOperationsPolicy = odata.Policy{
		RequiredSelect: []string{
			"associatedEndpointList",
			"clientName",
			"createdTime",
			"description",
			"displayName",
			"entityType",
			"extId",
			"lastUpdatedTime",
			"operationType",
			"relatedOperationList",
			"tenantId",
		},
	}
	listRolesPolicy = odata.Policy{
		RequiredSelect: []string{
			"accessibleClients",
			"accessibleClientsCount",
			"accessibleEntityTypes",
			"accessibleEntityTypesCount",
			"assignedUserGroupsCount",
			"assignedUsersCount",
			"clientName",
			"createdBy",
			"createdTime",
			"description",
			"displayName",
			"extId",
			"isSystemDefined",
			"lastUpdatedTime",
			"operations",
			"tenantId",
		},
	}
)

type executor interface {
	Execute(context.Context, transport.Request) (transport.Response, error)
}

// Client owns IAM v4.0 operation policies over one transport client.
type Client struct {
	executor executor
}

// NewClient constructs an IAM client without making a network request.
func NewClient(transportClient *transport.Client) (*Client, error) {
	if transportClient == nil {
		return nil, ErrMissingClient
	}
	return &Client{executor: transportClient}, nil
}

// ListOperations returns validated IAM operations and the caller-only query identity.
func (c *Client) ListOperations(
	ctx context.Context,
	options odata.ListOptions,
) ([]Operation, url.Values, error) {
	return list[Operation](
		ctx,
		c,
		options,
		"listOperations",
		listOperationsPath,
		listOperationsPolicy,
		validateOperations,
	)
}

// ListRoles returns validated IAM roles and the caller-only query identity.
func (c *Client) ListRoles(
	ctx context.Context,
	options odata.ListOptions,
) ([]Role, url.Values, error) {
	return list[Role](
		ctx,
		c,
		options,
		"listRoles",
		listRolesPath,
		listRolesPolicy,
		validateRoles,
	)
}

func list[T any](
	ctx context.Context,
	client *Client,
	options odata.ListOptions,
	operation string,
	path string,
	policy odata.Policy,
	validate func([]T) error,
) ([]T, url.Values, error) {
	if client == nil || client.executor == nil {
		return nil, nil, ErrMissingClient
	}
	query, err := odata.Build(options, policy)
	if err != nil {
		return nil, nil, fmt.Errorf("build %s query: %w", operation, err)
	}
	request, err := transport.NewRequest(transport.RequestOptions{
		Operation:        operation,
		Method:           http.MethodGet,
		PathTemplate:     path,
		Query:            query.Values(),
		ExpectedStatuses: []int{http.StatusOK},
		RetryClass:       transport.RetryRead,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("build %s request: %w", operation, err)
	}
	response, err := client.executor.Execute(ctx, request)
	if err != nil {
		return nil, nil, fmt.Errorf("execute %s: %w", operation, err)
	}
	entities, err := apiresponse.DecodeList[T](response.Body())
	if err != nil {
		return nil, nil, fmt.Errorf("decode %s response: %w", operation, err)
	}
	if err := validate(entities); err != nil {
		return nil, nil, err
	}
	return entities, query.IdentityValues(), nil
}

func validateOperations(operations []Operation) error {
	for index := range operations {
		if operations[index].ExtID == nil {
			return ErrInvalidOperation
		}
		if _, err := uuid.Parse(*operations[index].ExtID); err != nil {
			return ErrInvalidOperation
		}
	}
	return nil
}

func validateRoles(roles []Role) error {
	for index := range roles {
		if roles[index].ExtID == nil {
			return ErrInvalidRole
		}
		if _, err := uuid.Parse(*roles[index].ExtID); err != nil {
			return ErrInvalidRole
		}
	}
	return nil
}
