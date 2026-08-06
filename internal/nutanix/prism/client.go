// Package prism implements hand-written Prism API operations.
package prism

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

const listCategoriesPath = "/api/prism/v4.3/config/categories"

var (
	// ErrMissingClient identifies an absent transport dependency.
	ErrMissingClient = errors.New("prism client is missing")
	// ErrInvalidCategory identifies a category response without its required identity.
	ErrInvalidCategory = errors.New("prism category response is invalid")

	listCategoriesPolicy = odata.Policy{
		RequiredSelect: []string{
			"associations",
			"description",
			"detailedAssociations",
			"extId",
			"key",
			"ownerUuid",
			"type",
			"value",
		},
		RequiredExpand: []string{"associations", "detailedAssociations"},
		AllowExpand:    true,
	}
)

type executor interface {
	Execute(context.Context, transport.Request) (transport.Response, error)
}

// Client owns the Prism operation policies over one transport client.
type Client struct {
	executor executor
}

// NewClient constructs a Prism client without making a network request.
func NewClient(transportClient *transport.Client) (*Client, error) {
	if transportClient == nil {
		return nil, ErrMissingClient
	}
	return &Client{executor: transportClient}, nil
}

// ListCategories returns validated categories and the caller-only query identity.
func (c *Client) ListCategories(
	ctx context.Context,
	options odata.ListOptions,
) ([]Category, url.Values, error) {
	if c == nil || c.executor == nil {
		return nil, nil, ErrMissingClient
	}

	query, err := odata.Build(options, listCategoriesPolicy)
	if err != nil {
		return nil, nil, fmt.Errorf("build listCategories query: %w", err)
	}
	request, err := transport.NewRequest(transport.RequestOptions{
		Operation:        "listCategories",
		Method:           http.MethodGet,
		PathTemplate:     listCategoriesPath,
		Query:            query.Values(),
		ExpectedStatuses: []int{http.StatusOK},
		RetryClass:       transport.RetryRead,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("build listCategories request: %w", err)
	}
	response, err := c.executor.Execute(ctx, request)
	if err != nil {
		return nil, nil, fmt.Errorf("execute listCategories: %w", err)
	}
	categories, err := apiresponse.DecodeList[Category](response.Body())
	if err != nil {
		return nil, nil, fmt.Errorf("decode listCategories response: %w", err)
	}
	if err := validateCategories(categories); err != nil {
		return nil, nil, err
	}
	return categories, query.IdentityValues(), nil
}

func validateCategories(categories []Category) error {
	for index := range categories {
		category := categories[index]
		if category.ExtID == nil || category.Key == nil || category.Value == nil {
			return ErrInvalidCategory
		}
		if _, err := uuid.Parse(*category.ExtID); err != nil {
			return ErrInvalidCategory
		}
	}
	return nil
}
