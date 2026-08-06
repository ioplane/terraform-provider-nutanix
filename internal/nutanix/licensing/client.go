// Package licensing implements hand-written Licensing API operations.
package licensing

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
	listLicensesPath    = "/api/licensing/v4.3/config/licenses"
	listLicenseKeysPath = "/api/licensing/v4.3/config/license-keys"
)

var (
	// ErrMissingClient identifies an absent transport dependency.
	ErrMissingClient = errors.New("licensing client is missing")
	// ErrInvalidLicense identifies a malformed applied-license identity.
	ErrInvalidLicense = errors.New("licensing license response is invalid")
	// ErrInvalidLicenseKey identifies a malformed license-key identity.
	ErrInvalidLicenseKey = errors.New("licensing license-key response is invalid")

	listLicensesPolicy = odata.Policy{
		RequiredSelect: []string{
			"category",
			"expiryDate",
			"extId",
			"meter",
			"name",
			"quantity",
			"salesforceLicenseId",
			"scope",
			"subCategory",
			"type",
		},
		AllowExpand: true,
	}
	listLicenseKeysPolicy = odata.Policy{
		RequiredSelect: []string{
			"category",
			"entitlementExpiryDate",
			"enforcementPolicy",
			"extId",
			"groupId",
			"key",
			"meter",
			"quantity",
			"subCategory",
			"tenantId",
			"type",
			"validationDetail",
		},
		AllowExpand: true,
	}
)

type executor interface {
	Execute(context.Context, transport.Request) (transport.Response, error)
}

// Client owns Licensing v4.3 operation policies over one transport client.
type Client struct {
	executor executor
}

// NewClient constructs a Licensing client without making a network request.
func NewClient(transportClient *transport.Client) (*Client, error) {
	if transportClient == nil {
		return nil, ErrMissingClient
	}
	return &Client{executor: transportClient}, nil
}

// ListLicenses returns applied licenses and the caller-only query identity.
func (c *Client) ListLicenses(
	ctx context.Context,
	options odata.ListOptions,
) ([]License, url.Values, error) {
	if c == nil || c.executor == nil {
		return nil, nil, ErrMissingClient
	}
	query, err := odata.Build(options, listLicensesPolicy)
	if err != nil {
		return nil, nil, fmt.Errorf("build listLicenses query: %w", err)
	}
	request, err := transport.NewRequest(transport.RequestOptions{
		Operation:        "listLicenses",
		Method:           http.MethodGet,
		PathTemplate:     listLicensesPath,
		Query:            query.Values(),
		ExpectedStatuses: []int{http.StatusOK},
		RetryClass:       transport.RetryRead,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("build listLicenses request: %w", err)
	}
	response, err := c.executor.Execute(ctx, request)
	if err != nil {
		return nil, nil, fmt.Errorf("execute listLicenses: %w", err)
	}
	licenses, err := apiresponse.DecodeList[License](response.Body())
	if err != nil {
		return nil, nil, fmt.Errorf("decode listLicenses response: %w", err)
	}
	if err := validateLicenses(licenses); err != nil {
		return nil, nil, err
	}
	return licenses, query.IdentityValues(), nil
}

func validateLicenses(licenses []License) error {
	for index := range licenses {
		if licenses[index].ExtID == nil {
			continue
		}
		if _, err := uuid.Parse(*licenses[index].ExtID); err != nil {
			return ErrInvalidLicense
		}
	}
	return nil
}

// ListLicenseKeys returns the license-key inventory and the caller-only query identity.
func (c *Client) ListLicenseKeys(
	ctx context.Context,
	options odata.ListOptions,
) ([]LicenseKey, url.Values, error) {
	if c == nil || c.executor == nil {
		return nil, nil, ErrMissingClient
	}
	query, err := odata.Build(options, listLicenseKeysPolicy)
	if err != nil {
		return nil, nil, fmt.Errorf("build listLicenseKeys query: %w", err)
	}
	request, err := transport.NewRequest(transport.RequestOptions{
		Operation:        "listLicenseKeys",
		Method:           http.MethodGet,
		PathTemplate:     listLicenseKeysPath,
		Query:            query.Values(),
		ExpectedStatuses: []int{http.StatusOK},
		RetryClass:       transport.RetryRead,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("build listLicenseKeys request: %w", err)
	}
	response, err := c.executor.Execute(ctx, request)
	if err != nil {
		return nil, nil, fmt.Errorf("execute listLicenseKeys: %w", err)
	}
	keys, err := apiresponse.DecodeList[LicenseKey](response.Body())
	if err != nil {
		return nil, nil, fmt.Errorf("decode listLicenseKeys response: %w", err)
	}
	if err := validateLicenseKeys(keys); err != nil {
		return nil, nil, err
	}
	return keys, query.IdentityValues(), nil
}

func validateLicenseKeys(keys []LicenseKey) error {
	for index := range keys {
		key := keys[index]
		if key.ExtID != nil {
			if _, err := uuid.Parse(*key.ExtID); err != nil {
				return ErrInvalidLicenseKey
			}
		}
		if key.TenantID != nil {
			if _, err := uuid.Parse(*key.TenantID); err != nil {
				return ErrInvalidLicenseKey
			}
		}
		if key.AssignmentDetails == nil {
			continue
		}
		for assignmentIndex := range *key.AssignmentDetails {
			clusterExtID := (*key.AssignmentDetails)[assignmentIndex].ClusterExtID
			if clusterExtID == nil {
				continue
			}
			if _, err := uuid.Parse(*clusterExtID); err != nil {
				return ErrInvalidLicenseKey
			}
		}
	}
	return nil
}
