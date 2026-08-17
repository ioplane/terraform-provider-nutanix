// Command automation provides the repository's Go-native gates.
package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/ioplane/terraform-provider-nutanix/internal/automation"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "automation: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	root, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolve working directory: %w", err)
	}
	if len(args) == 0 {
		return fmt.Errorf("a command is required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Minute)
	defer cancel()
	switch args[0] {
	case "repo-check":
		if err = automation.RepositoryCheck(ctx, root); err == nil {
			fmt.Println("repository: ok")
		}
	case "staged-cache-check":
		if err = automation.StagedCacheCheck(ctx, root); err == nil {
			fmt.Println("staged: vendor artifact cache is absent from the index")
		}
	case "docs-links":
		if err = automation.DocsLinksCheck(ctx, root); err == nil {
			fmt.Println("docs-links: ok")
		}
	case "docs-generate":
		epoch := sourceDateEpoch()
		if err = automation.GenerateDocs(ctx, root, automation.DefaultVersion, epoch); err == nil {
			fmt.Println("provider-docs: generated")
		}
	case "package-test":
		epoch := sourceDateEpoch()
		var result string
		result, err = automation.PackageTest(ctx, root, automation.DefaultVersion, epoch)
		if err == nil {
			fmt.Println(result)
		}
	case "artifacts-count", "artifacts-discover", "artifacts-update", "artifacts-verify":
		mode := map[string]string{"artifacts-count": "count", "artifacts-discover": "discover", "artifacts-update": "update", "artifacts-verify": "verify"}[args[0]]
		var output string
		output, err = automation.ArtifactCommand(ctx, root, mode)
		if err == nil {
			fmt.Println(output)
		}
	case "pins-check":
		err = automation.PinsCheck(root)
		if err == nil {
			fmt.Println("pins: ok")
		}
	case "docs-check":
		err = automation.DocsCheck(ctx, root)
		if err == nil {
			fmt.Println("docs: ok")
		}
	case "tools-check":
		err = automation.ToolsCheck(ctx, root)
		if err == nil {
			fmt.Println("tools: ok")
		}
	case "terraform-version":
		if len(args) != 3 || args[1] != "--expected" {
			return fmt.Errorf("terraform-version requires --expected VERSION")
		}
		err = automation.TerraformVersion(ctx, root, args[2])
	default:
		err = fmt.Errorf("unknown command %q", args[0])
	}
	return err
}

func sourceDateEpoch() int64 {
	raw := os.Getenv("SOURCE_DATE_EPOCH")
	if raw == "" {
		return automation.SourceDateEpoch
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return automation.SourceDateEpoch
	}
	return value
}
