package revenuecat

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// decodeBody reads a request body as generic JSON so a test can assert on the
// exact wire shape rather than on the Go struct that produced it.
func decodeBody(t *testing.T, r *http.Request) map[string]any {
	t.Helper()
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatalf("reading the request body: %v", err)
	}
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatalf("decoding the request body %q: %v", raw, err)
	}
	return body
}

// playStoreOf pulls the nested "play_store" object out of a decoded body.
func playStoreOf(t *testing.T, body map[string]any) map[string]any {
	t.Helper()
	nested, ok := body["play_store"].(map[string]any)
	if !ok {
		t.Fatalf("body has no nested play_store object: %v", body)
	}
	return nested
}

// TestCreateAppNestsServiceAccountCredentials pins where the credential goes on
// the wire. RevenueCat nests it inside the type-specific object alongside
// package_name, not at the top level — sending it flat is silently ignored,
// which is exactly the failure that leaves purchases unacknowledged.
func TestCreateAppNestsServiceAccountCredentials(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = decodeBody(t, r)
		writeJSON(t, w, http.StatusOK, App{ID: "app1"})
	}))
	defer srv.Close()

	c, _ := newTestClient(t, srv)
	_, err := c.CreateApp(context.Background(), "proj1", CreateAppRequest{
		Name: "Acme Android",
		Type: AppTypePlayStore,
		PlayStore: &PlayStoreConfig{
			PackageName:                   "com.acme.app",
			ServiceAccountCredentialsJSON: `{"type":"service_account"}`,
		},
	})
	if err != nil {
		t.Fatalf("CreateApp: %v", err)
	}

	playStore := playStoreOf(t, got)
	if want := `{"type":"service_account"}`; playStore["play_service_account_credentials_json"] != want {
		t.Errorf("play_store.play_service_account_credentials_json = %v, want %q",
			playStore["play_service_account_credentials_json"], want)
	}
	if want := "com.acme.app"; playStore["package_name"] != want {
		t.Errorf("play_store.package_name = %v, want %q", playStore["package_name"], want)
	}
}

// TestCreateAppOmitsAbsentServiceAccountCredentials keeps an unset credential
// out of the payload entirely. Sending an empty string would read as "clear the
// credential", which would silently break verification on an app that already
// has one.
func TestCreateAppOmitsAbsentServiceAccountCredentials(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = decodeBody(t, r)
		writeJSON(t, w, http.StatusOK, App{ID: "app1"})
	}))
	defer srv.Close()

	c, _ := newTestClient(t, srv)
	_, err := c.CreateApp(context.Background(), "proj1", CreateAppRequest{
		Name:      "Acme Android",
		Type:      AppTypePlayStore,
		PlayStore: &PlayStoreConfig{PackageName: "com.acme.app"},
	})
	if err != nil {
		t.Fatalf("CreateApp: %v", err)
	}

	if _, present := playStoreOf(t, got)["play_service_account_credentials_json"]; present {
		t.Error("play_service_account_credentials_json was sent although none was configured")
	}
}

// TestUpdateAppCanReplaceServiceAccountCredentials covers key rotation: a new
// credential must reach the API through the update path, without the caller
// having to destroy and recreate the app.
func TestUpdateAppCanReplaceServiceAccountCredentials(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = decodeBody(t, r)
		writeJSON(t, w, http.StatusOK, App{ID: "app1"})
	}))
	defer srv.Close()

	c, _ := newTestClient(t, srv)
	_, err := c.UpdateApp(context.Background(), "proj1", "app1", UpdateAppRequest{
		PlayStore: &PlayStoreConfig{ServiceAccountCredentialsJSON: `{"type":"rotated"}`},
	})
	if err != nil {
		t.Fatalf("UpdateApp: %v", err)
	}

	if want := `{"type":"rotated"}`; playStoreOf(t, got)["play_service_account_credentials_json"] != want {
		t.Errorf("play_store.play_service_account_credentials_json = %v, want %q",
			playStoreOf(t, got)["play_service_account_credentials_json"], want)
	}
}

// TestUpdateAppOmitsPlayStoreWhenOnlyRenaming keeps a plain rename from
// touching the credential. An empty play_store object in the payload could be
// read as clearing it.
func TestUpdateAppOmitsPlayStoreWhenOnlyRenaming(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = decodeBody(t, r)
		writeJSON(t, w, http.StatusOK, App{ID: "app1"})
	}))
	defer srv.Close()

	c, _ := newTestClient(t, srv)
	name := "Renamed"
	if _, err := c.UpdateApp(context.Background(), "proj1", "app1", UpdateAppRequest{Name: &name}); err != nil {
		t.Fatalf("UpdateApp: %v", err)
	}

	if _, present := got["play_store"]; present {
		t.Errorf("a rename sent a play_store object: %v", got)
	}
}

// TestAppReadsCredentialsConfiguredFlag reads back the one signal the API does
// return about the credential. The JSON itself is write-only, so this boolean
// is the only thing that can surface "the credential is missing" as drift
// instead of as a refunded purchase three days later.
func TestAppReadsCredentialsConfiguredFlag(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{
			"id": "app1",
			"name": "Acme Android",
			"type": "play_store",
			"play_store": {
				"package_name": "com.acme.app",
				"play_service_account_credentials_configured": true
			}
		}`)
	}))
	defer srv.Close()

	c, _ := newTestClient(t, srv)
	app, err := c.GetApp(context.Background(), "proj1", "app1")
	if err != nil {
		t.Fatalf("GetApp: %v", err)
	}

	if app.PlayStore == nil {
		t.Fatal("GetApp returned no play_store configuration")
	}
	if !app.PlayStore.ServiceAccountCredentialsConfigured {
		t.Error("ServiceAccountCredentialsConfigured = false, want true")
	}
}

// TestAppNeverEchoesTheCredential guards the assumption the write-only design
// rests on: the API returns whether a credential is set, never the credential.
// If RevenueCat ever started echoing it, storing the response in state would
// quietly reintroduce the secret this design exists to keep out.
func TestAppNeverEchoesTheCredential(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{
			"id": "app1",
			"play_store": {
				"package_name": "com.acme.app",
				"play_service_account_credentials_json": "leaked-secret",
				"play_service_account_credentials_configured": true
			}
		}`)
	}))
	defer srv.Close()

	c, _ := newTestClient(t, srv)
	app, err := c.GetApp(context.Background(), "proj1", "app1")
	if err != nil {
		t.Fatalf("GetApp: %v", err)
	}

	if app.PlayStore.ServiceAccountCredentialsJSON != "" {
		t.Error("the client kept a credential returned by the API; responses must never populate it")
	}
}
