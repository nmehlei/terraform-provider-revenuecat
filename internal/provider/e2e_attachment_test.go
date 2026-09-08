package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// catalogFixture is the shared prelude for attachment tests: an app and three
// products for an attachment to point at.
func catalogFixture(projectID string) string {
	return fmt.Sprintf(`
resource "revenuecat_app" "ios" {
  project_id   = %[1]q
  name         = "Acme Android"
  type         = "play_store"
  package_name = "com.acme.app"
}

resource "revenuecat_product" "a" {
  project_id       = %[1]q
  app_id           = revenuecat_app.ios.id
  store_identifier = "com.acme.a"
  type             = "subscription"
}

resource "revenuecat_product" "b" {
  project_id       = %[1]q
  app_id           = revenuecat_app.ios.id
  store_identifier = "com.acme.b"
  type             = "subscription"
}

resource "revenuecat_product" "c" {
  project_id       = %[1]q
  app_id           = revenuecat_app.ios.id
  store_identifier = "com.acme.c"
  type             = "subscription"
}

resource "revenuecat_entitlement" "pro" {
  project_id   = %[1]q
  lookup_key   = "pro"
  display_name = "Pro"
}
`, projectID)
}

func TestE2EEntitlementAttachmentLifecycle(t *testing.T) {
	requireTerraform(t)
	env := newE2EEnv(t)

	config := func(products string) string {
		return catalogFixture(env.projectID) + fmt.Sprintf(`
resource "revenuecat_entitlement_product_attachment" "pro" {
  project_id     = %q
  entitlement_id = revenuecat_entitlement.pro.id
  product_ids    = %s
}
`, env.projectID, products)
	}

	const attachmentName = "revenuecat_entitlement_product_attachment.pro"

	env.steps(t,
		resource.TestStep{
			Config:           config("[revenuecat_product.a.id, revenuecat_product.b.id]"),
			ConfigPlanChecks: expectEmptyPlan(),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr(attachmentName, "product_ids.#", "2"),
				checkAttachedCount(t, env, "entitlement", "revenuecat_entitlement.pro", 2),
			),
		},
		// Swap one product for another. The provider must send only the
		// difference, and the result must settle with no pending change.
		resource.TestStep{
			Config:           config("[revenuecat_product.b.id, revenuecat_product.c.id]"),
			ConfigPlanChecks: expectEmptyPlan(),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr(attachmentName, "product_ids.#", "2"),
				checkAttachedCount(t, env, "entitlement", "revenuecat_entitlement.pro", 2),
				checkAttachedMatchesState(t, env, "entitlement", "revenuecat_entitlement.pro", attachmentName),
			),
		},
		resource.TestStep{
			ResourceName:      attachmentName,
			ImportState:       true,
			ImportStateVerify: true,
			ImportStateIdFunc: importIDFunc(attachmentName, "project_id", "entitlement_id"),
		},
		// Removing only the attachment must leave the products and the
		// entitlement in place: destroying a relationship is not destroying
		// what it relates.
		resource.TestStep{
			Config:           catalogFixture(env.projectID),
			ConfigPlanChecks: expectEmptyPlan(),
			Check: resource.ComposeAggregateTestCheckFunc(
				checkAttachedCount(t, env, "entitlement", "revenuecat_entitlement.pro", 0),
				checkMockCount(t, env, "product", 3),
				checkMockCount(t, env, "entitlement", 1),
			),
		},
	)

	env.checkNoObjectsRemain(t, "entitlement", "product", "app")
}

