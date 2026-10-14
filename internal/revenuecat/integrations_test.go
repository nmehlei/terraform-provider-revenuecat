package revenuecat

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestCreateWebhookIntegrationSendsTheAuthorizationHeader pins the create body:
// the header is the only way RevenueCat proves to the receiver that a webhook
// came from it, so dropping it on the wire leaves an endpoint that rejects
// every event (or, worse, one configured to accept anything).
func TestCreateWebhookIntegrationSendsTheAuthorizationHeader(t *testing.T) {
	var got map[string]any
	var path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		got = decodeBody(t, r)
		writeJSON(t, w, http.StatusCreated, map[string]any{"id": "wh1", "name": "Hook"})
	}))
	defer srv.Close()

	c, _ := newTestClient(t, srv)
	header := "Bearer s3cret"
	_, err := c.CreateWebhookIntegration(context.Background(), "proj1", CreateWebhookIntegrationRequest{
		Name:                "Hook",
		URL:                 "https://example.com/hook",
		AuthorizationHeader: &header,
		EventTypes:          []string{"initial_purchase"},
	})
	if err != nil {
		t.Fatalf("CreateWebhookIntegration: %v", err)
	}

	if path != "/v2/projects/proj1/integrations/webhooks" {
		t.Errorf("path = %q, want the project's webhook collection", path)
	}
	if got["authorization_header"] != header {
		t.Errorf("authorization_header was not sent: %v", got)
	}
	if _, present := got["environment"]; present {
		t.Errorf("an unset environment must be omitted on create, got %v", got["environment"])
	}
}

// TestWebhookIntegrationNeverDecodesASigningSecret keeps secrets out of state
// at the boundary: RevenueCat returns signing_secret on some responses, and
// nothing in this provider may carry it onward.
func TestWebhookIntegrationNeverDecodesASigningSecret(t *testing.T) {
	var hook WebhookIntegration
	raw := `{"id":"wh1","name":"Hook","url":"https://example.com","authorization_header":"Bearer leak","signing_secret":"leak"}`
	if err := json.Unmarshal([]byte(raw), &hook); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	encoded, _ := json.Marshal(hook)
	if strings.Contains(string(encoded), "leak") {
		t.Errorf("a secret survived decoding: %s", encoded)
	}
}

// TestUpdateWebhookIntegrationSendsExplicitNulls pins the update body. The API
// treats an omitted field as "leave unchanged", so clearing an environment or
// an app scope in configuration only reaches RevenueCat as an explicit null.
// The header is the opposite: omitted unless it is being rotated, so a rename
// never pushes the secret over the wire.
func TestUpdateWebhookIntegrationSendsExplicitNulls(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = decodeBody(t, r)
		writeJSON(t, w, http.StatusOK, map[string]any{"id": "wh1"})
	}))
	defer srv.Close()

	c, _ := newTestClient(t, srv)
	_, err := c.UpdateWebhookIntegration(context.Background(), "proj1", "wh1", UpdateWebhookIntegrationRequest{
		Name: "Hook",
		URL:  "https://example.com/hook",
	})
	if err != nil {
		t.Fatalf("UpdateWebhookIntegration: %v", err)
	}

	for _, field := range []string{"environment", "event_types", "app_id"} {
		value, present := got[field]
		if !present || value != nil {
			t.Errorf("%s must be sent as an explicit null to clear it, got present=%v value=%v", field, present, value)
		}
	}
	if _, present := got["authorization_header"]; present {
		t.Errorf("authorization_header must be omitted unless rotated, got %v", got)
	}
}

// TestUpdateWebhookIntegrationCanClearTheHeader covers the rotation edge: a
// rotation to "no header" has to send null, or the old secret stays live.
func TestUpdateWebhookIntegrationCanClearTheHeader(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = decodeBody(t, r)
		writeJSON(t, w, http.StatusOK, map[string]any{"id": "wh1"})
	}))
	defer srv.Close()

	c, _ := newTestClient(t, srv)
	_, err := c.UpdateWebhookIntegration(context.Background(), "proj1", "wh1", UpdateWebhookIntegrationRequest{
		Name:                      "Hook",
		URL:                       "https://example.com/hook",
		RotateAuthorizationHeader: true,
	})
	if err != nil {
		t.Fatalf("UpdateWebhookIntegration: %v", err)
	}

	value, present := got["authorization_header"]
	if !present || value != nil {
		t.Errorf("a rotation to no header must send null, got present=%v value=%v", present, value)
	}
}

// TestListAppPublicAPIKeys reads the SDK keys RevenueCat generates for an app,
// which until now could only be copied out of the dashboard by hand.
func TestListAppPublicAPIKeys(t *testing.T) {
	var path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		writeJSON(t, w, http.StatusOK, map[string]any{
			"object": "list",
			"items": []map[string]any{
				{"object": "public_api_key", "id": "pk1", "key": "goog_abc", "environment": "production", "app_id": "app1", "created_at": 1},
			},
		})
	}))
	defer srv.Close()

	c, _ := newTestClient(t, srv)
	keys, err := c.ListAppPublicAPIKeys(context.Background(), "proj1", "app1")
	if err != nil {
		t.Fatalf("ListAppPublicAPIKeys: %v", err)
	}
	if path != "/v2/projects/proj1/apps/app1/public_api_keys" {
		t.Errorf("path = %q", path)
	}
	if len(keys) != 1 || keys[0].Key != "goog_abc" || keys[0].Environment != "production" {
		t.Errorf("keys = %+v", keys)
	}
}
