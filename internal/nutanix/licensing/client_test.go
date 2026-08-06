package licensing

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ioplane/terraform-provider-nutanix/internal/auth"
	"github.com/ioplane/terraform-provider-nutanix/internal/nutanix/odata"
	"github.com/ioplane/terraform-provider-nutanix/internal/transport"
)

func TestListLicenseKeysUsesLockedOperationAndQuery(t *testing.T) {
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet {
			t.Errorf("method = %q, want GET", request.Method)
		}
		if request.URL.Path != listLicenseKeysPath {
			t.Errorf("path = %q, want %q", request.URL.Path, listLicenseKeysPath)
		}
		query := request.URL.Query()
		for name, want := range map[string]string{
			"$page":    "2",
			"$limit":   "10",
			"$filter":  "key eq 'NCI-KEY'",
			"$orderby": "key desc",
			"$select":  "category,enforcementPolicy,entitlementExpiryDate,extId,foo,groupId,key,meter,quantity,subCategory,tenantId,type,validationDetail",
			"$expand":  "assignmentDetails,associationDetails",
		} {
			if got := query.Get(name); got != want {
				t.Errorf("query %s = %q, want %q", name, got, want)
			}
		}
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{"data":[{"extId":"691d7304-52c4-4325-9cb7-2168a210759f","tenantId":"a088a6f0-6822-4ba9-b4a1-5928abfeb344","key":"NCI-KEY","validationDetail":"valid","type":"NCI","category":"PRO","subCategory":"PRIMARY","entitlementExpiryDate":"2027-01-31","meter":"CORES","quantity":72.5,"groupId":"group-1","enforcementPolicy":"ALL","assignmentDetails":[{"key":"NCI-KEY","quantityUsed":12.5,"clusterExtId":"b6c476a0-1d38-4530-9bf8-f61ac524b9d7"}],"associationDetails":[{"baseKey":"NCI-BASE","associatedKey":"NCI-KEY","associationType":"CHILD_KEYS","reclaimType":"FULL_QUANTITY"}]}]}`))
	}))
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	server.Listener = listener
	server.StartTLS()
	t.Cleanup(server.Close)

	origin, err := transport.ParseOrigin(server.URL)
	if err != nil {
		t.Fatalf("ParseOrigin() error = %v", err)
	}
	tlsConfig, err := transport.NewTLSConfig(origin, true, "")
	if err != nil {
		t.Fatalf("NewTLSConfig() error = %v", err)
	}
	transportClient, err := transport.NewClient(origin, auth.NewAPIKey("test-api-key"), tlsConfig, time.Second, "test", "test")
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	client, err := NewClient(transportClient)
	if err != nil {
		t.Fatalf("licensing.NewClient() error = %v", err)
	}

	page, limit := int64(2), int64(10)
	filter, orderBy, selectValue, expand := "key eq 'NCI-KEY'", "key desc", "foo", "associationDetails,assignmentDetails"
	keys, identity, err := client.ListLicenseKeys(context.Background(), odata.ListOptions{
		Page:    &page,
		Limit:   &limit,
		Filter:  &filter,
		OrderBy: &orderBy,
		Select:  &selectValue,
		Expand:  &expand,
	})
	if err != nil {
		t.Fatalf("ListLicenseKeys() error = %v", err)
	}
	if len(keys) != 1 || keys[0].Key == nil || *keys[0].Key != "NCI-KEY" {
		t.Fatalf("keys = %#v, want one NCI-KEY", keys)
	}
	if keys[0].AssignmentDetails == nil || len(*keys[0].AssignmentDetails) != 1 {
		t.Fatalf("assignment details = %#v, want one item", keys[0].AssignmentDetails)
	}
	if keys[0].AssociationDetails == nil || len(*keys[0].AssociationDetails) != 1 {
		t.Fatalf("association details = %#v, want one item", keys[0].AssociationDetails)
	}
	if got := identity.Get("$select"); got != "foo" || identity.Get("$expand") != "associationDetails,assignmentDetails" {
		t.Fatalf("caller identity = %v, want caller-only select/expand", identity)
	}
}

func TestListLicenseKeysRejectsInvalidIdentity(t *testing.T) {
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "application/json")
		_, _ = response.Write([]byte(`{"data":[{"extId":"not-a-uuid"}]}`))
	}))
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	server.Listener = listener
	server.StartTLS()
	t.Cleanup(server.Close)
	origin, err := transport.ParseOrigin(server.URL)
	if err != nil {
		t.Fatalf("ParseOrigin() error = %v", err)
	}
	tlsConfig, err := transport.NewTLSConfig(origin, true, "")
	if err != nil {
		t.Fatalf("NewTLSConfig() error = %v", err)
	}
	transportClient, err := transport.NewClient(origin, auth.NewAPIKey("test-api-key"), tlsConfig, time.Second, "test", "test")
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	client, err := NewClient(transportClient)
	if err != nil {
		t.Fatalf("licensing.NewClient() error = %v", err)
	}
	_, _, err = client.ListLicenseKeys(context.Background(), odata.ListOptions{})
	if !errors.Is(err, ErrInvalidLicenseKey) {
		t.Fatalf("ListLicenseKeys() error = %v, want %v", err, ErrInvalidLicenseKey)
	}
	if strings.Contains(err.Error(), "not-a-uuid") {
		t.Fatal("invalid remote identity leaked into the error")
	}
}