func TestE2EPackageAttachmentLifecycle(t *testing.T) {
	requireTerraform(t)
	env := newE2EEnv(t)

	prelude := catalogFixture(env.projectID) + fmt.Sprintf(`
resource "revenuecat_offering" "default" {
  project_id = %[1]q
  lookup_key = "default"
}

resource "revenuecat_package" "monthly" {
  project_id   = %[1]q
  offering_id  = revenuecat_offering.default.id
  lookup_key   = "$rc_monthly"
  display_name = "Monthly"
  position     = 1
}
`, env.projectID)

	config := func(blocks string) string {
		return prelude + fmt.Sprintf(`
resource "revenuecat_package_product_attachment" "monthly" {
  project_id = %q
  package_id = revenuecat_package.monthly.id
%s
}
`, env.projectID, blocks)
	}

	const attachmentName = "revenuecat_package_product_attachment.monthly"

	env.steps(t,
		resource.TestStep{
			Config: config(`
  product {
    product_id           = revenuecat_product.a.id
    eligibility_criteria = "all"
  }
`),
			ConfigPlanChecks: expectEmptyPlan(),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr(attachmentName, "product.#", "1"),
				checkAttachedCount(t, env, "package", "revenuecat_package.monthly", 1),
			),
		},
		// Changing only the eligibility criteria must converge: the provider
		// re-attaches rather than detaching and re-adding.
		resource.TestStep{
			Config: config(`
  product {
    product_id           = revenuecat_product.a.id
    eligibility_criteria = "google_sdk_lt_6"
  }
`),
			ConfigPlanChecks: expectEmptyPlan(),
			Check: resource.ComposeAggregateTestCheckFunc(
				checkAttachedCount(t, env, "package", "revenuecat_package.monthly", 1),
			),
		},
		// Adding a second product.
		resource.TestStep{
			Config: config(`
  product {
    product_id           = revenuecat_product.a.id
    eligibility_criteria = "google_sdk_lt_6"
  }

  product {
    product_id = revenuecat_product.b.id
  }
`),
			ConfigPlanChecks: expectEmptyPlan(),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr(attachmentName, "product.#", "2"),
				checkAttachedCount(t, env, "package", "revenuecat_package.monthly", 2),
			),
		},
		resource.TestStep{
			Config:           prelude,
			ConfigPlanChecks: expectEmptyPlan(),
			Check: resource.ComposeAggregateTestCheckFunc(
				checkAttachedCount(t, env, "package", "revenuecat_package.monthly", 0),
				checkMockCount(t, env, "product", 3),
				checkMockCount(t, env, "package", 1),
			),
		},
	)

	env.checkNoObjectsRemain(t, "package", "offering", "product", "app")
}

// checkAttachedCount asserts how many products the mock has attached to the
// parent, which verifies the relationship on the service rather than in state.
func checkAttachedCount(t *testing.T, env *e2eEnv, kind, parentResource string, want int) resource.TestCheckFunc {
	t.Helper()
	return func(state *terraform.State) error {
		parentID, err := attrValue(state, parentResource, "id")
		if err != nil {
			return err
		}

		attached := env.mock.AttachedProducts(kind, parentID)
		if len(attached) != want {
			return fmt.Errorf("the mock has %d products attached to %s %s (%v), want %d",
				len(attached), kind, parentID, attached, want)
		}
		return nil
	}
}

// checkAttachedMatchesState asserts the products the mock holds are exactly the
// ones Terraform believes are attached.
func checkAttachedMatchesState(t *testing.T, env *e2eEnv, kind, parentResource, attachmentResource string) resource.TestCheckFunc {
	t.Helper()
	return func(state *terraform.State) error {
		parentID, err := attrValue(state, parentResource, "id")
		if err != nil {
			return err
		}

		res, ok := state.RootModule().Resources[attachmentResource]
		if !ok {
			return fmt.Errorf("resource %s is not in state", attachmentResource)
		}

		inState := map[string]bool{}
		for name, value := range res.Primary.Attributes {
			if len(name) > len("product_ids.") && name[:len("product_ids.")] == "product_ids." && name != "product_ids.#" {
				inState[value] = true
			}
		}

		onServer := map[string]bool{}
		for _, id := range env.mock.AttachedProducts(kind, parentID) {
			onServer[id] = true
		}

		for id := range inState {
			if !onServer[id] {
				return fmt.Errorf("state says product %s is attached but the mock does not have it", id)
			}
		}
		for id := range onServer {
			if !inState[id] {
				return fmt.Errorf("the mock has product %s attached but state does not list it", id)
			}
		}
		return nil
	}
}

// checkMockCount asserts how many objects of a kind the mock holds.
func checkMockCount(t *testing.T, env *e2eEnv, kind string, want int) resource.TestCheckFunc {
	t.Helper()
	return func(*terraform.State) error {
		if got := env.mock.Count(kind); got != want {
			return fmt.Errorf("the mock holds %d %s objects, want %d", got, kind, want)
		}
		return nil
	}
}
