package main

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

func TestProjectNameUsesStableRepositoryIdentity(t *testing.T) {
	common := "/repo/.git"
	root := "/repo/worktrees/provider"
	wantDigest := sha256.Sum256([]byte(common + "\x00" + root))
	want := projectPrefix + hex.EncodeToString(wantDigest[:])[:12]
	if got := stableProjectName(common, root); got != want {
		t.Fatalf("stableProjectName() = %q, want %q", got, want)
	}
}

func TestAppendContainerRuntimeMountsPodmanSocketForTestcontainers(t *testing.T) {
	args := appendContainerRuntime([]string{"run", "image", "sleep", "infinity"}, "unix:///run/podman/podman.sock")
	joined := " " + strings.Join(args, " ") + " "
	for _, expected := range []string{
		" DOCKER_HOST=unix:///var/run/docker.sock ",
		" TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE=/var/run/docker.sock ",
		" /run/podman/podman.sock:/var/run/docker.sock ",
	} {
		if !strings.Contains(joined, expected) {
			t.Fatalf("runtime arguments %q do not contain %q", args, expected)
		}
	}
}

func TestConfiguredContainerHostPrefersContainerHostAndFallsBackToDockerHost(t *testing.T) {
	t.Setenv("CONTAINER_HOST", "unix:///run/podman/preferred.sock")
	t.Setenv("DOCKER_HOST", "unix:///run/podman/fallback.sock")
	if got := configuredContainerHost(); got != "unix:///run/podman/preferred.sock" {
		t.Fatalf("configuredContainerHost() = %q, want CONTAINER_HOST", got)
	}

	t.Setenv("CONTAINER_HOST", "")
	if got := configuredContainerHost(); got != "unix:///run/podman/fallback.sock" {
		t.Fatalf("configuredContainerHost() = %q, want DOCKER_HOST fallback", got)
	}
}
