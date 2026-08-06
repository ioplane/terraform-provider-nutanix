// Package prism implements hand-written Prism API operations.
package prism

import (
	"context"
	"encoding/json"
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

const categoryPath = listCategoriesPath + "/{extId}"

var (
	// ErrMissingClient identifies an absent transport dependency.
	ErrMissingClient = errors.New("prism client is missing")
	// ErrInvalidCategory identifies a category response without its required identity.
	ErrInvalidCategory = errors.New("prism category response is invalid")
	// ErrCategoryNotFound identifies a category that no longer exists.
	ErrCategoryNotFound = errors.New("prism category was not found")

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

// GetCategory returns one category and its opaque ETag for conditional updates.
func (c *Client) GetCategory(ctx context.Context, extID string) (Category, string, error) {
	if c == nil || c.executor == nil {
		return Category{}, "", ErrMissingClient
	}
	request, err := transport.NewRequest(transport.RequestOptions{
		Operation:        "getCategoryById",
		Method:           http.MethodGet,
		PathTemplate:     categoryPath,
		PathParameters:   map[string]string{"extId": extID},
		ExpectedStatuses: []int{http.StatusOK},
		RetryClass:       transport.RetryRead,
	})
	if err != nil {
		return Category{}, "", fmt.Errorf("build getCategoryById request: %w", err)
	}
	response, err := c.executor.Execute(ctx, request)
	if err != nil {
		if isHTTPStatus(err, http.StatusNotFound) {
			return Category{}, "", ErrCategoryNotFound
		}
		return Category{}, "", fmt.Errorf("execute getCategoryById: %w", err)
	}
	category, err := apiresponse.DecodeEntity[Category](response.Body())
	if err != nil {
		return Category{}, "", fmt.Errorf("decode getCategoryById response: %w", err)
	}
	if err := validateCategory(category); err != nil {
		return Category{}, "", err
	}
	return category, response.ETag(), nil
}

// CreateCategory creates a user-defined category and returns the server entity.
func (c *Client) CreateCategory(ctx context.Context, spec CategorySpec) (Category, error) {
	if c == nil || c.executor == nil {
		return Category{}, ErrMissingClient
	}
	body, err := json.Marshal(spec)
	if err != nil {
		return Category{}, fmt.Errorf("encode createCategory request: %w", err)
	}
	request, err := transport.NewRequest(transport.RequestOptions{
		Operation:        "createCategory",
		Method:           http.MethodPost,
		PathTemplate:     listCategoriesPath,
		JSONBody:         body,
		ExpectedStatuses: []int{http.StatusCreated},
		RetryClass:       transport.RetryNone,
	})
	if err != nil {
		return Category{}, fmt.Errorf("build createCategory request: %w", err)
	}
	response, err := c.executor.Execute(ctx, request)
	if err != nil {
		return Category{}, fmt.Errorf("execute createCategory: %w", err)
	}
	category, err := apiresponse.DecodeEntity[Category](response.Body())
	if err != nil {
		return Category{}, fmt.Errorf("decode createCategory response: %w", err)
	}
	if err := validateCategory(category); err != nil {
		return Category{}, err
	}
	return category, nil
}

// UpdateCategory updates a category with the ETag returned by the preceding read.
func (c *Client) UpdateCategory(ctx context.Context, extID, etag string, spec CategorySpec) error {
	if c == nil || c.executor == nil {
		return ErrMissingClient
	}
	body, err := json.Marshal(spec)
	if err != nil {
		return fmt.Errorf("encode updateCategoryById request: %w", err)
	}
	request, err := transport.NewRequest(transport.RequestOptions{
		Operation:        "updateCategoryById",
		Method:           http.MethodPut,
		PathTemplate:     categoryPath,
		PathParameters:   map[string]string{"extId": extID},
		JSONBody:         body,
		ExpectedStatuses: []int{http.StatusOK},
		RetryClass:       transport.RetryNone,
		IfMatchRequired:  true,
		IfMatchETag:      etag,
	})
	if err != nil {
		return fmt.Errorf("build updateCategoryById request: %w", err)
	}
	if _, err := c.executor.Execute(ctx, request); err != nil {
		return fmt.Errorf("execute updateCategoryById: %w", err)
	}
	return nil
}

// DeleteCategory deletes a user-defined category by its external identifier.
func (c *Client) DeleteCategory(ctx context.Context, extID string) error {
	if c == nil || c.executor == nil {
		return ErrMissingClient
	}
	request, err := transport.NewRequest(transport.RequestOptions{
		Operation:        "deleteCategoryById",
		Method:           http.MethodDelete,
		PathTemplate:     categoryPath,
		PathParameters:   map[string]string{"extId": extID},
		ExpectedStatuses: []int{http.StatusNoContent},
		RetryClass:       transport.RetryNone,
	})
	if err != nil {
		return fmt.Errorf("build deleteCategoryById request: %w", err)
	}
	if _, err := c.executor.Execute(ctx, request); err != nil {
		if isHTTPStatus(err, http.StatusNotFound) {
			return ErrCategoryNotFound
		}
		return fmt.Errorf("execute deleteCategoryById: %w", err)
	}
	return nil
}

func validateCategories(categories []Category) error {
	for index := range categories {
		if err := validateCategory(categories[index]); err != nil {
			return ErrInvalidCategory
		}
	}
	return nil
}

func validateCategory(category Category) error {
	if category.ExtID == nil || category.Key == nil || category.Value == nil {
		return ErrInvalidCategory
	}
	if _, err := uuid.Parse(*category.ExtID); err != nil {
		return ErrInvalidCategory
	}
	return nil
}

func isHTTPStatus(err error, status int) bool {
	var httpError *transport.HTTPError
	return errors.As(err, &httpError) && httpError.StatusCode() == status
}
