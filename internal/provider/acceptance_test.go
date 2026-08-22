package provider

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// Acceptance tests drive a real terraform binary against a real RevenueCat
// project, creating and destroying catalog objects. They run only when TF_ACC
// is set, so `go test ./...` stays hermetic.
//
// To run them:
//
//	export TF_ACC=1
//	export REVENUECAT_API_KEY=sk_...
//	export REVENUECAT_PROJECT_ID=proj...
//	make testacc
//
// Point them at a scratch project, never a production one.

const (
	envProjectID = "REVENUECAT_PROJECT_ID"
)

var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"revenuecat": providerserver.NewProtocol6WithError(New("test")()),
}

// requireRealService skips unless credentials for a real RevenueCat project are
// present. TF_ACC alone is not enough, because the mock-driven end-to-end tests
// in the e2e_* files also run under TF_ACC and need no credentials — failing
// here would make those unrunnable.
func requireRealService(t *testing.T) {
	t.Helper()

	if os.Getenv("TF_ACC") == "" {
		t.Skip("set TF_ACC=1 to run acceptance tests")
	}

	for _, name := range []string{envAPIKey, envProjectID} {
		if os.Getenv(name) == "" {
			t.Skipf("%s is not set; skipping the tests that need a real RevenueCat project. "+
				"Run `make testacc-mock` for the credential-free end-to-end tests.", name)
		}
	}
}

// testAccPreCheck runs immediately before the first step, once the test has
// already decided to run.
func testAccPreCheck(t *testing.T) {
	t.Helper()

	for _, name := range []string{envAPIKey, envProjectID} {
		if os.Getenv(name) == "" {
			t.Fatalf("%s must be set to run acceptance tests", name)
		}
	}
}

func testAccProjectID(t *testing.T) string {
	t.Helper()
	return os.Getenv(envProjectID)
}

func TestAccEntitlement_lifecycle(t *testing.T) {
	requireRealService(t)

	projectID := testAccProjectID(t)
	lookupKey := "tfacc-entitlement"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccEntitlementConfig(projectID, lookupKey, "Acceptance Pro"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("revenuecat_entitlement.test", "lookup_key", lookupKey),
					resource.TestCheckResourceAttr("revenuecat_entitlement.test", "display_name", "Acceptance Pro"),
					resource.TestCheckResourceAttrSet("revenuecat_entitlement.test", "id"),
					resource.TestCheckResourceAttrSet("revenuecat_entitlement.test", "created_at"),
				),
			},
			{
				// Update in place: the display name changes without replacement.
				Config: testAccEntitlementConfig(projectID, lookupKey, "Acceptance Pro Plus"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("revenuecat_entitlement.test", "display_name", "Acceptance Pro Plus"),
				),
			},
			{
				ResourceName:      "revenuecat_entitlement.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: importIDFunc("revenuecat_entitlement.test", "project_id", "id"),
			},
		},
	})
}

func TestAccOffering_lifecycle(t *testing.T) {
	requireRealService(t)

	projectID := testAccProjectID(t)
	lookupKey := "tfacc-offering"

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccOfferingConfig(projectID, lookupKey, "Acceptance Offering", "a"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("revenuecat_offering.test", "lookup_key", lookupKey),
					resource.TestCheckResourceAttr("revenuecat_offering.test", "metadata.paywall_variant", "a"),
					resource.TestCheckResourceAttrSet("revenuecat_offering.test", "id"),
				),
			},
			{
				Config: testAccOfferingConfig(projectID, lookupKey, "Acceptance Offering", "b"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("revenuecat_offering.test", "metadata.paywall_variant", "b"),
				),
			},
		},
	})
}

// importIDFunc builds the colon-delimited import identifier this provider
// expects from the attributes recorded in state.
func importIDFunc(resourceName string, attributes ...string) func(*terraform.State) (string, error) {
	return func(state *terraform.State) (string, error) {
		res, ok := state.RootModule().Resources[resourceName]
		if !ok {
			return "", fmt.Errorf("resource %s is not in state", resourceName)
		}

		segments := make([]string, 0, len(attributes))
		for _, attribute := range attributes {
			value, ok := res.Primary.Attributes[attribute]
			if !ok {
				return "", fmt.Errorf("resource %s has no %s attribute in state", resourceName, attribute)
			}
			segments = append(segments, value)
		}
		return strings.Join(segments, ":"), nil
	}
}

func testAccEntitlementConfig(projectID, lookupKey, displayName string) string {
	return fmt.Sprintf(`
resource "revenuecat_entitlement" "test" {
  project_id   = %[1]q
  lookup_key   = %[2]q
  display_name = %[3]q
}
`, projectID, lookupKey, displayName)
}

func testAccOfferingConfig(projectID, lookupKey, displayName, variant string) string {
	return fmt.Sprintf(`
resource "revenuecat_offering" "test" {
  project_id   = %[1]q
  lookup_key   = %[2]q
  display_name = %[3]q

  metadata = {
    paywall_variant = %[4]q
  }
}
`, projectID, lookupKey, displayName, variant)
}
