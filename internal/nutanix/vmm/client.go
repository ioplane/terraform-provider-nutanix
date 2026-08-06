// Package vmm implements hand-written Virtual Machine Management API operations.
package vmm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/google/uuid"

	"github.com/ioplane/terraform-provider-nutanix/internal/nutanix/apiresponse"
	"github.com/ioplane/terraform-provider-nutanix/internal/nutanix/odata"
	"github.com/ioplane/terraform-provider-nutanix/internal/task"
	"github.com/ioplane/terraform-provider-nutanix/internal/transport"
)

const (
	listImagesPath            = "/api/vmm/v4.2/content/images"
	listPlacementPoliciesPath = "/api/vmm/v4.2/images/config/placement-policies"
	placementPolicyPath       = listPlacementPoliciesPath + "/{extId}"
	placementPolicyEntityRel  = "vmm:images:config:placement-policy"
)

var (
	// ErrMissingClient identifies an absent transport dependency.
	ErrMissingClient = errors.New("vmm client is missing")
	// ErrInvalidImage identifies an image response without its required identity.
	ErrInvalidImage = errors.New("vmm image response is invalid")
	// ErrInvalidPlacementPolicy identifies a policy response without its required identity.
	ErrInvalidPlacementPolicy = errors.New("vmm placement policy response is invalid")
	// ErrPlacementPolicyNotFound identifies a policy absent from the remote system.
	ErrPlacementPolicyNotFound = errors.New("vmm placement policy was not found")
	// ErrMissingTaskWaiter identifies an async client without a task dependency.
	ErrMissingTaskWaiter = errors.New("vmm task waiter is missing")
	// ErrInvalidPlacementPolicySpec identifies a write payload outside the API contract.
	ErrInvalidPlacementPolicySpec = errors.New("vmm placement policy specification is invalid")
	// ErrInvalidAsyncResponse identifies a malformed asynchronous response.
	ErrInvalidAsyncResponse = errors.New("vmm asynchronous response is invalid")
	// ErrPlacementPolicyIdentityMissing identifies a task without a policy entity.
	ErrPlacementPolicyIdentityMissing = errors.New("vmm placement policy identity is missing from task")

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
	waiter   TaskWaiter
}

