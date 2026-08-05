package main

import "testing"

func TestServeOptions(t *testing.T) {
	t.Parallel()

	options := serveOptions(true)
	if options.Address != "registry.terraform.io/ioplane/nutanix" {
		t.Fatalf(
			"provider address = %q, want %q",
			options.Address,
			"registry.terraform.io/ioplane/nutanix",
		)
	}
	if options.ProtocolVersion != 6 {
		t.Fatalf("protocol version = %d, want 6", options.ProtocolVersion)
	}
	if !options.Debug {
		t.Fatal("debug option = false, want true")
	}
}
