// Package clustermgmt implements hand-written Cluster Management API operations.
package clustermgmt

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/ioplane/terraform-provider-nutanix/internal/nutanix/apiresponse"
	"github.com/ioplane/terraform-provider-nutanix/internal/nutanix/odata"
	"github.com/ioplane/terraform-provider-nutanix/internal/task"
	"github.com/ioplane/terraform-provider-nutanix/internal/transport"
)

const (
	listClustersPath          = "/api/clustermgmt/v4.2/config/clusters"
	listStorageContainersPath = "/api/clustermgmt/v4.2/config/storage-containers"
	storageContainerPath      = listStorageContainersPath + "/{extId}"
	storageContainerEntityRel = "clustermgmt:config:storage-containers"
)

var (
	// ErrMissingClient identifies an absent transport dependency.
	ErrMissingClient = errors.New("clustermgmt client is missing")
	// ErrInvalidCluster identifies a cluster response without a valid remote identity.
	ErrInvalidCluster = errors.New("clustermgmt cluster response is invalid")
	// ErrInvalidStorageContainer identifies a response without a valid remote identity.
	ErrInvalidStorageContainer = errors.New("clustermgmt storage container response is invalid")
	// ErrStorageContainerNotFound identifies a storage container that no longer exists.
	ErrStorageContainerNotFound = errors.New("clustermgmt storage container was not found")
	// ErrMissingTaskWaiter identifies an async client without a task dependency.
	ErrMissingTaskWaiter = errors.New("clustermgmt task waiter is missing")
	// ErrInvalidStorageContainerSpec identifies a write payload outside the reviewed API contract.
	ErrInvalidStorageContainerSpec = errors.New("clustermgmt storage container specification is invalid")
	// ErrInvalidAsyncResponse identifies a malformed asynchronous API response.
	ErrInvalidAsyncResponse = errors.New("clustermgmt asynchronous response is invalid")
	// ErrStorageContainerIdentityMissing identifies a successful task without a storage-container entity.
	ErrStorageContainerIdentityMissing = errors.New("clustermgmt storage container identity is missing from task")

	listClustersPolicy = odata.Policy{
		RequiredSelect: []string{
			"backupEligibilityScore",
			"categories",
			"clusterProfileExtId",
			"containerName",
			"extId",
			"inefficientVmCount",
			"name",
			"vmCount",
		},
		AllowExpand: true,
	}
)

type executor interface {
	Execute(context.Context, transport.Request) (transport.Response, error)
}

// Client owns the Cluster Management operation policies over one transport client.
type Client struct {
	executor executor
	waiter   StorageContainerTaskWaiter
}

// NewClientWithTaskWaiter constructs a Cluster Management client with optional
// asynchronous task polling. Existing read-only callers do not require a waiter.
func NewClientWithTaskWaiter(transportClient *transport.Client, waiter StorageContainerTaskWaiter) (*Client, error) {
	if transportClient == nil {
		return nil, ErrMissingClient
	}
	return &Client{executor: transportClient, waiter: waiter}, nil
}

