package provider

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	resource2 "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestE2ECompleteExampleApplies applies the repository's own examples/complete
// configuration against the mock.
//
// Testing a configuration written for the test would verify the test. Testing
// the shipped example verifies the documentation, which is the thing that
// actually goes stale — a renamed attribute or a removed resource breaks this
// before a practitioner copies it and finds out.
func TestE2ECompleteExampleApplies(t *testing.T) {
	requireTerraform(t)
	env := newE2EEnv(t)

	// The example looks its project up by name, so the mock is seeded with a
	// project of that name rather than the example being rewritten.
	config := loadExample(t, "complete")

	env.steps(t,
		resource.TestStep{
			Config:           config,
			ConfigPlanChecks: expectEmptyPlan(),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttrSet("revenuecat_entitlement.pro", "id"),
				resource.TestCheckResourceAttr("revenuecat_offering.default", "is_current", "true"),
				resource.TestCheckResourceAttr("revenuecat_entitlement_product_attachment.pro", "product_ids.#", "3"),
				checkMockCount(t, env, "app", 2),
				checkMockCount(t, env, "product", 3),
				checkMockCount(t, env, "package", 2),
			),
		},
	)

	// Destroy runs at the end of the case; nothing the example created may
	// survive it.
	env.checkNoObjectsRemain(t, "app", "product", "entitlement", "offering", "package")
}

// loadExample reads an example directory's .tf files and adapts them for the
// test harness: the terraform and provider blocks are dropped, because the
// harness supplies the provider through reattach and points it at the mock
// through the environment. Everything else — the resources, data sources and
// their wiring, which is what the example actually documents — is used as is.
func loadExample(t *testing.T, name string) string {
	t.Helper()

	dir := filepath.Join(repoRoot(t), "examples", name)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}

	var builder strings.Builder
	var files int

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".tf" {
			continue
		}

		raw, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			t.Fatalf("reading %s: %v", entry.Name(), err)
		}
		builder.WriteString(stripProviderBlocks(string(raw)))
		builder.WriteString("\n")
		files++
	}

	if files == 0 {
		t.Fatalf("no .tf files found in %s", dir)
	}
	return builder.String()
}

// blockPattern matches a top-level terraform or provider block, including
// nested braces, so the harness can drop them without disturbing the rest.
var blockPattern = regexp.MustCompile(`(?ms)^(terraform|provider)\s[^\n]*\{.*?^\}\n?`)

func stripProviderBlocks(config string) string {
	return blockPattern.ReplaceAllString(config, "")
}

// TestStripProviderBlocks guards the adaptation above: it must remove the
// provider plumbing and nothing else, or the example test would be silently
// testing a mutilated configuration.
func TestStripProviderBlocks(t *testing.T) {
	input := `terraform {
  required_providers {
    revenuecat = {
      source = "nmehlei/revenuecat"
    }
  }
}

provider "revenuecat" {
  # a comment
}

resource "revenuecat_entitlement" "pro" {
  project_id = "p"
  lookup_key = "pro"
}

variable "project_name" {
  type = string
}
`

	got := stripProviderBlocks(input)

	for _, unwanted := range []string{"required_providers", `provider "revenuecat"`} {
		if strings.Contains(got, unwanted) {
			t.Errorf("output still contains %q:\n%s", unwanted, got)
		}
	}
	for _, wanted := range []string{`resource "revenuecat_entitlement" "pro"`, `variable "project_name"`, `lookup_key = "pro"`} {
		if !strings.Contains(got, wanted) {
			t.Errorf("output lost %q:\n%s", wanted, got)
		}
	}
}

// TestExampleReferencesOnlyRealTypes checks every resource and data source the
// examples mention is actually registered, catching a typo or a renamed type
// even when the Terraform-driven tests are not enabled.
func TestExampleReferencesOnlyRealTypes(t *testing.T) {
	registered := registeredTypeNames(t)

	declaration := regexp.MustCompile(`(?m)^\s*(resource|data)\s+"(revenuecat_[a-z_]+)"`)

	root := filepath.Join(repoRoot(t), "examples")
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || filepath.Ext(path) != ".tf" {
			return err
		}

		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		for _, match := range declaration.FindAllStringSubmatch(string(raw), -1) {
			kind, typeName := match[1], match[2]
			key := kind + ":" + typeName
			if !registered[key] {
				t.Errorf("%s declares %s %q, which the provider does not register",
					filepath.Base(path), kind, typeName)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking examples: %v", err)
	}
}

// registeredTypeNames returns the set of "resource:<name>" and "data:<name>"
// keys the provider registers.
func registeredTypeNames(t *testing.T) map[string]bool {
	t.Helper()

	ctx := context.Background()
	p := New("test")()
	names := map[string]bool{}

	for _, newResource := range p.Resources(ctx) {
		var metadata resource2.MetadataResponse
		newResource().Metadata(ctx, resource2.MetadataRequest{ProviderTypeName: "revenuecat"}, &metadata)
		names["resource:"+metadata.TypeName] = true
	}
	for _, newDataSource := range p.DataSources(ctx) {
		var metadata datasource.MetadataResponse
		newDataSource().Metadata(ctx, datasource.MetadataRequest{ProviderTypeName: "revenuecat"}, &metadata)
		names["data:"+metadata.TypeName] = true
	}
	return names
}