// NewClientWithTaskWaiter constructs a VMM client with optional task polling.
func NewClientWithTaskWaiter(transportClient *transport.Client, waiter TaskWaiter) (*Client, error) {
	if transportClient == nil {
		return nil, ErrMissingClient
	}
	return &Client{executor: transportClient, waiter: waiter}, nil
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

// GetPlacementPolicyByID returns one policy and its opaque ETag.
func (c *Client) GetPlacementPolicyByID(ctx context.Context, extID string) (PlacementPolicyRead, error) {
	if c == nil || c.executor == nil {
		return PlacementPolicyRead{}, ErrMissingClient
	}
	requestedID, err := uuid.Parse(extID)
	if err != nil {
		return PlacementPolicyRead{}, ErrInvalidPlacementPolicy
	}
	request, err := transport.NewRequest(transport.RequestOptions{
		Operation:        "getPlacementPolicyById",
		Method:           http.MethodGet,
		PathTemplate:     placementPolicyPath,
		PathParameters:   map[string]string{"extId": requestedID.String()},
		ExpectedStatuses: []int{http.StatusOK},
		RetryClass:       transport.RetryRead,
	})
	if err != nil {
		return PlacementPolicyRead{}, fmt.Errorf("build getPlacementPolicyById request: %w", err)
	}
	response, err := c.executor.Execute(ctx, request)
	if err != nil {
		if isHTTPStatus(err, http.StatusNotFound) {
			return PlacementPolicyRead{}, ErrPlacementPolicyNotFound
		}
		return PlacementPolicyRead{}, fmt.Errorf("execute getPlacementPolicyById: %w", err)
	}
	policy, err := apiresponse.DecodeEntity[PlacementPolicy](response.Body())
	if err != nil {
		return PlacementPolicyRead{}, fmt.Errorf("decode getPlacementPolicyById response: %w", err)
	}
	if err := validatePlacementPolicyIdentity(policy, requestedID); err != nil {
		return PlacementPolicyRead{}, err
	}
	return PlacementPolicyRead{Policy: policy, ETag: response.ETag()}, nil
}

// CreatePlacementPolicy submits the asynchronous VMM create operation.
func (c *Client) CreatePlacementPolicy(ctx context.Context, spec PlacementPolicySpec) (AsyncOperation, error) {
	return c.submitPlacementPolicyMutation(ctx, "createPlacementPolicy", http.MethodPost, listPlacementPoliciesPath, "", spec, "", true)
}

// UpdatePlacementPolicyByID submits a conditional asynchronous update.
func (c *Client) UpdatePlacementPolicyByID(ctx context.Context, extID string, spec PlacementPolicySpec, etag string) (AsyncOperation, error) {
	return c.submitPlacementPolicyMutation(ctx, "updatePlacementPolicyById", http.MethodPut, placementPolicyPath, extID, spec, etag, true)
}

// DeletePlacementPolicyByID submits the asynchronous delete operation.
func (c *Client) DeletePlacementPolicyByID(ctx context.Context, extID string) (AsyncOperation, error) {
	return c.submitPlacementPolicyMutation(ctx, "deletePlacementPolicyById", http.MethodDelete, placementPolicyPath, extID, nil, "", false)
}

// WaitPlacementPolicyTask waits through the shared Prism task waiter.
func (c *Client) WaitPlacementPolicyTask(ctx context.Context, operation AsyncOperation) (task.Snapshot, error) {
	if c == nil || c.executor == nil {
		return task.Snapshot{}, ErrMissingClient
	}
	if c.waiter == nil {
		return task.Snapshot{}, ErrMissingTaskWaiter
	}
	if operation.TaskID == "" {
		return task.Snapshot{}, ErrInvalidAsyncResponse
	}
	return c.waiter.Wait(ctx, operation.TaskID)
}

// PlacementPolicyIDFromTask returns exactly one policy entity from a completed task.
func PlacementPolicyIDFromTask(snapshot task.Snapshot) (string, error) {
	policyID, err := task.EntityIDByRelation(snapshot, placementPolicyEntityRel)
	if err != nil {
		return "", ErrPlacementPolicyIdentityMissing
	}
	return policyID, nil
}

func (c *Client) submitPlacementPolicyMutation(ctx context.Context, operation, method, pathTemplate, extID string, spec any, etag string, requestIDRequired bool) (AsyncOperation, error) {
	if c == nil || c.executor == nil {
		return AsyncOperation{}, ErrMissingClient
	}
	if ctx == nil {
		return AsyncOperation{}, ErrInvalidPlacementPolicySpec
	}
	parameters := map[string]string{}
	if extID != "" {
		requestedID, err := uuid.Parse(extID)
		if err != nil {
			return AsyncOperation{}, ErrInvalidPlacementPolicy
		}
		parameters["extId"] = requestedID.String()
	}
	if method != http.MethodDelete {
		selected, ok := spec.(PlacementPolicySpec)
		if !ok || validatePlacementPolicySpec(selected) != nil {
			return AsyncOperation{}, ErrInvalidPlacementPolicySpec
		}
	}
	var body []byte
	if spec != nil {
		var err error
		body, err = json.Marshal(spec)
		if err != nil {
			return AsyncOperation{}, fmt.Errorf("encode %s request: %w", operation, err)
		}
	}
	request, err := transport.NewRequest(transport.RequestOptions{
		Operation:         operation,
		Method:            method,
		PathTemplate:      pathTemplate,
		PathParameters:    parameters,
		Headers:           http.Header{"Content-Type": {"application/json"}},
		JSONBody:          body,
		ExpectedStatuses:  []int{http.StatusAccepted},
		RetryClass:        retryClassForPlacementMutation(requestIDRequired),
		RequestIDRequired: requestIDRequired,
		Replayable:        requestIDRequired,
		IfMatchRequired:   method == http.MethodPut,
		IfMatchETag:       etag,
	})
	if err != nil {
		return AsyncOperation{}, fmt.Errorf("build %s request: %w", operation, err)
	}
	response, err := c.executor.Execute(ctx, request)
	if err != nil {
		if isHTTPStatus(err, http.StatusNotFound) {
			return AsyncOperation{}, ErrPlacementPolicyNotFound
		}
		return AsyncOperation{}, fmt.Errorf("execute %s: %w", operation, err)
	}
	return decodePlacementPolicyAsyncOperation(response)
}

func retryClassForPlacementMutation(required bool) transport.RetryClass {
	if required {
		return transport.RetryIdempotentMutation
	}
	return transport.RetryNone
}

type placementPolicyTaskReference struct {
	ExtID string `json:"extId"`
}

func decodePlacementPolicyAsyncOperation(response transport.Response) (AsyncOperation, error) {
	if strings.TrimSpace(response.Headers().Get("Location")) == "" {
		return AsyncOperation{}, ErrInvalidAsyncResponse
	}
	var envelope struct {
		Data *placementPolicyTaskReference `json:"data"`
	}
	decoder := json.NewDecoder(bytes.NewReader(response.Body()))
	if err := decoder.Decode(&envelope); err != nil || envelope.Data == nil || envelope.Data.ExtID == "" {
		return AsyncOperation{}, ErrInvalidAsyncResponse
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return AsyncOperation{}, ErrInvalidAsyncResponse
	}
	if _, err := url.ParseRequestURI(response.Headers().Get("Location")); err != nil {
		return AsyncOperation{}, ErrInvalidAsyncResponse
	}
	return AsyncOperation{TaskID: envelope.Data.ExtID}, nil
}

func validatePlacementPolicySpec(spec PlacementPolicySpec) error {
	if len(spec.Name) < 1 || len(spec.Name) > 256 || (spec.PlacementType != "SOFT" && spec.PlacementType != "HARD") {
		return ErrInvalidPlacementPolicySpec
	}
	if spec.Description != nil && len(*spec.Description) > 1000 {
		return ErrInvalidPlacementPolicySpec
	}
	if !validCategoryFilter(spec.ImageEntityFilter) || !validCategoryFilter(spec.ClusterEntityFilter) {
		return ErrInvalidPlacementPolicySpec
	}
	return nil
}

func validCategoryFilter(filter CategoryFilter) bool {
	if filter.Type != "CATEGORIES_MATCH_ALL" && filter.Type != "CATEGORIES_MATCH_ANY" {
		return false
	}
	if len(filter.CategoryExtIDs) < 1 || len(filter.CategoryExtIDs) > 100 {
		return false
	}
	for _, value := range filter.CategoryExtIDs {
		if _, err := uuid.Parse(value); err != nil {
			return false
		}
	}
	return true
}

func validatePlacementPolicyIdentity(policy PlacementPolicy, requestedID uuid.UUID) error {
	if policy.ExtID == nil || policy.Name == nil || policy.PlacementType == nil || policy.ImageEntityFilter == nil || policy.ClusterEntityFilter == nil {
		return ErrInvalidPlacementPolicy
	}
	responseID, err := uuid.Parse(*policy.ExtID)
	if err != nil || responseID != requestedID {
		return ErrInvalidPlacementPolicy
	}
	return nil
}

func isHTTPStatus(err error, status int) bool {
	var httpError *transport.HTTPError
	return errors.As(err, &httpError) && httpError.StatusCode() == status
}
