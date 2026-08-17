package testkit

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadRejectsUnknownFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "testing.yaml")
	if err := os.WriteFile(path, []byte("testcontainers:\n  image: example\n  unknown: true\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := Load(path)
	if err == nil {
		t.Fatal("Load() error = nil, want unknown-field error")
	}
}

func TestLoadRejectsMultipleDocuments(t *testing.T) {
	path := filepath.Join(t.TempDir(), "testing.yaml")
	body := "testcontainers:\n  image: example\n  startup_timeout: 1s\n  cleanup_timeout: 1s\n  ready_message: ready\n  command: [echo, ready]\n---\ntestcontainers: {}\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := Load(path)
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("Load() error = %v, want ErrInvalidConfig", err)
	}
}

func TestLoadRejectsInvalidRuntime(t *testing.T) {
	path := filepath.Join(t.TempDir(), "testing.yaml")
	body := "testcontainers:\n  image: example image\n  startup_timeout: 0s\n  cleanup_timeout: 1s\n  ready_message: ready\n  command: [echo, ready]\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := Load(path)
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("Load() error = %v, want ErrInvalidConfig", err)
	}
}

func TestRepositoryConfigIsValid(t *testing.T) {
	config, err := Load(filepath.Join("..", "..", "config", "testing.yaml"))
	if err != nil {
		t.Fatalf("Load(repository config) error = %v", err)
	}
	if config.Testcontainers.Image == "" {
		t.Fatal("repository test container image is empty")
	}
}
