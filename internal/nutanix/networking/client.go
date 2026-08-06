// Package networking implements hand-written Networking API operations.
package networking

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
	"github.com/ioplane/terraform-provider-nutanix/internal/task"
	"github.com/ioplane/terraform-provider-nutanix/internal/transport"
)

const (
	listSubnetsPath   = "/api/networking/v4.3/config/subnets"
	getSubnetByIDPath = "/api/networking/v4.3/config/subnets/{extId}"
	subnetEntityRel   = "networking:config:subnet"
)

var (
	// ErrMissingClient identifies an absent transport dependency.
	ErrMissingClient = errors.New("networking client is missing")
	// ErrInvalidSubnetID identifies a request without a valid subnet UUID.
	ErrInvalidSubnetID = errors.New("networking subnet identity is invalid")
	// ErrInvalidSubnet identifies a response with a missing or different subnet identity.
	ErrInvalidSubnet = errors.New("networking subnet response is invalid")
	// ErrSubnetNotFound identifies a subnet that no longer exists remotely.
	ErrSubnetNotFound = errors.New("networking subnet was not found")
	// ErrMissingTaskWaiter identifies an async client without a task dependency.
	ErrMissingTaskWaiter = errors.New("networking task waiter is missing")
	// ErrInvalidSubnetSpec identifies a write payload outside the reviewed API contract.
	ErrInvalidSubnetSpec = errors.New("networking subnet specification is invalid")
	// ErrInvalidAsyncResponse identifies a malformed asynchronous API response.
	ErrInvalidAsyncResponse = errors.New("networking asynchronous response is invalid")
	// ErrSubnetIdentityMissing identifies a successful task without a subnet entity.
	ErrSubnetIdentityMissing = errors.New("networking subnet identity is missing from task")
)

type executor interface {
	Execute(context.Context, transport.Request) (transport.Response, error)
}

// Client owns the Networking operation policies over one transport client.
type Client struct {
	executor executor
	waiter   TaskWaiter
}

// NewClientWithTaskWaiter constructs a Networking client with optional async
// task polling. Read operations do not require the waiter dependency.
func NewClientWithTaskWaiter(transportClient *transport.Client, waiter TaskWaiter) (*Client, error) {
	if transportClient == nil {
		return nil, ErrMissingClient
	}
	return &Client{executor: transportClient, waiter: waiter}, nil
}

// SubnetRead is a validated subnet projection plus its opaque response ETag.
type SubnetRead struct {
	Subnet Subnet
	ETag   string
}

// GetSubnetByID returns one subnet whose identity matches the requested UUID.
func (c *Client) GetSubnetByID(ctx context.Context, extID string) (Subnet, error) {
	read, err := c.GetSubnetByIDWithETag(ctx, extID)
	if err != nil {
		return Subnet{}, err
	}
	return read.Subnet, nil
}

// GetSubnetByIDWithETag returns a subnet projection and the current opaque ETag.
func (c *Client) GetSubnetByIDWithETag(ctx context.Context, extID string) (SubnetRead, error) {
	if c == nil || c.executor == nil {
		return SubnetRead{}, ErrMissingClient
	}
	requestedID, err := uuid.Parse(extID)
	if err != nil {
		return SubnetRead{}, ErrInvalidSubnetID
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
		return SubnetRead{}, fmt.Errorf("build getSubnetById request: %w", err)
	}
	response, err := c.executor.Execute(ctx, request)
	if err != nil {
		var httpErr *transport.HTTPError
		if errors.As(err, &httpErr) && httpErr.StatusCode() == http.StatusNotFound {
			return SubnetRead{}, ErrSubnetNotFound
		}
		return SubnetRead{}, fmt.Errorf("execute getSubnetById: %w", err)
	}
	subnet, err := apiresponse.DecodeEntity[Subnet](response.Body())
	if err != nil {
		return SubnetRead{}, fmt.Errorf("decode getSubnetById response: %w", err)
	}
	if err := validateSubnetIdentity(subnet, requestedID); err != nil {
		return SubnetRead{}, err
	}
	return SubnetRead{Subnet: subnet, ETag: response.ETag()}, nil
}

