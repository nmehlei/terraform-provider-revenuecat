package mockrevenuecat

import (
	"net/http"
	"strings"
	"testing"
)

// TestWebhookIntegrationKeepsTheHeaderWithoutEchoingIt mirrors the real API:
// the authorization header goes in and never comes back. A mock that echoed it
// would let a provider that stores the header in state pass unnoticed.
func TestWebhookIntegrationKeepsTheHeaderWithoutEchoingIt(t *testing.T) {
	h := newHarness(t, Options{})
	projectID := h.server.CreateProject("Acme")
	base := "/v2/projects/" + projectID + "/integrations/webhooks"

	status, created := h.do(http.MethodPost, base, map[string]any{
		"name":                 "Hook",
		"url":                  "https://example.com/hook",
		"authorization_header": "Bearer first",
		"environment":          "production",
	})
	if status != http.StatusCreated {
		t.Fatalf("create returned %d: %v", status, created)
	}
	if _, echoed := created["authorization_header"]; echoed {
		t.Error("the mock echoed the authorization header; the real API never returns it")
	}
	id, _ := created["id"].(string)
	if got := h.server.WebhookAuthorizationHeader(id); got != "Bearer first" {
		t.Errorf("stored header = %q, want the one sent", got)
	}

	// An update without the header keeps it; explicit nulls clear scope fields.
	status, updated := h.do(http.MethodPost, base+"/"+id, map[string]any{
		"name":        "Hook renamed",
		"url":         "https://example.com/hook",
		"environment": nil,
		"event_types": nil,
		"app_id":      nil,
	})
	if status != http.StatusOK {
		t.Fatalf("update returned %d: %v", status, updated)
	}
	if updated["environment"] != nil {
		t.Errorf("environment = %v, want it cleared by the explicit null", updated["environment"])
	}
	if got := h.server.WebhookAuthorizationHeader(id); got != "Bearer first" {
		t.Errorf("an update without the header changed it to %q", got)
	}

	// A null header clears it.
	h.do(http.MethodPost, base+"/"+id, map[string]any{"name": "Hook", "url": "https://example.com/hook", "authorization_header": nil})
	if got := h.server.WebhookAuthorizationHeader(id); got != "" {
		t.Errorf("a null header left %q in place", got)
	}
}

// TestWebhookIntegrationRequiresNameAndURL matches the real API's 400s, so a
// provider that forgets either fails here rather than only in production.
func TestWebhookIntegrationRequiresNameAndURL(t *testing.T) {
	h := newHarness(t, Options{})
	projectID := h.server.CreateProject("Acme")

	status, _ := h.do(http.MethodPost, "/v2/projects/"+projectID+"/integrations/webhooks", map[string]any{"name": "Hook"})
	if status != http.StatusBadRequest {
		t.Errorf("create without url returned %d, want 400", status)
	}
}

// TestAppPublicAPIKeys serves a production SDK key for every app, and 404s
// once the app is gone, like the real endpoint.
func TestAppPublicAPIKeys(t *testing.T) {
	h := newHarness(t, Options{})
	projectID := h.server.CreateProject("Acme")

	_, app := h.do(http.MethodPost, "/v2/projects/"+projectID+"/apps", map[string]any{
		"name": "Acme Android", "type": "play_store", "play_store": map[string]any{"package_name": "com.acme.app"},
	})
	appID, _ := app["id"].(string)
	path := "/v2/projects/" + projectID + "/apps/" + appID + "/public_api_keys"

	status, list := h.do(http.MethodGet, path, nil)
	if status != http.StatusOK {
		t.Fatalf("list returned %d: %v", status, list)
	}
	items, _ := list["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("items = %v, want one production key", list["items"])
	}
	key, _ := items[0].(map[string]any)
	if key["environment"] != "production" || !strings.HasPrefix(key["key"].(string), "goog_") || key["app_id"] != appID {
		t.Errorf("key = %v", key)
	}

	h.do(http.MethodDelete, "/v2/projects/"+projectID+"/apps/"+appID, nil)
	if status, _ := h.do(http.MethodGet, path, nil); status != http.StatusNotFound {
		t.Errorf("keys of a deleted app returned %d, want 404", status)
	}
}
