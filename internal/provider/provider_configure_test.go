package provider

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/nmehlei/terraform-provider-revenuecat/internal/revenuecat"
)

// configureWith runs the provider's Configure with the given raw attribute
// values. A nil entry means the attribute is absent from configuration;
// tftypes.UnknownValue marks it unknown at plan time.
func configureWith(t *testing.T, values map[string]tftypes.Value) provider.ConfigureResponse {
	t.Helper()
	ctx := context.Background()
	p := New("test")()

	var schemaResp provider.SchemaResponse
	p.Schema(ctx, provider.SchemaRequest{}, &schemaResp)
	if schemaResp.Diagnostics.HasError() {
		t.Fatalf("provider schema: %v", schemaResp.Diagnostics)
	}

	objectType := tftypes.Object{
		AttributeTypes: map[string]tftypes.Type{
			"api_key":                 tftypes.String,
			"base_url":                tftypes.String,
			"max_retries":             tftypes.Number,
			"request_timeout_seconds": tftypes.Number,
		},
	}

	raw := map[string]tftypes.Value{
		"api_key":                 tftypes.NewValue(tftypes.String, nil),
		"base_url":                tftypes.NewValue(tftypes.String, nil),
		"max_retries":             tftypes.NewValue(tftypes.Number, nil),
		"request_timeout_seconds": tftypes.NewValue(tftypes.Number, nil),
	}
	for name, value := range values {
		raw[name] = value
	}

	req := provider.ConfigureRequest{
		Config: tfsdk.Config{
			Raw:    tftypes.NewValue(objectType, raw),
			Schema: schemaResp.Schema,
		},
	}
	resp := provider.ConfigureResponse{}
	p.Configure(ctx, req, &resp)
	return resp
}

func str(v string) tftypes.Value { return tftypes.NewValue(tftypes.String, v) }
func num(v int64) tftypes.Value  { return tftypes.NewValue(tftypes.Number, v) }
func unknownString() tftypes.Value {
	return tftypes.NewValue(tftypes.String, tftypes.UnknownValue)
}

func TestConfigureUsesAPIKeyFromConfiguration(t *testing.T) {
	t.Setenv(envAPIKey, "")

	resp := configureWith(t, map[string]tftypes.Value{"api_key": str("sk-from-config")})
	if resp.Diagnostics.HasError() {
		t.Fatalf("Configure: %v", resp.Diagnostics)
	}
	if resp.ResourceData == nil {
		t.Fatal("Configure did not set ResourceData")
	}
	if _, ok := resp.ResourceData.(*revenuecat.Client); !ok {
		t.Errorf("ResourceData is %T, want *revenuecat.Client", resp.ResourceData)
	}
	if resp.DataSourceData == nil {
		t.Error("Configure did not set DataSourceData")
	}
}

func TestConfigureFallsBackToEnvironment(t *testing.T) {
	t.Setenv(envAPIKey, "sk-from-env")

	resp := configureWith(t, nil)
	if resp.Diagnostics.HasError() {
		t.Fatalf("Configure: %v", resp.Diagnostics)
	}
	if resp.ResourceData == nil {
		t.Fatal("Configure did not build a client from the environment variable")
	}
}

func TestConfigureRequiresAnAPIKey(t *testing.T) {
	t.Setenv(envAPIKey, "")

	resp := configureWith(t, nil)
	if !resp.Diagnostics.HasError() {
		t.Fatal("Configure succeeded with no API key; want an error")
	}

	summary := resp.Diagnostics.Errors()[0].Summary()
	detail := resp.Diagnostics.Errors()[0].Detail()
	if !strings.Contains(summary, "API key") {
		t.Errorf("summary = %q, want it to mention the API key", summary)
	}
	if !strings.Contains(detail, envAPIKey) {
		t.Errorf("detail = %q, want it to name the %s environment variable", detail, envAPIKey)
	}
}

func TestConfigureDefersOnUnknownAPIKey(t *testing.T) {
	t.Setenv(envAPIKey, "")

	// An unknown value may be filled in by another resource during apply, so it
	// must not be reported as a missing credential at plan time.
	resp := configureWith(t, map[string]tftypes.Value{"api_key": unknownString()})
	if resp.Diagnostics.HasError() {
		t.Errorf("Configure reported an error for an unknown api_key: %v", resp.Diagnostics)
	}
	if resp.ResourceData != nil {
		t.Error("Configure built a client from an unknown api_key; it should defer")
	}
}

func TestConfigureRejectsMalformedBaseURL(t *testing.T) {
	t.Setenv(envAPIKey, "sk-test")

	resp := configureWith(t, map[string]tftypes.Value{"base_url": str("not-a-url")})
	if !resp.Diagnostics.HasError() {
		t.Fatal("Configure accepted a malformed base_url; want an error")
	}
	if summary := resp.Diagnostics.Errors()[0].Summary(); !strings.Contains(summary, "base URL") {
		t.Errorf("summary = %q, want it to mention the base URL", summary)
	}
}