// CreateSubnet submits a Networking v4.3 createSubnet operation.
func (c *Client) CreateSubnet(ctx context.Context, spec SubnetSpec) (AsyncOperation, error) {
	return c.submitSubnetMutation(ctx, "createSubnet", http.MethodPost, listSubnetsPath, "", spec, "")
}

// UpdateSubnetByID submits a conditional Networking v4.3 updateSubnetById operation.
func (c *Client) UpdateSubnetByID(
	ctx context.Context,
	extID string,
	spec SubnetSpec,
	etag string,
) (AsyncOperation, error) {
	return c.submitSubnetMutation(ctx, "updateSubnetById", http.MethodPut, getSubnetByIDPath, extID, spec, etag)
}

// DeleteSubnetByID submits a Networking v4.3 deleteSubnetById operation.
func (c *Client) DeleteSubnetByID(ctx context.Context, extID string) (AsyncOperation, error) {
	return c.submitSubnetMutation(ctx, "deleteSubnetById", http.MethodDelete, getSubnetByIDPath, extID, nil, "")
}

// WaitTask waits for one submitted Networking operation through the shared task port.
func (c *Client) WaitTask(ctx context.Context, operation AsyncOperation) (task.Snapshot, error) {
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

// SubnetIDFromTask returns the UUID of the subnet entity affected by a task.
// It fails closed when the Prism task does not identify exactly a valid subnet
// reference rather than falling back to a name-based lookup.
func SubnetIDFromTask(snapshot task.Snapshot) (string, error) {
	subnetID, err := task.EntityIDByRelation(snapshot, subnetEntityRel)
	if err != nil {
		return "", ErrSubnetIdentityMissing
	}
	return subnetID, nil
}

func (c *Client) submitSubnetMutation(
	ctx context.Context,
	operation string,
	method string,
	pathTemplate string,
	extID string,
	spec any,
	etag string,
) (AsyncOperation, error) {
	if c == nil || c.executor == nil {
		return AsyncOperation{}, ErrMissingClient
	}
	if ctx == nil {
		return AsyncOperation{}, ErrInvalidSubnetSpec
	}
	if method != http.MethodDelete {
		if err := validateSubnetSpec(spec.(SubnetSpec)); err != nil {
			return AsyncOperation{}, err
		}
	}
	parameters := map[string]string{}
	if extID != "" {
		requestedID, err := uuid.Parse(extID)
		if err != nil {
			return AsyncOperation{}, ErrInvalidSubnetID
		}
		parameters["extId"] = requestedID.String()
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
		RetryClass:        transport.RetryIdempotentMutation,
		RequestIDRequired: true,
		Replayable:        true,
		IfMatchRequired:   method == http.MethodPut,
		IfMatchETag:       etag,
	})
	if err != nil {
		return AsyncOperation{}, fmt.Errorf("build %s request: %w", operation, err)
	}
	response, err := c.executor.Execute(ctx, request)
	if err != nil {
		var httpErr *transport.HTTPError
		if errors.As(err, &httpErr) && httpErr.StatusCode() == http.StatusNotFound {
			return AsyncOperation{}, ErrSubnetNotFound
		}
		return AsyncOperation{}, fmt.Errorf("execute %s: %w", operation, err)
	}
	return decodeAsyncOperation(response)
}

type taskReference struct {
	ExtID string `json:"extId"`
}

func decodeAsyncOperation(response transport.Response) (AsyncOperation, error) {
	if strings.TrimSpace(response.Headers().Get("Location")) == "" {
		return AsyncOperation{}, ErrInvalidAsyncResponse
	}
	var envelope struct {
		Data *taskReference `json:"data"`
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

func validateSubnetSpec(spec SubnetSpec) error {
	if spec.Name == "" || len(spec.Name) > 128 ||
		(spec.SubnetType != "VLAN" && spec.SubnetType != "OVERLAY") {
		return ErrInvalidSubnetSpec
	}
	if spec.Description != nil && len(*spec.Description) > 1000 {
		return ErrInvalidSubnetSpec
	}
	if spec.NetworkID != nil {
		if *spec.NetworkID < 0 || *spec.NetworkID > 16777215 ||
			(spec.SubnetType == "VLAN" && *spec.NetworkID > 4095) {
			return ErrInvalidSubnetSpec
		}
	}
	return nil
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