// ListClusters returns validated clusters and the caller-only query identity.
func (c *Client) ListClusters(
	ctx context.Context,
	options odata.ListOptions,
) ([]Cluster, url.Values, error) {
	if c == nil || c.executor == nil {
		return nil, nil, ErrMissingClient
	}

	query, err := odata.Build(options, listClustersPolicy)
	if err != nil {
		return nil, nil, fmt.Errorf("build listClusters query: %w", err)
	}
	request, err := transport.NewRequest(transport.RequestOptions{
		Operation:        "listClusters",
		Method:           http.MethodGet,
		PathTemplate:     listClustersPath,
		Query:            query.Values(),
		ExpectedStatuses: []int{http.StatusOK},
		RetryClass:       transport.RetryRead,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("build listClusters request: %w", err)
	}
	response, err := c.executor.Execute(ctx, request)
	if err != nil {
		return nil, nil, fmt.Errorf("execute listClusters: %w", err)
	}
	clusters, err := apiresponse.DecodeList[Cluster](response.Body())
	if err != nil {
		return nil, nil, fmt.Errorf("decode listClusters response: %w", err)
	}
	if err := validateClusters(clusters); err != nil {
		return nil, nil, err
	}
	return clusters, query.IdentityValues(), nil
}

func validateClusters(clusters []Cluster) error {
	for index := range clusters {
		if clusters[index].ExtID == nil {
			return ErrInvalidCluster
		}
		if _, err := uuid.Parse(*clusters[index].ExtID); err != nil {
			return ErrInvalidCluster
		}
	}
	return nil
}

// GetStorageContainerByID returns one storage container and its opaque ETag.
func (c *Client) GetStorageContainerByID(ctx context.Context, extID string) (StorageContainerRead, error) {
	if c == nil || c.executor == nil {
		return StorageContainerRead{}, ErrMissingClient
	}
	requestedID, err := uuid.Parse(extID)
	if err != nil {
		return StorageContainerRead{}, ErrInvalidStorageContainer
	}
	request, err := transport.NewRequest(transport.RequestOptions{
		Operation:        "getStorageContainerById",
		Method:           http.MethodGet,
		PathTemplate:     storageContainerPath,
		PathParameters:   map[string]string{"extId": requestedID.String()},
		ExpectedStatuses: []int{http.StatusOK},
		RetryClass:       transport.RetryRead,
	})
	if err != nil {
		return StorageContainerRead{}, fmt.Errorf("build getStorageContainerById request: %w", err)
	}
	response, err := c.executor.Execute(ctx, request)
	if err != nil {
		if isHTTPStatus(err, http.StatusNotFound) {
			return StorageContainerRead{}, ErrStorageContainerNotFound
		}
		return StorageContainerRead{}, fmt.Errorf("execute getStorageContainerById: %w", err)
	}
	container, err := apiresponse.DecodeEntity[StorageContainer](response.Body())
	if err != nil {
		return StorageContainerRead{}, fmt.Errorf("decode getStorageContainerById response: %w", err)
	}
	if err := validateStorageContainerIdentity(container, requestedID); err != nil {
		return StorageContainerRead{}, err
	}
	return StorageContainerRead{StorageContainer: container, ETag: response.ETag()}, nil
}

// CreateStorageContainer submits clustermgmt v4.2 createStorageContainer.
func (c *Client) CreateStorageContainer(ctx context.Context, clusterExtID string, spec StorageContainerSpec) (StorageContainerAsyncOperation, error) {
	return c.submitStorageContainerMutation(ctx, "createStorageContainer", http.MethodPost, listStorageContainersPath, "", clusterExtID, spec, "", nil)
}

// UpdateStorageContainerByID submits a conditional clustermgmt v4.2 update.
func (c *Client) UpdateStorageContainerByID(ctx context.Context, extID string, spec StorageContainerSpec, etag string) (StorageContainerAsyncOperation, error) {
	return c.submitStorageContainerMutation(ctx, "updateStorageContainerById", http.MethodPut, storageContainerPath, extID, "", spec, etag, nil)
}

// DeleteStorageContainerByID submits clustermgmt v4.2 deleteStorageContainerById.
func (c *Client) DeleteStorageContainerByID(ctx context.Context, extID string, ignoreSmallFiles bool) (StorageContainerAsyncOperation, error) {
	return c.submitStorageContainerMutation(ctx, "deleteStorageContainerById", http.MethodDelete, storageContainerPath, extID, "", nil, "", url.Values{"ignoreSmallFiles": {strconv.FormatBool(ignoreSmallFiles)}})
}

// WaitStorageContainerTask waits through the shared Prism task waiter.
func (c *Client) WaitStorageContainerTask(ctx context.Context, operation StorageContainerAsyncOperation) (task.Snapshot, error) {
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

// StorageContainerIDFromTask returns the only valid storage-container entity
// reported by the completed Prism task.
func StorageContainerIDFromTask(snapshot task.Snapshot) (string, error) {
	storageContainerID, err := task.EntityIDByRelation(snapshot, storageContainerEntityRel)
	if err != nil {
		return "", ErrStorageContainerIdentityMissing
	}
	return storageContainerID, nil
}

func (c *Client) submitStorageContainerMutation(
	ctx context.Context,
	operation string,
	method string,
	pathTemplate string,
	extID string,
	clusterExtID string,
	spec any,
	etag string,
	query url.Values,
) (StorageContainerAsyncOperation, error) {
	if c == nil || c.executor == nil {
		return StorageContainerAsyncOperation{}, ErrMissingClient
	}
	if ctx == nil {
		return StorageContainerAsyncOperation{}, ErrInvalidStorageContainerSpec
	}
	parameters := map[string]string{}
	if extID != "" {
		requestedID, err := uuid.Parse(extID)
		if err != nil {
			return StorageContainerAsyncOperation{}, ErrInvalidStorageContainer
		}
		parameters["extId"] = requestedID.String()
	}
	if method != http.MethodDelete {
		storageSpec, ok := spec.(StorageContainerSpec)
		if !ok || validateStorageContainerSpec(storageSpec) != nil {
			return StorageContainerAsyncOperation{}, ErrInvalidStorageContainerSpec
		}
	}
	var body []byte
	if spec != nil {
		var err error
		body, err = json.Marshal(spec)
		if err != nil {
			return StorageContainerAsyncOperation{}, fmt.Errorf("encode %s request: %w", operation, err)
		}
	}
	headers := http.Header{"Content-Type": {"application/json"}}
	if clusterExtID != "" {
		clusterID, err := uuid.Parse(clusterExtID)
		if err != nil {
			return StorageContainerAsyncOperation{}, ErrInvalidStorageContainerSpec
		}
		headers.Set("X-Cluster-Id", clusterID.String())
	}
	request, err := transport.NewRequest(transport.RequestOptions{
		Operation:         operation,
		Method:            method,
		PathTemplate:      pathTemplate,
		PathParameters:    parameters,
		Query:             query,
		Headers:           headers,
		JSONBody:          body,
		ExpectedStatuses:  []int{http.StatusAccepted},
		RetryClass:        transport.RetryIdempotentMutation,
		RequestIDRequired: true,
		Replayable:        true,
		IfMatchRequired:   method == http.MethodPut,
		IfMatchETag:       etag,
	})
	if err != nil {
		return StorageContainerAsyncOperation{}, fmt.Errorf("build %s request: %w", operation, err)
	}
	response, err := c.executor.Execute(ctx, request)
	if err != nil {
		if isHTTPStatus(err, http.StatusNotFound) {
			return StorageContainerAsyncOperation{}, ErrStorageContainerNotFound
		}
		return StorageContainerAsyncOperation{}, fmt.Errorf("execute %s: %w", operation, err)
	}
	return decodeStorageContainerAsyncOperation(response)
}

type storageContainerTaskReference struct {
	ExtID string `json:"extId"`
}

func decodeStorageContainerAsyncOperation(response transport.Response) (StorageContainerAsyncOperation, error) {
	if strings.TrimSpace(response.Headers().Get("Location")) == "" {
		return StorageContainerAsyncOperation{}, ErrInvalidAsyncResponse
	}
	var envelope struct {
		Data *storageContainerTaskReference `json:"data"`
	}
	decoder := json.NewDecoder(bytes.NewReader(response.Body()))
	if err := decoder.Decode(&envelope); err != nil || envelope.Data == nil || envelope.Data.ExtID == "" {
		return StorageContainerAsyncOperation{}, ErrInvalidAsyncResponse
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return StorageContainerAsyncOperation{}, ErrInvalidAsyncResponse
	}
	if _, err := url.ParseRequestURI(response.Headers().Get("Location")); err != nil {
		return StorageContainerAsyncOperation{}, ErrInvalidAsyncResponse
	}
	return StorageContainerAsyncOperation{TaskID: envelope.Data.ExtID}, nil
}

func validateStorageContainerSpec(spec StorageContainerSpec) error {
	if spec.Name == "" || len(spec.Name) > 75 {
		return ErrInvalidStorageContainerSpec
	}
	if spec.ErasureCode != nil && !oneOf(*spec.ErasureCode, "NONE", "OFF", "ON") {
		return ErrInvalidStorageContainerSpec
	}
	if spec.CacheDeduplication != nil && !oneOf(*spec.CacheDeduplication, "NONE", "OFF", "ON") {
		return ErrInvalidStorageContainerSpec
	}
	if spec.OnDiskDedup != nil && !oneOf(*spec.OnDiskDedup, "NONE", "OFF", "POST_PROCESS") {
		return ErrInvalidStorageContainerSpec
	}
	return nil
}

func validateStorageContainerIdentity(container StorageContainer, requestedID uuid.UUID) error {
	if container.ExtID == nil {
		return ErrInvalidStorageContainer
	}
	responseID, err := uuid.Parse(*container.ExtID)
	if err != nil || responseID != requestedID {
		return ErrInvalidStorageContainer
	}
	return nil
}

func oneOf(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}

func isHTTPStatus(err error, status int) bool {
	var httpError *transport.HTTPError
	return errors.As(err, &httpError) && httpError.StatusCode() == status
}
