// Package vmm implements hand-written Virtual Machine Management API operations.
package vmm

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

const listImagesPath = "/api/vmm/v4.2/content/images"

var (
	// ErrMissingClient identifies an absent transport dependency.
	ErrMissingClient = errors.New("vmm client is missing")
	// ErrInvalidImage identifies an image response without its required identity.
	ErrInvalidImage = errors.New("vmm image response is invalid")

	listImagesPolicy = odata.Policy{
		RequiredSelect: []string{
			"categoryExtIds",
			"checksum",
			"clusterLocationExtIds",
			"createTime",
			"description",
			"extId",
			"lastUpdateTime",
			"name",
			"ownerExtId",
			"ownerName",
			"placementPolicyStatus",
			"sizeBytes",
			"type",
		},
	}
)

type executor interface {
	Execute(context.Context, transport.Request) (transport.Response, error)
}

// Client owns the VMM operation policies over one transport client.
type Client struct {
	executor executor
}

// NewClient constructs a VMM client without making a network request.
func NewClient(transportClient *transport.Client) (*Client, error) {
	if transportClient == nil {
		return nil, ErrMissingClient
	}
	return &Client{executor: transportClient}, nil
}

// ListImages returns validated images and the caller-only query identity.
func (c *Client) ListImages(
	ctx context.Context,
	options odata.ListOptions,
) ([]Image, url.Values, error) {
	if c == nil || c.executor == nil {
		return nil, nil, ErrMissingClient
	}

	query, err := odata.Build(options, listImagesPolicy)
	if err != nil {
		return nil, nil, fmt.Errorf("build listImages query: %w", err)
	}
	request, err := transport.NewRequest(transport.RequestOptions{
		Operation:        "listImages",
		Method:           http.MethodGet,
		PathTemplate:     listImagesPath,
		Query:            query.Values(),
		ExpectedStatuses: []int{http.StatusOK},
		RetryClass:       transport.RetryRead,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("build listImages request: %w", err)
	}
	response, err := c.executor.Execute(ctx, request)
	if err != nil {
		return nil, nil, fmt.Errorf("execute listImages: %w", err)
	}
	images, err := apiresponse.DecodeList[Image](response.Body())
	if err != nil {
		return nil, nil, fmt.Errorf("decode listImages response: %w", err)
	}
	if err := validateImages(images); err != nil {
		return nil, nil, err
	}
	return images, query.IdentityValues(), nil
}

func validateImages(images []Image) error {
	for index := range images {
		image := images[index]
		if image.ExtID == nil || image.Name == nil || image.Type == nil {
			return ErrInvalidImage
		}
		if _, err := uuid.Parse(*image.ExtID); err != nil {
			return ErrInvalidImage
		}
	}
	return nil
}
