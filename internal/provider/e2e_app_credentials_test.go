package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
)

// TestE2EAppServiceAccountCredentialsLifecycle drives the write-only credential
// through a real terraform binary, which is the only way to prove the two
// properties that matter: that the key never lands in state, and that bumping
// the version rotates it in place rather than replacing the app.
//
// Write-only attributes need Terraform 1.11, so this skips below it rather than
// failing — the provider still builds and works on older versions, it simply
// cannot accept this attribute there.
func TestE2EAppServiceAccountCredentialsLifecycle(t *testing.T) {
	requireTerraform(t)
	env := newE2EEnv(t)

	config := func(version, credential string) string {
		return fmt.Sprintf(`
resource "revenuecat_app" "test" {
  project_id   = %q
  name         = "Acme Android"
  type         = "play_store"
  package_name = "com.acme.app"

  play_service_account_credentials_json_wo         = %q
  play_service_account_credentials_json_wo_version = %q
}
`, env.projectID, credential, version)
	}

	var originalID string

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_11_0),
		},
		Steps: []resource.TestStep{
			{
				Config:           config("1", `{"type":"service_account","private_key":"first"}`),
				ConfigPlanChecks: expectEmptyPlan(),
				Check: resource.ComposeAggregateTestCheckFunc(
					// The credential reached RevenueCat...
					resource.TestCheckResourceAttr("revenuecat_app.test",
						"play_service_account_credentials_configured", "true"),
					// ...and did not stay behind in state.
					resource.TestCheckNoResourceAttr("revenuecat_app.test",
						"play_service_account_credentials_json_wo"),
					resource.TestCheckResourceAttr("revenuecat_app.test",
						"play_service_account_credentials_json_wo_version", "1"),
					captureAttr(t, "revenuecat_app.test", "id", &originalID),
				),
			},
			// Rotation: a new key with a bumped version updates the same app.
			// The configured flag is already true here, so it cannot tell a
			// rotation that happened from one that silently did not — the
			// assertion that matters is on the credential the mock holds.
			{
				Config:           config("2", `{"type":"service_account","private_key":"second"}`),
				ConfigPlanChecks: expectEmptyPlan(),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("revenuecat_app.test",
						"play_service_account_credentials_configured", "true"),
					resource.TestCheckResourceAttr("revenuecat_app.test",
						"play_service_account_credentials_json_wo_version", "2"),
					checkAttrUnchanged(t, "revenuecat_app.test", "id", &originalID),
					checkStoredCredential(t, env, &originalID,
						`{"type":"service_account","private_key":"second"}`),
				),
			},
		},
	})
}

// checkStoredCredential asserts what the service actually ended up holding,
// reaching past the API surface because the credential is write-only and no
// response reveals it.
func checkStoredCredential(t *testing.T, env *e2eEnv, appID *string, want string) resource.TestCheckFunc {
	t.Helper()
	return func(*terraform.State) error {
		if got := env.mock.PlayServiceAccountCredentials(*appID); got != want {
			return fmt.Errorf("the app holds a credential that is not the one configured; "+
				"rotation did not reach the service (got %d bytes, want %d)", len(got), len(want))
		}
		return nil
	}
}

// TestE2EAppWithoutCredentialsReportsUnconfigured pins what MergeTap's own app
// looked like for weeks: managed by Terraform, apparently healthy, and unable
// to verify a single purchase. The configured flag is what makes that visible.
func TestE2EAppWithoutCredentialsReportsUnconfigured(t *testing.T) {
	requireTerraform(t)
	env := newE2EEnv(t)

	env.steps(t,
		resource.TestStep{
			Config: fmt.Sprintf(`
resource "revenuecat_app" "test" {
  project_id   = %q
  name         = "Acme Android"
  type         = "play_store"
  package_name = "com.acme.app"
}
`, env.projectID),
			ConfigPlanChecks: expectEmptyPlan(),
			Check: resource.TestCheckResourceAttr("revenuecat_app.test",
				"play_service_account_credentials_configured", "false"),
		},
	)
}

// TestE2EAppCredentialWithoutVersionIsRejected proves the config validator
// fires where it matters. Accepting this configuration would be worse than
// rejecting it: the key would appear to be managed while a rotation silently
// never reached RevenueCat.
func TestE2EAppCredentialWithoutVersionIsRejected(t *testing.T) {
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
resource "revenuecat_app" "test" {
  project_id   = %q
  name         = "Acme Android"
  type         = "play_store"
  package_name = "com.acme.app"

  play_service_account_credentials_json_wo = "{}"
}
`, env.projectID),
				ExpectError: regexp.MustCompile(`play_service_account_credentials_json_wo_version`),
			},
		},
	})
}
