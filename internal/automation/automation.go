// Package automation contains repository gates and deterministic packaging.
package automation

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	ProviderAddress  = "registry.terraform.io/ioplane/nutanix"
	ProviderName     = "terraform-provider-nutanix"
	DefaultVersion   = "0.0.0-dev"
	SourceDateEpoch  = int64(1_700_000_000)
	NutanixAPIBase   = "https://developers.nutanix.com/api/v1/"
	MaxArtifactBytes = 64 * 1024 * 1024
)

var (
	govulncheckVersionPattern = regexp.MustCompile(`govulncheck@v?([0-9]+\.[0-9]+\.[0-9]+)`)
	semverPattern             = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(-[0-9A-Za-z.-]+)?$`)
	linkPattern               = regexp.MustCompile(`!?(?:\[[^\]\n]*\])\(([^)\n]+)\)`)
	headingPattern            = regexp.MustCompile(`(?m)^\s{0,3}#{1,6}\s+(.+?)\s*#*\s*$`)
	anchorPattern             = regexp.MustCompile(`(?i)<(?:a|span)\s+[^>]*(?:id|name)=["']([^"']+)["'][^>]*>`)
)

type CommandError struct {
	Name   string
	Output string
	Err    error
}

func (e *CommandError) Error() string {
	if e.Output == "" {
		return fmt.Sprintf("%s failed: %v", e.Name, e.Err)
	}
	return fmt.Sprintf("%s failed: %v: %s", e.Name, e.Err, strings.TrimSpace(e.Output))
}

func command(ctx context.Context, root, name string, args ...string) ([]byte, error) {
	if name == "git" {
		args = append([]string{"--work-tree=" + root}, args...)
	}
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = root
	if name == "git" {
		filtered := make([]string, 0, len(os.Environ()))
		for _, value := range os.Environ() {
			if strings.HasPrefix(value, "GIT_DIR=") || strings.HasPrefix(value, "GIT_COMMON_DIR=") || strings.HasPrefix(value, "GIT_WORK_TREE=") {
				continue
			}
			filtered = append(filtered, value)
		}
		cmd.Env = filtered
	}
	var output bytes.Buffer
	cmd.Stdout, cmd.Stderr = &output, &output
	if err := cmd.Run(); err != nil {
		return nil, &CommandError{Name: name, Output: output.String(), Err: err}
	}
	return output.Bytes(), nil
}

func TrackedFiles(ctx context.Context, root string, patterns ...string) ([]string, error) {
	args := []string{"ls-files", "-z", "--cached", "--others", "--exclude-standard"}
	if len(patterns) > 0 {
		args = append(args, "--")
		args = append(args, patterns...)
	}
	output, err := command(ctx, root, "git", args...)
	if err != nil {
		return nil, fmt.Errorf("inspect repository files: %w", err)
	}
	parts := strings.Split(string(output), "\x00")
	files := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			files = append(files, filepath.ToSlash(part))
		}
	}
	sort.Strings(files)
	return files, nil
}

func RepositoryCheck(ctx context.Context, root string) error {
	required := []string{".hadolint.yaml", ".release-please-manifest.json", ".goreleaser.yml", "CHANGELOG.md", "CODE_OF_CONDUCT.md", "CONTRIBUTING.md", "LICENSE", "README.md", "SECURITY.md", "Taskfile.yml", "dev", "docs/architecture.md", "docs/contract.md", "docs/index.md", "docs/release-process.md", "docs/roadmap.md", "docs/standards/dependencies.md", "docs/standards/go-1.26.md", "docs/standards/naming.md", "docs/standards/nutanix-artifacts.md", "docs/standards/testing.md", "deployments/containers/tool-assets.lock", "go.mod", "go.sum", "release-please-config.json"}
	files, err := TrackedFiles(ctx, root)
	if err != nil {
		return err
	}
	set := make(map[string]bool, len(files))
	for _, file := range files {
		set[file] = true
	}
	var diagnostics []string
	for _, file := range required {
		if _, err := os.Stat(filepath.Join(root, file)); err != nil {
			diagnostics = append(diagnostics, "required file missing: "+file)
		}
	}
	for _, file := range files {
		base := filepath.Base(file)
		if strings.Contains(file, ".cache/") || strings.Contains(file, ".terraform/") || base == ".env" || strings.HasPrefix(base, ".env.") || base == "tfplan" || base == "crash.log" || strings.HasSuffix(base, ".tfstate") || strings.HasSuffix(base, ".tfplan") || strings.HasSuffix(base, ".pem") || strings.HasSuffix(base, ".key") {
			diagnostics = append(diagnostics, "tracked forbidden path: "+file)
		}
	}
	if body, readErr := os.ReadFile(filepath.Join(root, "go.mod")); readErr == nil && regexp.MustCompile(`(?im)^\s*(?:require\s+)?github\.com/(?:nutanix|nutanix-cloud-native)/\S+`).Find(body) != nil {
		diagnostics = append(diagnostics, "go.mod contains a Nutanix SDK dependency")
	}
	if len(diagnostics) > 0 {
		sort.Strings(diagnostics)
		return errors.New(strings.Join(diagnostics, "\n"))
	}
	return nil
}

