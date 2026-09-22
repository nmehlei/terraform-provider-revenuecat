package mockrevenuecat

import (
	"net/http"
	"testing"
)

// playStoreFrom pulls the nested play_store object out of a rendered app.
func playStoreFrom(t *testing.T, body map[string]any) map[string]any {
	t.Helper()
	nested, ok := body["play_store"].(map[string]any)
	if !ok {
		t.Fatalf("rendered app has no play_store object: %v", body)
	}
	return nested
}

// TestAppRendersCredentialsConfiguredWithoutEchoingTheCredential is the
// behaviour the provider's write-only design depends on the API having: the
// credential goes in and never comes back, and a boolean reports that it is
// held. A mock that echoed the secret would let a provider bug that stores it
// pass the end-to-end tests unnoticed.
func TestAppRendersCredentialsConfiguredWithoutEchoingTheCredential(t *testing.T) {
	h := newHarness(t, Options{})
	projectID := h.server.CreateProject("Acme")

	status, created := h.do(http.MethodPost, "/v2/projects/"+projectID+"/apps", map[string]any{
		"name": "Acme Android",
		"type": "play_store",
		"play_store": map[string]any{
			"package_name":                          "com.acme.app",
			"play_service_account_credentials_json": `{"type":"service_account"}`,
		},
	})
	if status != http.StatusCreated && status != http.StatusOK {
		t.Fatalf("create returned %d: %v", status, created)
	}

	playStore := playStoreFrom(t, created)
	if _, echoed := playStore["play_service_account_credentials_json"]; echoed {
		t.Error("the mock echoed the credential back; the real API never returns it")
	}
	if playStore["play_service_account_credentials_configured"] != true {
		t.Errorf("play_service_account_credentials_configured = %v, want true",
			playStore["play_service_account_credentials_configured"])
	}
	if playStore["package_name"] != "com.acme.app" {
		t.Errorf("package_name = %v, want com.acme.app", playStore["package_name"])
	}
}

// TestAppWithoutCredentialsReportsNotConfigured covers the state MergeTap was
// actually in: an app created with no credential must say so, because that
// boolean is the only thing that can turn a silently unverifiable app into
// visible drift.
func TestAppWithoutCredentialsReportsNotConfigured(t *testing.T) {
	h := newHarness(t, Options{})
	projectID := h.server.CreateProject("Acme")

	_, created := h.do(http.MethodPost, "/v2/projects/"+projectID+"/apps", map[string]any{
		"name":       "Acme Android",
		"type":       "play_store",
		"play_store": map[string]any{"package_name": "com.acme.app"},
	})

	if playStoreFrom(t, created)["play_service_account_credentials_configured"] != false {
		t.Errorf("an app created without a credential reports configured = %v, want false",
			playStoreFrom(t, created)["play_service_account_credentials_configured"])
	}
}

// TestAppRenameKeepsCredentialsConfigured guards the rotation path's
// neighbour: updating an unrelated field must not drop the credential, or
// every rename would silently break purchase verification.
func TestAppRenameKeepsCredentialsConfigured(t *testing.T) {
	h := newHarness(t, Options{})
	projectID := h.server.CreateProject("Acme")

	_, created := h.do(http.MethodPost, "/v2/projects/"+projectID+"/apps", map[string]any{
		"name": "Acme Android",
		"type": "play_store",
		"play_store": map[string]any{
			"package_name":                          "com.acme.app",
			"play_service_account_credentials_json": `{"type":"service_account"}`,
		},
	})
	appID, _ := created["id"].(string)

	_, updated := h.do(http.MethodPost, "/v2/projects/"+projectID+"/apps/"+appID, map[string]any{
		"name": "Acme Android Renamed",
	})

	if updated["name"] != "Acme Android Renamed" {
		t.Errorf("name = %v, want the renamed value", updated["name"])
	}
	if playStoreFrom(t, updated)["play_service_account_credentials_configured"] != true {
		t.Error("a rename cleared the stored credential")
	}
}

// TestAppUpdateAcceptsARotatedCredential is the rotation path itself: a new key
// must be accepted through update, without recreating the app.
func TestAppUpdateAcceptsARotatedCredential(t *testing.T) {
	h := newHarness(t, Options{})
	projectID := h.server.CreateProject("Acme")

	_, created := h.do(http.MethodPost, "/v2/projects/"+projectID+"/apps", map[string]any{
		"name":       "Acme Android",
		"type":       "play_store",
		"play_store": map[string]any{"package_name": "com.acme.app"},
	})
	appID, _ := created["id"].(string)

	_, updated := h.do(http.MethodPost, "/v2/projects/"+projectID+"/apps/"+appID, map[string]any{
		"play_store": map[string]any{"play_service_account_credentials_json": `{"type":"rotated"}`},
	})

	playStore := playStoreFrom(t, updated)
	if playStore["play_service_account_credentials_configured"] != true {
		t.Error("a rotated credential did not register as configured")
	}
	if _, echoed := playStore["play_service_account_credentials_json"]; echoed {
		t.Error("the mock echoed the rotated credential back")
	}
	if playStore["package_name"] != "com.acme.app" {
		t.Errorf("package_name = %v; a credential rotation must not disturb it", playStore["package_name"])
	}
}
