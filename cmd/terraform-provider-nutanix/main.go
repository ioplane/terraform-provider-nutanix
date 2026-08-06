package main

import (
	"context"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"

	"github.com/ioplane/terraform-provider-nutanix/internal/provider"
)

const providerAddress = "registry.terraform.io/ioplane/nutanix"

var version = "0.0.0-dev"

func serveOptions(debug bool) providerserver.ServeOpts {
	return providerserver.ServeOpts{
		Address:         providerAddress,
		Debug:           debug,
		ProtocolVersion: 6,
	}
}

func main() {
	err := providerserver.Serve(
		context.Background(),
		provider.New(version),
		serveOptions(false),
	)
	if err != nil {
		log.Fatal(err)
	}
}