func StagedCacheCheck(ctx context.Context, root string) error {
	output, err := command(ctx, root, "git", "diff", "--cached", "--name-only", "-z")
	if err != nil {
		return fmt.Errorf("inspect staged paths: %w", err)
	}
	var rejected []string
	for _, file := range strings.Split(string(output), "\x00") {
		if strings.HasPrefix(file, ".cache/nutanix/artifacts/") {
			rejected = append(rejected, file)
		}
	}
	if len(rejected) > 0 {
		return fmt.Errorf("vendor artifact body is staged: %s", strings.Join(rejected, ", "))
	}
	return nil
}

func MarkdownFiles(ctx context.Context, root string) ([]string, error) {
	return TrackedFiles(ctx, root, "*.md", "*.markdown")
}

func slug(value string) string {
	value = strings.ToLower(value)
	var b strings.Builder
	lastDash := false
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if (r == ' ' || r == '\t') && !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

func anchors(path string) (map[string]bool, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	result := map[string]bool{}
	for _, match := range anchorPattern.FindAllSubmatch(body, -1) {
		result[strings.ToLower(string(match[1]))] = true
	}
	counts := map[string]int{}
	for _, match := range headingPattern.FindAllSubmatch(body, -1) {
		base := slug(string(match[1]))
		if base == "" {
			continue
		}
		n := counts[base]
		counts[base] = n + 1
		if n > 0 {
			base = fmt.Sprintf("%s-%d", base, n)
		}
		result[base] = true
	}
	return result, nil
}

func DocsLinksCheck(ctx context.Context, root string) error {
	files, err := MarkdownFiles(ctx, root)
	if err != nil {
		return err
	}
	anchorCache := map[string]map[string]bool{}
	var diagnostics []string
	for _, relative := range files {
		if !strings.HasSuffix(strings.ToLower(relative), ".md") && !strings.HasSuffix(strings.ToLower(relative), ".markdown") {
			continue
		}
		path := filepath.Join(root, filepath.FromSlash(relative))
		body, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		for _, match := range linkPattern.FindAllSubmatch(body, -1) {
			target := strings.TrimSpace(string(match[1]))
			if i := strings.IndexAny(target, " \t"); i >= 0 {
				target = target[:i]
			}
			lower := strings.ToLower(target)
			if strings.HasPrefix(target, "<") && strings.Contains(target, ">") {
				target = target[1:strings.Index(target, ">")]
			}
			if strings.HasPrefix(lower, "http:") || strings.HasPrefix(lower, "https:") || strings.HasPrefix(lower, "mailto:") || strings.HasPrefix(lower, "tel:") || strings.HasPrefix(target, "//") {
				continue
			}
			parts := strings.SplitN(target, "#", 2)
			candidate := filepath.Join(filepath.Dir(path), filepath.FromSlash(parts[0]))
			if parts[0] == "" {
				candidate = path
			}
			resolved, resolveErr := filepath.Abs(candidate)
			if resolveErr != nil {
				return resolveErr
			}
			cleanRoot, _ := filepath.Abs(root)
			if !strings.HasPrefix(resolved, cleanRoot+string(filepath.Separator)) && resolved != cleanRoot {
				diagnostics = append(diagnostics, relative+": local link escapes repository: "+target)
				continue
			}
			if _, statErr := os.Stat(resolved); statErr != nil {
				diagnostics = append(diagnostics, relative+": missing local link target: "+target)
				continue
			}
			if len(parts) == 2 && strings.HasSuffix(strings.ToLower(resolved), ".md") {
				if anchorCache[resolved] == nil {
					anchorCache[resolved], err = anchors(resolved)
					if err != nil {
						return err
					}
				}
				if !anchorCache[resolved][strings.ToLower(parts[1])] {
					diagnostics = append(diagnostics, relative+": missing local link fragment: "+target)
				}
			}
		}
	}
	if len(diagnostics) > 0 {
		sort.Strings(diagnostics)
		return errors.New(strings.Join(diagnostics, "\n"))
	}
	return nil
}

func BuildProvider(ctx context.Context, root, output, version string) error {
	if !semverPattern.MatchString(version) {
		return errors.New("unsafe provider version")
	}
	_, err := command(ctx, root, "go", "build", "-buildvcs=false", "-trimpath", "-ldflags", "-s -w -X main.version="+version, "-o", output, "./cmd/terraform-provider-nutanix")
	return err
}

func zipTime(epoch int64) (time.Time, error) {
	t := time.Unix(epoch, 0).UTC()
	if t.Year() < 1980 || t.Year() > 2107 {
		return time.Time{}, errors.New("unsafe SOURCE_DATE_EPOCH")
	}
	return t.Truncate(2 * time.Second), nil
}

func CreatePackage(binary, license, outputDir, version, goos, goarch string, epoch int64) (string, string, error) {
	if !semverPattern.MatchString(version) {
		return "", "", errors.New("unsafe provider version")
	}
	if !regexp.MustCompile(`^[a-z0-9]+$`).MatchString(goos) || !regexp.MustCompile(`^[a-z0-9]+$`).MatchString(goarch) {
		return "", "", errors.New("unsafe provider platform")
	}
	providerBody, err := os.ReadFile(binary)
	if err != nil {
		return "", "", fmt.Errorf("read provider binary: %w", err)
	}
	licenseBody, err := os.ReadFile(license)
	if err != nil {
		return "", "", fmt.Errorf("read license: %w", err)
	}
	stamp, err := zipTime(epoch)
	if err != nil {
		return "", "", err
	}
	var buffer bytes.Buffer
	archive := zip.NewWriter(&buffer)
	entries := []struct {
		name string
		body []byte
		mode uint32
	}{{"LICENSE", licenseBody, 0o644}, {ProviderName + "_v" + version, providerBody, 0o755}}
	for _, entry := range entries {
		header := &zip.FileHeader{Name: entry.name, Method: zip.Deflate, Modified: stamp}
		header.SetMode(os.FileMode(entry.mode))
		writer, writeErr := archive.CreateHeader(header)
		if writeErr != nil {
			return "", "", writeErr
		}
		if _, writeErr = writer.Write(entry.body); writeErr != nil {
			return "", "", writeErr
		}
	}
	if err = archive.Close(); err != nil {
		return "", "", err
	}
	body := buffer.Bytes()
	digest := sha256.Sum256(body)
	digestText := hex.EncodeToString(digest[:])
	if err = os.MkdirAll(outputDir, 0o755); err != nil {
		return "", "", err
	}
	name := fmt.Sprintf("%s_%s_%s_%s.zip", ProviderName, version, goos, goarch)
	archivePath := filepath.Join(outputDir, name)
	if err = os.WriteFile(archivePath, body, 0o644); err != nil {
		return "", "", err
	}
	checksums := filepath.Join(outputDir, "SHA256SUMS")
	if err = os.WriteFile(checksums, []byte(digestText+"  "+name+"\n"), 0o644); err != nil {
		return "", "", err
	}
	return archivePath, checksums, nil
}

func verifySchema(payload []byte) error {
	var document map[string]json.RawMessage
	if err := json.Unmarshal(payload, &document); err != nil {
		return errors.New("terraform schema output is invalid JSON")
	}
	raw, ok := document["provider_schemas"]
	if !ok {
		return errors.New("terraform schema output shape is invalid")
	}
	var schemas map[string]json.RawMessage
	if err := json.Unmarshal(raw, &schemas); err != nil || len(schemas) != 1 {
		return errors.New("terraform schema provider address differs")
	}
	if _, ok = schemas[ProviderAddress]; !ok {
		return errors.New("terraform schema provider address differs")
	}
	return nil
}

func OfflineSchema(ctx context.Context, root, temporary, version string, epoch int64) ([]byte, error) {
	goos, goarch := runtime.GOOS, runtime.GOARCH
	build := filepath.Join(temporary, ProviderName)
	if err := os.MkdirAll(temporary, 0o755); err != nil {
		return nil, err
	}
	if err := BuildProvider(ctx, root, build, version); err != nil {
		return nil, fmt.Errorf("provider build: %w", err)
	}
	archiveDir := filepath.Join(temporary, "package")
	archive, _, err := CreatePackage(build, filepath.Join(root, "LICENSE"), archiveDir, version, goos, goarch, epoch)
	if err != nil {
		return nil, err
	}
	mirror := filepath.Join(temporary, "mirror", "registry.terraform.io", "ioplane", "nutanix", version, goos+"_"+goarch)
	if err = os.MkdirAll(mirror, 0o755); err != nil {
		return nil, err
	}
	archiveReader, err := zip.OpenReader(archive)
	if err != nil {
		return nil, err
	}
	defer func() { _ = archiveReader.Close() }()
	for _, entry := range archiveReader.File {
		if entry.Name == "LICENSE" {
			continue
		}
		source, openErr := entry.Open()
		if openErr != nil {
			return nil, openErr
		}
		body, readErr := io.ReadAll(source)
		_ = source.Close()
		if readErr != nil {
			return nil, readErr
		}
		target := filepath.Join(mirror, entry.Name)
		if writeErr := os.WriteFile(target, body, 0o755); writeErr != nil {
			return nil, writeErr
		}
	}
	configuration := filepath.Join(temporary, "configuration")
	if err = os.MkdirAll(configuration, 0o755); err != nil {
		return nil, err
	}
	cliConfig := filepath.Join(temporary, "terraform.tfrc")
	config := fmt.Sprintf("provider_installation {\n  filesystem_mirror {\n    path = %q\n    include = [%q]\n  }\n}\n", filepath.Join(temporary, "mirror"), ProviderAddress)
	if err = os.WriteFile(cliConfig, []byte(config), 0o644); err != nil {
		return nil, err
	}
	mainTF := fmt.Sprintf("terraform {\n  required_providers {\n    nutanix = {\n      source = \"ioplane/nutanix\"\n      version = \"=%s\"\n    }\n  }\n}\nprovider \"nutanix\" {}\n", version)
	if err = os.WriteFile(filepath.Join(configuration, "main.tf"), []byte(mainTF), 0o644); err != nil {
		return nil, err
	}
	env := append(os.Environ(), "TF_CLI_CONFIG_FILE="+cliConfig, "TF_DATA_DIR="+filepath.Join(temporary, "terraform-data"), "TF_IN_AUTOMATION=1", "TF_INPUT=0", "CHECKPOINT_DISABLE=1")
	init := exec.CommandContext(ctx, "terraform", "init", "-backend=false", "-input=false", "-no-color")
	init.Dir, init.Env = configuration, env
	if output, runErr := init.CombinedOutput(); runErr != nil {
		return nil, &CommandError{Name: "terraform init", Output: string(output), Err: runErr}
	}
	schema := exec.CommandContext(ctx, "terraform", "providers", "schema", "-json")
	schema.Dir, schema.Env = configuration, env
	payload, runErr := schema.Output()
	if runErr != nil {
		return nil, fmt.Errorf("terraform schema: %w", runErr)
	}
	if err = verifySchema(payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func PackageTest(ctx context.Context, root, version string, epoch int64) (string, error) {
	temporary, err := os.MkdirTemp("", "nutanix-package-")
	if err != nil {
		return "", err
	}
	defer func() { _ = os.RemoveAll(temporary) }()
	first, second := filepath.Join(temporary, "first"), filepath.Join(temporary, "second")
	if err = os.MkdirAll(first, 0o755); err != nil {
		return "", err
	}
	if err = os.MkdirAll(second, 0o755); err != nil {
		return "", err
	}
	firstBin, secondBin := filepath.Join(first, ProviderName), filepath.Join(second, ProviderName)
	if err = BuildProvider(ctx, root, firstBin, version); err != nil {
		return "", err
	}
	if err = BuildProvider(ctx, root, secondBin, version); err != nil {
		return "", err
	}
	goos, goarch := runtime.GOOS, runtime.GOARCH
	firstArchive, _, err := CreatePackage(firstBin, filepath.Join(root, "LICENSE"), filepath.Join(temporary, "package-first"), version, goos, goarch, epoch)
	if err != nil {
		return "", err
	}
	secondArchive, _, err := CreatePackage(secondBin, filepath.Join(root, "LICENSE"), filepath.Join(temporary, "package-second"), version, goos, goarch, epoch)
	if err != nil {
		return "", err
	}
	firstBody, err := os.ReadFile(firstArchive)
	if err != nil {
		return "", err
	}
	secondBody, err := os.ReadFile(secondArchive)
	if err != nil {
		return "", err
	}
	if !bytes.Equal(firstBody, secondBody) {
		return "", errors.New("repeated provider packages differ")
	}
	if _, err = OfflineSchema(ctx, root, filepath.Join(temporary, "offline"), version, epoch); err != nil {
		return "", err
	}
	digest := sha256.Sum256(firstBody)
	return fmt.Sprintf("package: ok (%s, protocol 6 schema)", hex.EncodeToString(digest[:])), nil
}

func GenerateDocs(ctx context.Context, root, version string, epoch int64) error {
	temporary, err := os.MkdirTemp("", "nutanix-docs-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(temporary) }()
	schema, err := OfflineSchema(ctx, root, filepath.Join(temporary, "schema"), version, epoch)
	if err != nil {
		return err
	}
	var document map[string]json.RawMessage
	if err = json.Unmarshal(schema, &document); err != nil {
		return err
	}
	var schemas map[string]json.RawMessage
	if err = json.Unmarshal(document["provider_schemas"], &schemas); err != nil {
		return err
	}
	selected, ok := schemas[ProviderAddress]
	if !ok {
		return errors.New("offline provider schema address differs")
	}
	document["provider_schemas"], _ = json.Marshal(map[string]json.RawMessage{"registry.terraform.io/hashicorp/nutanix": selected})
	schemaPath := filepath.Join(temporary, "provider-schema.json")
	encoded, _ := json.Marshal(document)
	if err = os.WriteFile(schemaPath, append(encoded, '\n'), 0o644); err != nil {
		return err
	}
	rendered := filepath.Join(temporary, "rendered")
	providerDir := filepath.Join(root, "cmd", "terraform-provider-nutanix")
	relativeTemporary, err := filepath.Rel(providerDir, temporary)
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, "tfplugindocs", "generate", "--provider-dir", ".", "--provider-name", "nutanix", "--rendered-provider-name", "Nutanix", "--examples-dir", "../../examples", "--providers-schema", filepath.ToSlash(filepath.Join(relativeTemporary, "provider-schema.json")), "--rendered-website-dir", filepath.ToSlash(filepath.Join(relativeTemporary, "rendered")), "--website-temp-dir", filepath.ToSlash(filepath.Join(relativeTemporary, "website-work")))
	cmd.Dir = providerDir
	output, runErr := cmd.CombinedOutput()
	if runErr != nil {
		return &CommandError{Name: "tfplugindocs", Output: string(output), Err: runErr}
	}
	return copyGeneratedDocs(root, rendered)
}

func copyGeneratedDocs(root, rendered string) error {
	var copied int
	err := filepath.Walk(rendered, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(rendered, path)
		if err != nil {
			return err
		}
		destination := filepath.Join(root, "docs", relative)
		if err = os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
			return err
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err = os.WriteFile(destination, body, 0o644); err != nil {
			return err
		}
		copied++
		return nil
	})
	if err != nil {
		return err
	}
	if copied == 0 {
		return errors.New("tfplugindocs generated no files")
	}
	return nil
}

func ArtifactsCount(root string) (int, error) {
	body, err := os.ReadFile(filepath.Join(root, "specs/nutanix/manifest.json"))
	if err != nil {
		return 0, err
	}
	var manifest struct {
		Namespaces []json.RawMessage `json:"namespaces"`
	}
	if err = json.Unmarshal(body, &manifest); err != nil {
		return 0, err
	}
	return len(manifest.Namespaces), nil
}

var expectedTools = map[string]string{
	"go": "1.26.6", "terraform": "1.15.8", "task": "3.52.0", "beads": "1.1.2",
	"golangci-lint": "2.12.2", "goreleaser": "2.17.1", "syft": "1.50.0",
	"tfplugindocs": "0.25.0", "govulncheck": "1.6.0", "gopls": "0.23.0",
	"gh": "2.97.0", "hadolint": "2.15.1",
}

func toolVersion(ctx context.Context, root, name string, args ...string) (string, error) {
	output, err := command(ctx, root, name, args...)
	if err != nil {
		return "", err
	}
	return parseToolVersion(name, output)
}

func parseToolVersion(name string, output []byte) (string, error) {
	if name == "govulncheck" {
		match := govulncheckVersionPattern.FindSubmatch(output)
		if match == nil {
			return "", fmt.Errorf("%s did not report a semantic version", name)
		}
		return string(match[1]), nil
	}
	match := regexp.MustCompile(`\d+\.\d+\.\d+`).Find(output)
	if match == nil {
		return "", fmt.Errorf("%s did not report a semantic version", name)
	}
	return string(match), nil
}

func ToolsCheck(ctx context.Context, root string) error {
	commands := map[string]struct {
		name string
		args []string
	}{
		"go": {"go", []string{"version"}}, "terraform": {"terraform", []string{"version"}},
		"task": {"task", []string{"--version"}}, "beads": {"bd", []string{"version"}},
		"golangci-lint": {"golangci-lint", []string{"version"}}, "goreleaser": {"goreleaser", []string{"--version"}},
		"syft": {"syft", []string{"version"}}, "tfplugindocs": {"tfplugindocs", []string{"--version"}},
		"govulncheck": {"govulncheck", []string{"-version"}}, "gopls": {"gopls", []string{"version"}},
		"gh": {"gh", []string{"--version"}}, "hadolint": {"hadolint", []string{"--version"}},
	}
	var diagnostics []string
	for key, expected := range expectedTools {
		spec := commands[key]
		observed, err := toolVersion(ctx, root, spec.name, spec.args...)
		if err != nil {
			diagnostics = append(diagnostics, "cannot inspect tool "+key)
			continue
		}
		if observed != expected {
			diagnostics = append(diagnostics, "tool version differs: "+key)
		}
	}
	if len(diagnostics) > 0 {
		sort.Strings(diagnostics)
		return errors.New(strings.Join(diagnostics, "\n"))
	}
	return nil
}

func PinsCheck(root string) error {
	paths := []string{"deployments/containers/Containerfile.dev", "deployments/compose/compose.dev.yml", ".github/workflows/ci.yml", ".github/workflows/release.yml", ".github/dependabot.yml"}
	var diagnostics []string
	for _, relative := range paths {
		body, err := os.ReadFile(filepath.Join(root, relative))
		if err != nil {
			diagnostics = append(diagnostics, "cannot read "+relative)
			continue
		}
		text := string(body)
		if strings.Contains(text, "python") || strings.Contains(text, "uv") {
			diagnostics = append(diagnostics, "Python automation reference remains: "+relative)
		}
	}
	if len(diagnostics) > 0 {
		return errors.New(strings.Join(diagnostics, "\n"))
	}
	return nil
}

func DocsCheck(ctx context.Context, root string) error {
	files, err := TrackedFiles(ctx, root, "*.yml", "*.yaml")
	if err != nil {
		return err
	}
	for _, relative := range files {
		body, readErr := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
		if readErr != nil {
			if errors.Is(readErr, os.ErrNotExist) {
				continue
			}
			return readErr
		}
		var document any
		if decodeErr := yaml.Unmarshal(body, &document); decodeErr != nil {
			return fmt.Errorf("invalid YAML %s: %w", relative, decodeErr)
		}
	}
	if err := DocsLinksCheck(ctx, root); err != nil {
		return err
	}
	return nil
}

func TerraformVersion(ctx context.Context, root, expected string) error {
	observed, err := toolVersion(ctx, root, "terraform", "version")
	if err != nil {
		return err
	}
	if observed != expected {
		return fmt.Errorf("terraform version differs: got %s, want %s", observed, expected)
	}
	return nil
}

var (
	namespacePattern = regexp.MustCompile(`^[a-z][a-z0-9]*$`)
	gaPattern        = regexp.MustCompile(`^v(\d+)\.(\d+)$`)
	previewPattern   = regexp.MustCompile(`^v(\d+)\.(\d+)\.(a|b|rc)(\d+)$`)
)

type artifactClient struct {
	base *url.URL
	http *http.Client
}

func newArtifactClient(raw string) (*artifactClient, error) {
	if !strings.HasSuffix(raw, "/") {
		return nil, errors.New("base URL must end with '/'")
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return nil, errors.New("base URL is invalid")
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Path != "/api/v1/" {
		return nil, errors.New("base URL contains forbidden components")
	}
	if parsed.Scheme != "https" || parsed.Host != "developers.nutanix.com" {
		return nil, errors.New("official base URL must use developers.nutanix.com over HTTPS")
	}
	client := &http.Client{Timeout: 60 * time.Second}
	client.CheckRedirect = func(req *http.Request, _ []*http.Request) error { return validateArtifactURL(parsed, req.URL) }
	return &artifactClient{base: parsed, http: client}, nil
}

func validateArtifactURL(base, candidate *url.URL) error {
	if candidate.Scheme != base.Scheme || candidate.Host != base.Host || candidate.User != nil || candidate.RawQuery != "" || candidate.Fragment != "" || !strings.HasPrefix(candidate.Path, base.Path) || strings.Contains(candidate.Path, "..") {
		return errors.New("URL is outside allowed API prefix")
	}
	return nil
}

func (c *artifactClient) get(ctx context.Context, target, label string) (string, []byte, error) {
	parsed, err := url.Parse(target)
	if err != nil {
		return "", nil, fmt.Errorf("%s: invalid URL", label)
	}
	if err = validateArtifactURL(c.base, parsed); err != nil {
		return "", nil, fmt.Errorf("%s: %w", label, err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return "", nil, err
	}
	request.Header.Set("Accept", "*/*")
	request.Header.Set("User-Agent", "ioplane-terraform-provider-nutanix-artifact-lock/0.0.0-dev")
	response, err := c.http.Do(request)
	if err != nil {
		return "", nil, fmt.Errorf("%s: request failed (%T)", label, err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", nil, fmt.Errorf("%s: HTTP %d", label, response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, MaxArtifactBytes+1))
	if err != nil {
		return "", nil, fmt.Errorf("%s: read failed", label)
	}
	if len(body) > MaxArtifactBytes {
		return "", nil, fmt.Errorf("%s: response exceeds %d bytes", label, MaxArtifactBytes)
	}
	return strings.ToLower(strings.TrimSpace(strings.Split(response.Header.Get("Content-Type"), ";")[0])), body, nil
}

func (c *artifactClient) json(ctx context.Context, target, label string, destination any) error {
	media, body, err := c.get(ctx, target, label)
	if err != nil {
		return err
	}
	if !strings.Contains(media, "json") {
		return fmt.Errorf("%s: rejected media type %s", label, media)
	}
	if err = json.Unmarshal(body, destination); err != nil {
		return fmt.Errorf("%s: invalid JSON response", label)
	}
	return nil
}

type artifactSelection struct {
	Name      string                      `json:"name"`
	Version   string                      `json:"version"`
	Stability string                      `json:"stability"`
	Artifacts map[string]artifactMetadata `json:"artifacts"`
}
type artifactMetadata struct {
	URL       string `json:"url"`
	MediaType string `json:"media_type,omitempty"`
	Bytes     int    `json:"bytes,omitempty"`
	SHA256    string `json:"sha256,omitempty"`
	Path      string `json:"path,omitempty"`
}

func versionRank(version string) ([4]int, bool, bool) {
	if match := gaPattern.FindStringSubmatch(version); match != nil {
		return [4]int{atoi(match[1]), atoi(match[2]), 3, 0}, true, true
	}
	if match := previewPattern.FindStringSubmatch(version); match != nil {
		phase := map[string]int{"a": 0, "b": 1, "rc": 2}[match[3]]
		return [4]int{atoi(match[1]), atoi(match[2]), phase, atoi(match[4])}, true, false
	}
	return [4]int{}, false, false
}

func atoi(value string) int { parsed, _ := strconv.Atoi(value); return parsed }

func discoverArtifacts(ctx context.Context, client *artifactClient) ([]artifactSelection, error) {
	var registry struct {
		Namespaces []struct {
			Name string `json:"name"`
		} `json:"namespaces"`
	}
	if err := client.json(ctx, client.base.JoinPath("namespaces/").String(), "registry", &registry); err != nil {
		return nil, err
	}
	if len(registry.Namespaces) == 0 {
		return nil, errors.New("registry: namespaces array is empty")
	}
	selected := make([]artifactSelection, 0, len(registry.Namespaces))
	seen := map[string]bool{}
	for _, namespace := range registry.Namespaces {
		if !namespacePattern.MatchString(namespace.Name) || seen[namespace.Name] {
			return nil, errors.New("registry: unsafe or duplicate namespace")
		}
		seen[namespace.Name] = true
		var versions struct {
			Namespace string           `json:"namespace"`
			Versions  []map[string]any `json:"versions"`
		}
		endpoint := client.base.JoinPath("namespaces", namespace.Name, "versions/").String()
		if err := client.json(ctx, endpoint, "versions "+namespace.Name, &versions); err != nil {
			return nil, err
		}
		if versions.Namespace != namespace.Name || len(versions.Versions) == 0 {
			return nil, fmt.Errorf("versions %s: invalid response", namespace.Name)
		}
		var chosen map[string]any
		var rank [4]int
		ga := false
		for _, entry := range versions.Versions {
			raw, ok := entry["version"].(string)
			if !ok {
				return nil, fmt.Errorf("versions %s: version must be a string", namespace.Name)
			}
			candidateRank, supported, candidateGA := versionRank(raw)
			if !supported || (candidateGA && ga) || (!candidateGA && ga) {
				continue
			}
			if !ga || candidateGA && compareRank(candidateRank, rank) > 0 || !candidateGA && compareRank(candidateRank, rank) > 0 {
				chosen, rank, ga = entry, candidateRank, candidateGA
			}
		}
		if chosen == nil {
			return nil, fmt.Errorf("versions %s: no supported GA or preview version", namespace.Name)
		}
		version, _ := chosen["version"].(string)
		stability := "preview"
		if ga {
			stability = "ga"
		}
		selection := artifactSelection{Name: namespace.Name, Version: version, Stability: stability, Artifacts: map[string]artifactMetadata{}}
		link, ok := chosen["link"].(string)
		if !ok || link == "" {
			return nil, fmt.Errorf("versions %s: selected version has no OpenAPI URL", namespace.Name)
		}
		if err := setArtifactURL(client.base, selection.Artifacts, "openapi", link); err != nil {
			return nil, err
		}
		if link, ok = chosen["postmanCollectionLink"].(string); ok && link != "" {
			if err := setArtifactURL(client.base, selection.Artifacts, "postman", link); err != nil {
				return nil, err
			}
		}
		if references, ok := chosen["errDocLinks"].([]any); ok {
			for _, raw := range references {
				reference, ok := raw.(map[string]any)
				if !ok || reference["locale"] != "en_US" {
					continue
				}
				link, ok := reference["link"].(string)
				if !ok {
					return nil, fmt.Errorf("versions %s: invalid English error URL", namespace.Name)
				}
				if _, exists := selection.Artifacts["errors"]; exists {
					return nil, fmt.Errorf("versions %s: duplicate English error URLs", namespace.Name)
				}
				if err := setArtifactURL(client.base, selection.Artifacts, "errors", link); err != nil {
					return nil, err
				}
			}
		}
		selected = append(selected, selection)
	}
	sort.Slice(selected, func(i, j int) bool { return selected[i].Name < selected[j].Name })
	return selected, nil
}

func compareRank(left, right [4]int) int {
	for i := range left {
		if left[i] < right[i] {
			return -1
		}
		if left[i] > right[i] {
			return 1
		}
	}
	return 0
}
func setArtifactURL(base *url.URL, artifacts map[string]artifactMetadata, kind, raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil || validateArtifactURL(base, parsed) != nil {
		return errors.New("URL is outside allowed API prefix")
	}
	artifacts[kind] = artifactMetadata{URL: raw}
	return nil
}

func sameArtifactSelection(left, right []artifactSelection) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index].Name != right[index].Name || left[index].Version != right[index].Version || left[index].Stability != right[index].Stability || len(left[index].Artifacts) != len(right[index].Artifacts) {
			return false
		}
		for kind, metadata := range left[index].Artifacts {
			other, ok := right[index].Artifacts[kind]
			if !ok || metadata.URL != other.URL {
				return false
			}
		}
	}
	return true
}

func validateArtifact(kind, media string, body []byte) error {
	if kind == "openapi" {
		if media != "application/yaml" && media != "application/x-yaml" && media != "text/plain" && media != "text/x-yaml" && media != "text/yaml" {
			return fmt.Errorf("OpenAPI rejected media type %s", media)
		}
		var document map[string]any
		if err := yaml.Unmarshal(body, &document); err != nil {
			return errors.New("OpenAPI invalid YAML")
		}
		version, _ := document["openapi"].(string)
		if !strings.HasPrefix(version, "3.0") && !strings.HasPrefix(version, "3.1") {
			return errors.New("OpenAPI 3.0 or 3.1 metadata is required")
		}
		return nil
	}
	jsonMedia := strings.Contains(media, "json")
	postmanText := kind == "postman" && media == "text/plain"
	if !jsonMedia && !postmanText {
		return fmt.Errorf("%s rejected media type %s", kind, media)
	}
	var document any
	if err := json.Unmarshal(body, &document); err != nil {
		return fmt.Errorf("%s invalid JSON", kind)
	}
	if kind == "postman" {
		object, ok := document.(map[string]any)
		if !ok {
			return errors.New("postman document must be an object with an item array")
		}
		if _, ok = object["item"].([]any); !ok {
			return errors.New("postman document must be an object with an item array")
		}
	}
	if kind == "errors" {
		if _, ok := document.([]any); !ok {
			return errors.New("error reference must be a JSON array")
		}
	}
	return nil
}

func ArtifactCommand(ctx context.Context, root, mode string) (string, error) {
	manifestPath := filepath.Join(root, "specs/nutanix/manifest.json")
	cacheRoot := filepath.Join(root, ".cache/nutanix/artifacts")
	if mode == "count" {
		count, err := ArtifactsCount(root)
		return strconv.Itoa(count), err
	}
	client, err := newArtifactClient(NutanixAPIBase)
	if err != nil {
		return "", err
	}
	if mode == "discover" {
		selection, err := discoverArtifacts(ctx, client)
		if err != nil {
			return "", err
		}
		body, _ := json.MarshalIndent(selection, "", "  ")
		return string(body), nil
	}
	if mode == "update" {
		selection, err := discoverArtifacts(ctx, client)
		if err != nil {
			return "", err
		}
		for i := range selection {
			for kind := range selection[i].Artifacts {
				media, body, getErr := client.get(ctx, selection[i].Artifacts[kind].URL, selection[i].Name+" "+kind)
				if getErr != nil {
					return "", getErr
				}
				if validateErr := validateArtifact(kind, media, body); validateErr != nil {
					return "", validateErr
				}
				digest := sha256.Sum256(body)
				path := filepath.Join(cacheRoot, selection[i].Name, selection[i].Version, map[string]string{"openapi": "openapi.yaml", "postman": "postman.json", "errors": "errors.json"}[kind])
				if writeErr := os.MkdirAll(filepath.Dir(path), 0o755); writeErr != nil {
					return "", writeErr
				}
				if writeErr := os.WriteFile(path, body, 0o644); writeErr != nil {
					return "", writeErr
				}
				metadata := selection[i].Artifacts[kind]
				metadata.MediaType, metadata.Bytes, metadata.SHA256, metadata.Path = media, len(body), hex.EncodeToString(digest[:]), filepath.ToSlash(filepath.Join(selection[i].Name, selection[i].Version, filepath.Base(path)))
				selection[i].Artifacts[kind] = metadata
			}
		}
		encoded, _ := json.MarshalIndent(struct {
			SchemaVersion int                 `json:"schema_version"`
			Source        string              `json:"source"`
			Namespaces    []artifactSelection `json:"namespaces"`
		}{1, NutanixAPIBase, selection}, "", "  ")
		if err = os.WriteFile(manifestPath, append(encoded, '\n'), 0o644); err != nil {
			return "", err
		}
		return fmt.Sprintf("artifacts: locked %d namespaces", len(selection)), nil
	}
	body, err := os.ReadFile(manifestPath)
	if err != nil {
		return "", fmt.Errorf("manifest: cannot read %s", manifestPath)
	}
	var locked struct {
		SchemaVersion int                 `json:"schema_version"`
		Source        string              `json:"source"`
		Namespaces    []artifactSelection `json:"namespaces"`
	}
	if err = json.Unmarshal(body, &locked); err != nil || locked.SchemaVersion != 1 || locked.Source != NutanixAPIBase {
		return "", errors.New("manifest: invalid lock shape or source")
	}
	live, err := discoverArtifacts(ctx, client)
	if err != nil {
		return "", err
	}
	if !sameArtifactSelection(locked.Namespaces, live) {
		return "", errors.New("manifest: live namespace selection mismatch")
	}
	for _, namespace := range locked.Namespaces {
		for kind, metadata := range namespace.Artifacts {
			media, body, getErr := client.get(ctx, metadata.URL, namespace.Name+" "+kind)
			if getErr != nil {
				return "", getErr
			}
			if validateErr := validateArtifact(kind, media, body); validateErr != nil {
				return "", validateErr
			}
			digest := sha256.Sum256(body)
			if metadata.MediaType != media || metadata.Bytes != len(body) || metadata.SHA256 != hex.EncodeToString(digest[:]) {
				return "", fmt.Errorf("%s %s %s: locked metadata mismatch", namespace.Name, namespace.Version, kind)
			}
		}
	}
	return fmt.Sprintf("artifacts: verified %d namespaces", len(locked.Namespaces)), nil
}
