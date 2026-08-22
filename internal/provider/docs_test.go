package provider

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

// repoRoot resolves the repository root from this package's directory.
func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolving the repository root: %v", err)
	}
	return root
}

// TestEveryTypeHasADocsPage keeps the documentation in step with the code:
// registering a resource or data source without writing its page fails here
// rather than shipping a provider with a missing Registry page.
func TestEveryTypeHasADocsPage(t *testing.T) {
	ctx := context.Background()
	root := repoRoot(t)
	p := New("test")()

	for _, newResource := range p.Resources(ctx) {
		var metadata resource.MetadataResponse
		newResource().Metadata(ctx, resource.MetadataRequest{ProviderTypeName: "revenuecat"}, &metadata)

		name := strings.TrimPrefix(metadata.TypeName, "revenuecat_")
		path := filepath.Join(root, "docs", "resources", name+".md")

		if _, err := os.Stat(path); err != nil {
			t.Errorf("resource %s has no docs page at docs/resources/%s.md", metadata.TypeName, name)
			continue
		}
		assertDocsPage(t, path, metadata.TypeName, true)
	}

	for _, newDataSource := range p.DataSources(ctx) {
		var metadata datasource.MetadataResponse
		newDataSource().Metadata(ctx, datasource.MetadataRequest{ProviderTypeName: "revenuecat"}, &metadata)

		name := strings.TrimPrefix(metadata.TypeName, "revenuecat_")
		path := filepath.Join(root, "docs", "data-sources", name+".md")

		if _, err := os.Stat(path); err != nil {
			t.Errorf("data source %s has no docs page at docs/data-sources/%s.md", metadata.TypeName, name)
			continue
		}
		assertDocsPage(t, path, metadata.TypeName, false)
	}
}

// assertDocsPage checks a page carries the sections practitioners look for.
func assertDocsPage(t *testing.T, path, typeName string, isResource bool) {
	t.Helper()

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	content := string(raw)

	for _, section := range []string{"## Example Usage", "## Schema"} {
		if !strings.Contains(content, section) {
			t.Errorf("%s is missing the %q section", path, section)
		}
	}
	if !strings.Contains(content, typeName) {
		t.Errorf("%s never names the type %q it documents", path, typeName)
	}
	// Every managed resource supports import, so every page must say how.
	if isResource && !strings.Contains(content, "## Import") {
		t.Errorf("%s is missing the %q section, but the resource supports import", path, "## Import")
	}
}

// TestProviderIndexDocumentsEveryArgument checks the provider page lists each
// configuration argument the schema accepts.
func TestProviderIndexDocumentsEveryArgument(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(repoRoot(t), "docs", "index.md"))
	if err != nil {
		t.Fatalf("reading docs/index.md: %v", err)
	}
	content := string(raw)

	for _, argument := range []string{"api_key", "base_url", "max_retries", "request_timeout_seconds"} {
		if !strings.Contains(content, argument) {
			t.Errorf("docs/index.md does not document the %q provider argument", argument)
		}
	}

	// The caveat is the single most important thing on the page, and it has to
	// stay accurate in both directions: it must say what is verified and what
	// is not, so a reader neither over-trusts nor under-trusts the provider.
	for _, claim := range []string{
		"verified against a mock of the RevenueCat API, not against the live service",
		"self-consistent under Terraform",
	} {
		if !strings.Contains(content, claim) {
			t.Errorf("docs/index.md no longer states %q; the status caveat must stay accurate", claim)
		}
	}
}

// TestExamplesParse checks every shipped example is well-formed HCL, so a
// broken snippet cannot reach the Registry.
func TestExamplesParse(t *testing.T) {
	root := filepath.Join(repoRoot(t), "examples")

	var files []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && filepath.Ext(path) == ".tf" {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking examples: %v", err)
	}

	if len(files) == 0 {
		t.Fatal("no example .tf files were found")
	}

	for _, path := range files {
		t.Run(filepath.Base(filepath.Dir(path))+"/"+filepath.Base(path), func(t *testing.T) {
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("reading %s: %v", path, err)
			}

			_, diags := hclparse.NewParser().ParseHCL(raw, path)
			if diags.HasErrors() {
				t.Errorf("%s is not valid HCL: %v", path, diags)
			}
		})
	}
}
