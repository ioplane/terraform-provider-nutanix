// Package clustermgmt implements hand-written Cluster Management API operations.
package clustermgmt

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

const listClustersPath = "/api/clustermgmt/v4.2/config/clusters"

var (
	// ErrMissingClient identifies an absent transport dependency.
	ErrMissingClient = errors.New("clustermgmt client is missing")
	// ErrInvalidCluster identifies a cluster response without a valid remote identity.
	ErrInvalidCluster = errors.New("clustermgmt cluster response is invalid")

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
}

// NewClient constructs a Cluster Management client without making a network request.
func NewClient(transportClient *transport.Client) (*Client, error) {
	if transportClient == nil {
		return nil, ErrMissingClient
	}
	return &Client{executor: transportClient}, nil
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
