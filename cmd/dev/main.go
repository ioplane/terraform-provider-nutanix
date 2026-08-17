// Command dev manages the repository's Podman development toolbox.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	projectPrefix = "nutanix-provider-"
	serviceName   = "dev"
	defaultHost   = "unix:///run/podman/podman.sock"
	containerWait = 180 * time.Second
	commandWait   = 60 * time.Minute
)

type project struct {
	root      string
	commonDir string
	gitDir    string
	gitRel    string
	name      string
	image     string
	container string
	version   string
	revision  string
	created   string
}

func main() {
	if err := execute(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "dev: %v\n", err)
		os.Exit(1)
	}
}

func execute(args []string) error {
	if len(args) == 0 {
		return errors.New("a command is required: up, down, status, shell, task, or beads")
	}
	p, err := discoverProject()
	if err != nil {
		return err
	}
	switch args[0] {
	case "up":
		return p.up()
	case "down":
		return p.down()
	case "status":
		return p.status()
	case "shell":
		return p.shell()
	case "task":
		return p.execInToolbox("task", args[1:]...)
	case "beads":
		return p.execInToolbox("bd", args[1:]...)
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func discoverProject() (project, error) {
	root, err := gitPath("--show-toplevel")
	if err != nil {
		return project{}, fmt.Errorf("discover repository root: %w", err)
	}
	common, err := gitPath("--git-common-dir")
	if err != nil {
		return project{}, fmt.Errorf("discover Git common directory: %w", err)
	}
	gitDir, err := gitPath("--git-dir")
	if err != nil {
		return project{}, fmt.Errorf("discover Git directory: %w", err)
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return project{}, fmt.Errorf("resolve repository root: %w", err)
	}
	common, err = filepath.Abs(common)
	if err != nil {
		return project{}, fmt.Errorf("resolve Git common directory: %w", err)
	}
	gitDir, err = filepath.Abs(gitDir)
	if err != nil {
		return project{}, fmt.Errorf("resolve Git directory: %w", err)
	}
	relative, err := filepath.Rel(common, gitDir)
	if err != nil || strings.HasPrefix(relative, "..") {
		return project{}, fmt.Errorf("git directory %s is outside common directory %s", gitDir, common)
	}
	versionBytes, err := os.ReadFile(filepath.Join(root, "VERSION"))
	if err != nil {
		return project{}, fmt.Errorf("read VERSION: %w", err)
	}
	version := strings.TrimSpace(string(versionBytes))
	if version == "" {
		return project{}, errors.New("VERSION is empty")
	}
	revision, err := gitOutput("rev-parse", "HEAD")
	if err != nil {
		return project{}, fmt.Errorf("read Git revision: %w", err)
	}
	created, err := gitOutput("show", "-s", "--format=%cI", "HEAD")
	if err != nil {
		return project{}, fmt.Errorf("read Git timestamp: %w", err)
	}
	name := stableProjectName(common, root)
	return project{
		root:      root,
		commonDir: common,
		gitDir:    gitDir,
		gitRel:    relative,
		name:      name,
		image:     "localhost/" + name + "_dev:latest",
		container: name + "_dev_1",
		version:   version,
		revision:  strings.TrimSpace(revision),
		created:   strings.TrimSpace(created),
	}, nil
}

func stableProjectName(common, root string) string {
	identity := sha256.Sum256([]byte(common + "\x00" + root))
	return projectPrefix + hex.EncodeToString(identity[:])[:12]
}

func (p project) up() error {
	build := exec.Command("podman", "build", "--file", filepath.Join(p.root, "deployments/containers/Containerfile.dev"), "--tag", p.image,
		"--build-arg", "OCI_VERSION="+p.version,
		"--build-arg", "OCI_REVISION="+p.revision,
		"--build-arg", "OCI_CREATED="+p.created,
		p.root,
	)
	build.Stdout, build.Stderr = os.Stdout, os.Stderr
	if err := runWithTimeout(build, commandWait); err != nil {
		return fmt.Errorf("build toolbox image: %w", err)
	}
	_ = exec.Command("podman", "rm", "--force", p.container).Run()
	runArgs := []string{
		"run", "--detach", "--init", "--name", p.container,
		"--label", "io.podman.compose.project=" + p.name,
		"--label", "io.podman.compose.service=" + serviceName,
		"--workdir", "/workspace",
		"--env", "BEADS_DIR=/workspace/.beads",
		"--env", "GIT_COMMON_DIR=/git-metadata/.git",
		"--env", "GIT_DIR=/git-metadata/.git/" + filepath.ToSlash(p.gitRel),
		"--env", "GIT_WORK_TREE=/workspace",
		"--volume", p.root + ":/workspace:z",
		"--volume", p.commonDir + ":/git-metadata/.git:z",
		"--volume", p.name + "_go-mod-cache:/go/pkg/mod",
		"--volume", p.name + "_go-build-cache:/root/.cache/go-build",
		p.image, "sleep", "infinity",
	}
	runArgs = appendContainerRuntime(runArgs, configuredContainerHost())
	started := exec.Command("podman", runArgs...)
	started.Stdout, started.Stderr = os.Stdout, os.Stderr
	if err := runWithTimeout(started, commandWait); err != nil {
		return fmt.Errorf("start toolbox container: %w", err)
	}
	return p.waitHealthy()
}

func configuredContainerHost() string {
	if host := os.Getenv("CONTAINER_HOST"); host != "" {
		return host
	}
	if host := os.Getenv("DOCKER_HOST"); host != "" {
		return host
	}
	return defaultHost
}

func appendContainerRuntime(arguments []string, containerHost string) []string {
	if !strings.HasPrefix(containerHost, "unix://") {
		return arguments
	}
	socket := strings.TrimPrefix(containerHost, "unix://")
	if socket == "" {
		return arguments
	}
	tail := append([]string(nil), arguments[len(arguments)-3:]...)
	arguments = append(arguments[:len(arguments)-3],
		"--env", "DOCKER_HOST=unix:///var/run/docker.sock",
		"--env", "TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE=/var/run/docker.sock",
		"--volume", socket+":/var/run/docker.sock",
	)
	return append(arguments, tail...)
}

func (p project) down() error {
	command := exec.Command("podman", "rm", "--force", p.container)
	command.Stdout, command.Stderr = os.Stdout, os.Stderr
	if err := runWithTimeout(command, 120*time.Second); err != nil {
		return fmt.Errorf("stop toolbox container: %w", err)
	}
	return nil
}

func (p project) status() error {
	command := exec.Command("podman", "inspect", "--format", "{{.Name}} {{.State.Status}}", p.container)
	command.Stdout, command.Stderr = os.Stdout, os.Stderr
	if err := runWithTimeout(command, 15*time.Second); err != nil {
		return fmt.Errorf("inspect toolbox container: %w", err)
	}
	return nil
}

func (p project) shell() error {
	if err := p.waitHealthy(); err != nil {
		return err
	}
	command := exec.Command("podman", "exec", "--interactive", "--tty", p.container, "/bin/bash")
	command.Stdin, command.Stdout, command.Stderr = os.Stdin, os.Stdout, os.Stderr
	return command.Run()
}

func (p project) execInToolbox(binary string, args ...string) error {
	if len(args) == 0 {
		return fmt.Errorf("%s requires at least one argument", binary)
	}
	if err := p.waitHealthy(); err != nil {
		return err
	}
	command := exec.Command("podman", append([]string{"exec", p.container, binary}, args...)...)
	command.Stdin, command.Stdout, command.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := runWithTimeout(command, commandWait); err != nil {
		return fmt.Errorf("run %s in toolbox: %w", binary, err)
	}
	return nil
}

func (p project) waitHealthy() error {
	deadline := time.Now().Add(containerWait)
	for time.Now().Before(deadline) {
		inspect := exec.Command("podman", "inspect", "--format", "{{.State.Running}}", p.container)
		output, err := inspect.Output()
		if err == nil && strings.TrimSpace(string(output)) == "true" {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("toolbox container %s did not become running within %s", p.container, containerWait)
}

func gitPath(argument string) (string, error) {
	return gitOutput("rev-parse", "--path-format=absolute", argument)
}

func gitOutput(args ...string) (string, error) {
	command := exec.Command("git", append([]string{"-C", mustGetwd()}, args...)...)
	output, err := command.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func mustGetwd() string {
	workingDirectory, err := os.Getwd()
	if err != nil {
		return "."
	}
	return workingDirectory
}

func runWithTimeout(command *exec.Cmd, timeout time.Duration) error {
	if timeout <= 0 {
		return errors.New("command timeout must be positive")
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := command.Start(); err != nil {
		return err
	}
	wait := make(chan error, 1)
	go func() { wait <- command.Wait() }()
	select {
	case err := <-wait:
		return err
	case <-ctx.Done():
		_ = command.Process.Kill()
		<-wait
		return ctx.Err()
	}
}
