package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
)

// appSchema builds the revenuecat_app schema for assertions about its shape.
func appSchema(t *testing.T) resource.SchemaResponse {
	t.Helper()

	var resp resource.SchemaResponse
	NewAppResource().Schema(context.Background(), resource.SchemaRequest{}, &resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("app schema has errors: %v", resp.Diagnostics)
	}
	if diags := resp.Schema.ValidateImplementation(context.Background()); diags.HasError() {
		t.Fatalf("app schema failed validation: %v", diags)
	}
	return resp
}

// TestAppCredentialAttributeIsWriteOnly is the security property this whole
// attribute exists for: the Play service account key must never be written to
// state or to a plan file. Sensitive alone only hides it from console output —
// it would still sit in the state backend in plaintext.
func TestAppCredentialAttributeIsWriteOnly(t *testing.T) {
	attr, ok := appSchema(t).Schema.Attributes["play_service_account_credentials_json_wo"]
	if !ok {
		t.Fatal("revenuecat_app has no play_service_account_credentials_json_wo attribute")
	}

	if !attr.IsWriteOnly() {
		t.Error("play_service_account_credentials_json_wo must be write-only so the key never reaches state")
	}
	if !attr.IsSensitive() {
		t.Error("play_service_account_credentials_json_wo must be sensitive so the key never reaches console output")
	}
	if !attr.IsOptional() {
		t.Error("play_service_account_credentials_json_wo must be optional; apps can be managed without it")
	}
	if attr.IsComputed() {
		t.Error("play_service_account_credentials_json_wo must not be computed; the API never returns it")
	}
}

// TestAppCredentialVersionAttributeIsStored covers the other half of the
// write-only pattern. Terraform cannot diff a value it does not store, so a
// rotated key alone produces no plan; the stored version is what makes one.
func TestAppCredentialVersionAttributeIsStored(t *testing.T) {
	attr, ok := appSchema(t).Schema.Attributes["play_service_account_credentials_json_wo_version"]
	if !ok {
		t.Fatal("revenuecat_app has no play_service_account_credentials_json_wo_version attribute")
	}

	if attr.IsWriteOnly() {
		t.Error("the version must be stored in state; a write-only version could never trigger a rotation")
	}
	if !attr.IsOptional() {
		t.Error("the version must be optional; it is only meaningful alongside a credential")
	}
}

// TestAppReportsWhetherCredentialsAreConfigured is the drift signal. Without
// it, an app missing its credential looks identical in state to one that has
// it, and the failure only appears when Google refunds an unacknowledged
// purchase days later.
func TestAppReportsWhetherCredentialsAreConfigured(t *testing.T) {
	attr, ok := appSchema(t).Schema.Attributes["play_service_account_credentials_configured"]
	if !ok {
		t.Fatal("revenuecat_app has no play_service_account_credentials_configured attribute")
	}

	if !attr.IsComputed() {
		t.Error("play_service_account_credentials_configured must be computed; the API owns it")
	}
	if attr.IsOptional() || attr.IsRequired() {
		t.Error("play_service_account_credentials_configured must not be settable; it is a report, not an input")
	}
}

// TestAppCredentialRequiresAVersion stops the silent-no-op configuration: a
// credential with no version can be changed forever without Terraform ever
// noticing, so the provider must reject that pairing outright rather than
// appearing to apply it.
func TestAppCredentialRequiresAVersion(t *testing.T) {
	withValidators, ok := NewAppResource().(resource.ResourceWithConfigValidators)
	if !ok {
		t.Fatal("revenuecat_app implements no config validators; a credential without a version must be rejected")
	}

	if len(withValidators.ConfigValidators(context.Background())) == 0 {
		t.Fatal("revenuecat_app declares no config validators; a credential without a version must be rejected")
	}
}
