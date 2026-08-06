// Package networking implements hand-written Networking API operations.
package networking

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/google/uuid"

	"github.com/ioplane/terraform-provider-nutanix/internal/nutanix/apiresponse"
	"github.com/ioplane/terraform-provider-nutanix/internal/transport"
)

const getSubnetByIDPath = "/api/networking/v4.3/config/subnets/{extId}"

var (
	// ErrMissingClient identifies an absent transport dependency.
	ErrMissingClient = errors.New("networking client is missing")
	// ErrInvalidSubnetID identifies a request without a valid subnet UUID.
	ErrInvalidSubnetID = errors.New("networking subnet identity is invalid")
	// ErrInvalidSubnet identifies a response with a missing or different subnet identity.
	ErrInvalidSubnet = errors.New("networking subnet response is invalid")
)

type executor interface {
	Execute(context.Context, transport.Request) (transport.Response, error)
}

// Client owns the Networking operation policies over one transport client.
type Client struct {
	executor executor
}

// NewClient constructs a Networking client without making a network request.
func NewClient(transportClient *transport.Client) (*Client, error) {
	if transportClient == nil {
		return nil, ErrMissingClient
	}
	return &Client{executor: transportClient}, nil
}

// GetSubnetByID returns one subnet whose identity matches the requested UUID.
func (c *Client) GetSubnetByID(ctx context.Context, extID string) (Subnet, error) {
	if c == nil || c.executor == nil {
		return Subnet{}, ErrMissingClient
	}
	requestedID, err := uuid.Parse(extID)
	if err != nil {
		return Subnet{}, ErrInvalidSubnetID
	}
	request, err := transport.NewRequest(transport.RequestOptions{
		Operation:        "getSubnetById",
		Method:           http.MethodGet,
		PathTemplate:     getSubnetByIDPath,
		PathParameters:   map[string]string{"extId": extID},
		ExpectedStatuses: []int{http.StatusOK},
		RetryClass:       transport.RetryRead,
	})
	if err != nil {
		return Subnet{}, fmt.Errorf("build getSubnetById request: %w", err)
	}
	response, err := c.executor.Execute(ctx, request)
	if err != nil {
		return Subnet{}, fmt.Errorf("execute getSubnetById: %w", err)
	}
	subnet, err := apiresponse.DecodeEntity[Subnet](response.Body())
	if err != nil {
		return Subnet{}, fmt.Errorf("decode getSubnetById response: %w", err)
	}
	if err := validateSubnetIdentity(subnet, requestedID); err != nil {
		return Subnet{}, err
	}
	return subnet, nil
}

func validateSubnetIdentity(subnet Subnet, requestedID uuid.UUID) error {
	if subnet.ExtID == nil {
		return ErrInvalidSubnet
	}
	responseID, err := uuid.Parse(*subnet.ExtID)
	if err != nil || responseID != requestedID {
		return ErrInvalidSubnet
	}
	return nil
}