func TestConfigureAcceptsBaseURLOverride(t *testing.T) {
	t.Setenv(envAPIKey, "sk-test")

	resp := configureWith(t, map[string]tftypes.Value{"base_url": str("http://127.0.0.1:8080/v2")})
	if resp.Diagnostics.HasError() {
		t.Fatalf("Configure: %v", resp.Diagnostics)
	}

	client, ok := resp.ResourceData.(*revenuecat.Client)
	if !ok {
		t.Fatalf("ResourceData is %T, want *revenuecat.Client", resp.ResourceData)
	}
	if got, want := client.BaseURL(), "http://127.0.0.1:8080/v2"; got != want {
		t.Errorf("BaseURL = %q, want %q", got, want)
	}
}

func TestConfigureUsesEnvironmentBaseURL(t *testing.T) {
	t.Setenv(envAPIKey, "sk-test")
	t.Setenv(envBaseURL, "http://127.0.0.1:9000/v2")

	resp := configureWith(t, nil)
	if resp.Diagnostics.HasError() {
		t.Fatalf("Configure: %v", resp.Diagnostics)
	}

	client := resp.ResourceData.(*revenuecat.Client)
	if got, want := client.BaseURL(), "http://127.0.0.1:9000/v2"; got != want {
		t.Errorf("BaseURL = %q, want %q", got, want)
	}
}

func TestConfigureDefaultsBaseURL(t *testing.T) {
	t.Setenv(envAPIKey, "sk-test")
	t.Setenv(envBaseURL, "")

	resp := configureWith(t, nil)
	if resp.Diagnostics.HasError() {
		t.Fatalf("Configure: %v", resp.Diagnostics)
	}

	client := resp.ResourceData.(*revenuecat.Client)
	if got := client.BaseURL(); got != revenuecat.DefaultBaseURL {
		t.Errorf("BaseURL = %q, want the default %q", got, revenuecat.DefaultBaseURL)
	}
}

func TestConfigureRejectsNegativeTuningValues(t *testing.T) {
	t.Setenv(envAPIKey, "sk-test")

	tests := []struct {
		name      string
		attribute string
	}{
		{"negative retries", "max_retries"},
		{"negative timeout", "request_timeout_seconds"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp := configureWith(t, map[string]tftypes.Value{tc.attribute: num(-1)})
			if !resp.Diagnostics.HasError() {
				t.Fatalf("Configure accepted a negative %s; want an error", tc.attribute)
			}
		})
	}
}

func TestConfigureAcceptsTuningValues(t *testing.T) {
	t.Setenv(envAPIKey, "sk-test")

	resp := configureWith(t, map[string]tftypes.Value{
		"max_retries":             num(5),
		"request_timeout_seconds": num(60),
	})
	if resp.Diagnostics.HasError() {
		t.Fatalf("Configure: %v", resp.Diagnostics)
	}
	if resp.ResourceData == nil {
		t.Error("Configure did not build a client")
	}
}

func TestConfigurationTakesPrecedenceOverEnvironment(t *testing.T) {
	t.Setenv(envAPIKey, "sk-from-env")
	t.Setenv(envBaseURL, "http://from-env.invalid/v2")

	resp := configureWith(t, map[string]tftypes.Value{
		"base_url": str("http://from-config.invalid/v2"),
	})
	if resp.Diagnostics.HasError() {
		t.Fatalf("Configure: %v", resp.Diagnostics)
	}

	client := resp.ResourceData.(*revenuecat.Client)
	if got, want := client.BaseURL(), "http://from-config.invalid/v2"; got != want {
		t.Errorf("BaseURL = %q, want the configured value %q", got, want)
	}
}

// TestAPIKeyPrecedenceOnTheWire pins which key the configured client actually
// sends. The client deliberately does not expose its key, so the only honest
// way to assert precedence is to observe the Authorization header.
func TestAPIKeyPrecedenceOnTheWire(t *testing.T) {
	tests := []struct {
		name      string
		configKey string
		envKey    string
		wantAuth  string
	}{
		{"config wins over environment", "sk-from-config", "sk-from-env", "Bearer sk-from-config"},
		{"environment used when config is absent", "", "sk-from-env", "Bearer sk-from-env"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var gotAuth string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotAuth = r.Header.Get("Authorization")
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, `{"id":"proj1","name":"Acme"}`)
			}))
			defer srv.Close()

			t.Setenv(envAPIKey, tc.envKey)

			values := map[string]tftypes.Value{"base_url": str(srv.URL + "/v2")}
			if tc.configKey != "" {
				values["api_key"] = str(tc.configKey)
			}

			resp := configureWith(t, values)
			if resp.Diagnostics.HasError() {
				t.Fatalf("Configure: %v", resp.Diagnostics)
			}

			client := resp.ResourceData.(*revenuecat.Client)
			if _, err := client.GetProject(context.Background(), "proj1"); err != nil {
				t.Fatalf("GetProject: %v", err)
			}
			if gotAuth != tc.wantAuth {
				t.Errorf("Authorization = %q, want %q", gotAuth, tc.wantAuth)
			}
		})
	}
}
