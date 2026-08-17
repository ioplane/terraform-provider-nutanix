// Package testkit contains reusable Go test infrastructure.
package testkit

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

var (
	ErrInvalidConfig = errors.New("test configuration is invalid")
	ErrMissingConfig = errors.New("test configuration is missing")
)

// Config is the strict, repository-owned test runtime configuration.
type Config struct {
	Testcontainers ContainerConfig `yaml:"testcontainers"`
}

// ContainerConfig defines one suite-level Testcontainers dependency.
type ContainerConfig struct {
	Image          string   `yaml:"image"`
	StartupTimeout string   `yaml:"startup_timeout"`
	CleanupTimeout string   `yaml:"cleanup_timeout"`
	RyukDisabled   bool     `yaml:"ryuk_disabled"`
	ReadyMessage   string   `yaml:"ready_message"`
	Command        []string `yaml:"command"`
}

// Runtime returns validated duration values for container lifecycle code.
func (c ContainerConfig) Runtime() (ContainerRuntime, error) {
	startup, err := time.ParseDuration(c.StartupTimeout)
	if err != nil || startup <= 0 {
		return ContainerRuntime{}, fmt.Errorf("%w: startup_timeout must be positive: %v", ErrInvalidConfig, err)
	}
	cleanup, err := time.ParseDuration(c.CleanupTimeout)
	if err != nil || cleanup <= 0 {
		return ContainerRuntime{}, fmt.Errorf("%w: cleanup_timeout must be positive: %v", ErrInvalidConfig, err)
	}
	if strings.TrimSpace(c.Image) == "" || strings.ContainsAny(c.Image, "\r\n\t ") {
		return ContainerRuntime{}, fmt.Errorf("%w: image must be a single non-empty reference", ErrInvalidConfig)
	}
	if strings.TrimSpace(c.ReadyMessage) == "" {
		return ContainerRuntime{}, fmt.Errorf("%w: ready_message must not be empty", ErrInvalidConfig)
	}
	if len(c.Command) == 0 || anyEmpty(c.Command) {
		return ContainerRuntime{}, fmt.Errorf("%w: command must not be empty", ErrInvalidConfig)
	}
	return ContainerRuntime{
		Image:          c.Image,
		StartupTimeout: startup,
		CleanupTimeout: cleanup,
		RyukDisabled:   c.RyukDisabled,
		ReadyMessage:   c.ReadyMessage,
		Command:        append([]string(nil), c.Command...),
	}, nil
}

// ContainerRuntime is the validated form consumed by Testcontainers tests.
type ContainerRuntime struct {
	Image          string
	StartupTimeout time.Duration
	CleanupTimeout time.Duration
	RyukDisabled   bool
	ReadyMessage   string
	Command        []string
}

// Load reads exactly one YAML document and rejects unknown fields.
func Load(path string) (Config, error) {
	if strings.TrimSpace(path) == "" {
		return Config{}, ErrMissingConfig
	}
	file, err := os.Open(filepath.Clean(path))
	if err != nil {
		return Config{}, fmt.Errorf("open test config: %w", err)
	}
	defer func() { _ = file.Close() }()

	decoder := yaml.NewDecoder(file)
	decoder.KnownFields(true)
	var config Config
	if err := decoder.Decode(&config); err != nil {
		return Config{}, fmt.Errorf("decode test config: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return Config{}, fmt.Errorf("%w: multiple YAML documents", ErrInvalidConfig)
		}
		return Config{}, fmt.Errorf("decode trailing test config: %w", err)
	}
	if _, err := config.Testcontainers.Runtime(); err != nil {
		return Config{}, err
	}
	return config, nil
}

func anyEmpty(values []string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			return true
		}
	}
	return false
}
