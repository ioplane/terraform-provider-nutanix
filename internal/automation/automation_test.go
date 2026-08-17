package automation

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreatePackageIsDeterministic(t *testing.T) {
	root := t.TempDir()
	binary := filepath.Join(root, "provider")
	license := filepath.Join(root, "LICENSE")
	if err := os.WriteFile(binary, []byte("provider"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(license, []byte("license"), 0o644); err != nil {
		t.Fatal(err)
	}
	first, _, err := CreatePackage(binary, license, filepath.Join(root, "first"), DefaultVersion, "linux", "amd64", SourceDateEpoch)
	if err != nil {
		t.Fatal(err)
	}
	second, _, err := CreatePackage(binary, license, filepath.Join(root, "second"), DefaultVersion, "linux", "amd64", SourceDateEpoch)
	if err != nil {
		t.Fatal(err)
	}
	firstBody, err := os.ReadFile(first)
	if err != nil {
		t.Fatal(err)
	}
	secondBody, err := os.ReadFile(second)
	if err != nil {
		t.Fatal(err)
	}
	if string(firstBody) != string(secondBody) {
		t.Fatal("deterministic packages differ")
	}
}

func TestDocsLinksCheckAcceptsGeneratedAnchors(t *testing.T) {
	root := t.TempDir()
	document := "# Root\n\n[child](#nestedatt--child)\n\n<a id=\"nestedatt--child\"></a>\n"
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte(document), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".git-index"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := validateLocalDocument(root, "README.md"); err != nil {
		t.Fatal(err)
	}
}

func validateLocalDocument(root, relative string) error {
	body, err := os.ReadFile(filepath.Join(root, relative))
	if err != nil {
		return err
	}
	for _, match := range linkPattern.FindAllSubmatch(body, -1) {
		target := string(match[1])
		if !strings.HasPrefix(target, "#") {
			continue
		}
		anchors, anchorErr := anchors(filepath.Join(root, relative))
		if anchorErr != nil {
			return anchorErr
		}
		if !anchors[strings.ToLower(strings.TrimPrefix(target, "#"))] {
			return errors.New("missing anchor")
		}
	}
	return nil
}

func TestVersionRankOrdersVersions(t *testing.T) {
	ga, ok, isGA := versionRank("v4.3")
	if !ok || !isGA || compareRank(ga, [4]int{4, 2, 3, 0}) <= 0 {
		t.Fatalf("unexpected GA rank: %v %v %v", ga, ok, isGA)
	}
	preview, ok, isGA := versionRank("v4.4.rc1")
	if !ok || isGA || compareRank(preview, ga) <= 0 {
		t.Fatalf("unexpected preview rank: %v %v %v", preview, ok, isGA)
	}
}

func TestParseToolVersionUsesScannerVersionForGovulncheck(t *testing.T) {
	output := []byte("Go: go1.26.5\nScanner: govulncheck@v1.6.0\n")
	version, err := parseToolVersion("govulncheck", output)
	if err != nil {
		t.Fatal(err)
	}
	if version != "1.6.0" {
		t.Fatalf("version = %q, want 1.6.0", version)
	}
}
