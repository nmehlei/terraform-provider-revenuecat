package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
)

// TestE2EWebhookIntegrationLifecycle drives the webhook through a real
// terraform binary: the header must reach RevenueCat without landing in state,
// a rename must keep both the webhook and its header, clearing the environment
// must reach the service, and bumping the version must rotate the header in
// place.
func TestE2EWebhookIntegrationLifecycle(t *testing.T) {
	requireTerraform(t)
	env := newE2EEnv(t)

	config := func(name, environment, header, version string) string {
		envLine := ""
		if environment != "" {
			envLine = fmt.Sprintf("environment = %q", environment)
		}
		return fmt.Sprintf(`
resource "revenuecat_webhook_integration" "test" {
  project_id  = %q
  name        = %q
  url         = "https://example.com/hook"
  event_types = ["initial_purchase", "renewal"]
  %s

  authorization_header_wo         = %q
  authorization_header_wo_version = %q
}
`, env.projectID, name, envLine, header, version)
	}

	var originalID string

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_11_0),
		},
		Steps: []resource.TestStep{
			{
				Config:           config("Hook", "production", "Bearer first", "1"),
				ConfigPlanChecks: expectEmptyPlan(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("revenuecat_webhook_integration.test", "id"),
					resource.TestCheckResourceAttrSet("revenuecat_webhook_integration.test", "created_at"),
					resource.TestCheckResourceAttr("revenuecat_webhook_integration.test", "environment", "production"),
					resource.TestCheckResourceAttr("revenuecat_webhook_integration.test", "event_types.#", "2"),
					resource.TestCheckNoResourceAttr("revenuecat_webhook_integration.test", "authorization_header_wo"),
					captureAttr(t, "revenuecat_webhook_integration.test", "id", &originalID),
					checkStoredWebhookHeader(env, &originalID, "Bearer first"),
				),
			},
			// Rename and widen to both environments; the header stays as it was.
			{
				Config:           config("Hook renamed", "", "Bearer first", "1"),
				ConfigPlanChecks: expectEmptyPlan(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("revenuecat_webhook_integration.test", "name", "Hook renamed"),
					resource.TestCheckNoResourceAttr("revenuecat_webhook_integration.test", "environment"),
					checkAttrUnchanged(t, "revenuecat_webhook_integration.test", "id", &originalID),
					checkStoredWebhookHeader(env, &originalID, "Bearer first"),
				),
			},
			// Rotation: a new header with a bumped version updates in place.
			{
				Config:           config("Hook renamed", "", "Bearer second", "2"),
				ConfigPlanChecks: expectEmptyPlan(),
				Check: resource.ComposeAggregateTestCheckFunc(
					checkAttrUnchanged(t, "revenuecat_webhook_integration.test", "id", &originalID),
					checkStoredWebhookHeader(env, &originalID, "Bearer second"),
				),
			},
			{
				ResourceName:      "revenuecat_webhook_integration.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: importIDFunc("revenuecat_webhook_integration.test", "project_id", "id"),
				// The version is the practitioner's marker; RevenueCat never stores it.
				ImportStateVerifyIgnore: []string{"authorization_header_wo_version"},
			},
		},
	})

	env.checkNoObjectsRemain(t, "webhook_integration")
}

// TestE2EWebhookIntegrationWithoutHeader runs on every Terraform version: a
// webhook without a header needs no write-only support.
func TestE2EWebhookIntegrationWithoutHeader(t *testing.T) {
	requireTerraform(t)
	env := newE2EEnv(t)

	env.steps(t,
		resource.TestStep{
			Config: fmt.Sprintf(`
resource "revenuecat_webhook_integration" "test" {
  project_id = %q
  name       = "Hook"
  url        = "https://example.com/hook"
}
`, env.projectID),
			ConfigPlanChecks: expectEmptyPlan(),
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckNoResourceAttr("revenuecat_webhook_integration.test", "environment"),
				resource.TestCheckNoResourceAttr("revenuecat_webhook_integration.test", "event_types"),
				resource.TestCheckNoResourceAttr("revenuecat_webhook_integration.test", "app_id"),
			),
		},
	)

	env.checkNoObjectsRemain(t, "webhook_integration")
}

// TestE2EWebhookHeaderWithoutVersionIsRejected: without a version a rotated
// header would silently never reach RevenueCat.
func TestE2EWebhookHeaderWithoutVersionIsRejected(t *testing.T) {
	requireTerraform(t)
	env := newE2EEnv(t)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_11_0),
		},
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "revenuecat_webhook_integration" "test" {
  project_id              = %q
  name                    = "Hook"
  url                     = "https://example.com/hook"
  authorization_header_wo = "Bearer x"
}
`, env.projectID),
				ExpectError: regexp.MustCompile(`authorization_header_wo_version`),
			},
		},
	})
}

// TestE2EAppPublicAPIKeys reads an app's SDK key through Terraform, so it can
// flow into configuration instead of being copied from the dashboard.
func TestE2EAppPublicAPIKeys(t *testing.T) {
	requireTerraform(t)
	env := newE2EEnv(t)

	var appID string
	env.steps(t,
		resource.TestStep{
			Config: fmt.Sprintf(`
resource "revenuecat_app" "test" {
  project_id   = %q
  name         = "Acme Android"
  type         = "play_store"
  package_name = "com.acme.app"
}

data "revenuecat_app_public_api_keys" "test" {
  project_id = revenuecat_app.test.project_id
  app_id     = revenuecat_app.test.id
}
`, env.projectID),
			Check: resource.ComposeAggregateTestCheckFunc(
				captureAttr(t, "revenuecat_app.test", "id", &appID),
				resource.TestCheckResourceAttr("data.revenuecat_app_public_api_keys.test", "keys.#", "1"),
				resource.TestCheckResourceAttr("data.revenuecat_app_public_api_keys.test", "keys.0.environment", "production"),
				func(s *terraform.State) error {
					return resource.TestCheckResourceAttr("data.revenuecat_app_public_api_keys.test",
						"production_key", "goog_"+appID)(s)
				},
			),
		},
	)
}

func checkStoredWebhookHeader(env *e2eEnv, webhookID *string, want string) resource.TestCheckFunc {
	return func(*terraform.State) error {
		if got := env.mock.WebhookAuthorizationHeader(*webhookID); got != want {
			return fmt.Errorf("the webhook holds a header that is not the one configured "+
				"(got %d bytes, want %d)", len(got), len(want))
		}
		return nil
	}
}
