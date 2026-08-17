package testkit

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestContainerBoundary(t *testing.T) {
	if os.Getenv("NUTANIX_TESTCONTAINERS") != "1" {
		t.Skip("set NUTANIX_TESTCONTAINERS=1 to run the container boundary suite")
	}

	configPath := os.Getenv("NUTANIX_TEST_CONFIG")
	if configPath == "" {
		configPath = filepath.Join("..", "..", "config", "testing.yaml")
	}
	config, err := Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := config.Testcontainers.Runtime()
	if err != nil {
		t.Fatal(err)
	}
	if runtime.RyukDisabled {
		t.Setenv("TESTCONTAINERS_RYUK_DISABLED", "true")
	}

	ctx, cancel := context.WithTimeout(context.Background(), runtime.StartupTimeout+runtime.CleanupTimeout)
	t.Cleanup(cancel)
	container, err := testcontainers.Run(
		ctx,
		runtime.Image,
		testcontainers.WithProvider(testcontainers.ProviderPodman),
		testcontainers.WithCmd(runtime.Command...),
		testcontainers.WithLabels(map[string]string{
			"io.ioplane.nutanix.test-suite": "testkit",
		}),
		testcontainers.WithWaitStrategy(
			wait.ForLog(runtime.ReadyMessage).WithStartupTimeout(runtime.StartupTimeout),
		),
	)
	testcontainers.CleanupContainer(t, container)
	if err != nil {
		t.Fatalf("start test container: %v", err)
	}

	state, err := container.State(ctx)
	if err != nil {
		t.Fatalf("inspect test container: %v", err)
	}
	if !state.Running {
		t.Fatalf("test container running = false, want true")
	}
}
