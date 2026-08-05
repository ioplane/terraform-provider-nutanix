package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccProviderProtocol6(t *testing.T) {
	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"nutanix": providerserver.NewProtocol6WithError(New("test")()),
		},
		Steps: []resource.TestStep{{
			Config: `
terraform {
  required_providers {
    nutanix = {
      source = "ioplane/nutanix"
    }
  }
}

provider "nutanix" {}
`,
		}},
	})
}
