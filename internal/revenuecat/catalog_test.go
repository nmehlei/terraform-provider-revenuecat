package revenuecat

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

// capture records the method, path and decoded body of the request the client
// sent, so each operation's wire contract is asserted in one place.
type capture struct {
	method string
	path   string
	body   map[string]any
}

func captureServer(t *testing.T, status int, response any) (*httptest.Server, *capture) {
	t.Helper()
	got := &capture{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got.method = r.Method
		got.path = r.URL.Path
		got.body = readBody(t, r)
		if response == nil {
			w.WriteHeader(status)
			return
		}
		writeJSON(t, w, status, response)
	}))
	t.Cleanup(srv.Close)
	return srv, got
}

func TestCatalogOperationWireContract(t *testing.T) {
	ptr := func(s string) *string { return &s }
	boolPtr := func(b bool) *bool { return &b }
	i64 := func(i int64) *int64 { return &i }

	tests := []struct {
		name       string
		response   any
		call       func(context.Context, *Client) error
		wantMethod string
		wantPath   string
		wantBody   map[string]any
	}{
		{
			// The API is a discriminated union: "type" alone 400s without the
			// matching nested object (play_store here — the only type this
			// provider implements, see ImplementedAppTypes).
			name:     "create app",
			response: App{ID: "app1"},
			call: func(ctx context.Context, c *Client) error {
				_, err := c.CreateApp(ctx, "proj1", CreateAppRequest{
					Name:      "Android",
					Type:      AppTypePlayStore,
					PlayStore: &PlayStoreConfig{PackageName: "com.example.app"},
				})
				return err
			},
			wantMethod: "POST",
			wantPath:   "/v2/projects/proj1/apps",
			wantBody: map[string]any{
				"name": "Android",
				"type": "play_store",
				"play_store": map[string]any{
					"package_name": "com.example.app",
				},
			},
		},
		{
			name:     "get app",
			response: App{ID: "app1"},
			call: func(ctx context.Context, c *Client) error {
				_, err := c.GetApp(ctx, "proj1", "app1")
				return err
			},
			wantMethod: "GET",
			wantPath:   "/v2/projects/proj1/apps/app1",
		},
		{
			name:     "update app",
			response: App{ID: "app1"},
			call: func(ctx context.Context, c *Client) error {
				_, err := c.UpdateApp(ctx, "proj1", "app1", UpdateAppRequest{Name: ptr("iOS v2")})
				return err
			},
			wantMethod: "POST",
			wantPath:   "/v2/projects/proj1/apps/app1",
			wantBody:   map[string]any{"name": "iOS v2"},
		},
		{
			name: "delete app",
			call: func(ctx context.Context, c *Client) error {
				return c.DeleteApp(ctx, "proj1", "app1")
			},
			wantMethod: "DELETE",
			wantPath:   "/v2/projects/proj1/apps/app1",
		},
		{
			name:     "create product",
			response: Product{ID: "prod1"},
			call: func(ctx context.Context, c *Client) error {
				_, err := c.CreateProduct(ctx, "proj1", CreateProductRequest{
					StoreIdentifier: "com.example.monthly",
					AppID:           "app1",
					Type:            ProductTypeSubscription,
					DisplayName:     ptr("Monthly"),
				})
				return err
			},
			wantMethod: "POST",
			wantPath:   "/v2/projects/proj1/products",
			wantBody: map[string]any{
				"store_identifier": "com.example.monthly",
				"app_id":           "app1",
				"type":             "subscription",
				"display_name":     "Monthly",
			},
		},
		{
			name: "delete product",
			call: func(ctx context.Context, c *Client) error {
				return c.DeleteProduct(ctx, "proj1", "prod1")
			},
			wantMethod: "DELETE",
			wantPath:   "/v2/projects/proj1/products/prod1",
		},
		{
			name:     "create entitlement",
			response: Entitlement{ID: "entl1"},
			call: func(ctx context.Context, c *Client) error {
				_, err := c.CreateEntitlement(ctx, "proj1", CreateEntitlementRequest{LookupKey: "pro", DisplayName: ptr("Pro")})
				return err
			},
			wantMethod: "POST",
			wantPath:   "/v2/projects/proj1/entitlements",
			wantBody:   map[string]any{"lookup_key": "pro", "display_name": "Pro"},
		},
		{
			name:     "update entitlement",
			response: Entitlement{ID: "entl1"},
			call: func(ctx context.Context, c *Client) error {
				_, err := c.UpdateEntitlement(ctx, "proj1", "entl1", UpdateEntitlementRequest{DisplayName: ptr("Pro Plus")})
				return err
			},
			wantMethod: "POST",
			wantPath:   "/v2/projects/proj1/entitlements/entl1",
			wantBody:   map[string]any{"display_name": "Pro Plus"},
		},
		{
			// is_current is deliberately absent: the API rejects it on create
			// ("Additional properties are not allowed") — see UpdateOfferingRequest
			// for how a resource actually marks an offering current.
			name:     "create offering",
			response: Offering{ID: "ofrng1"},
			call: func(ctx context.Context, c *Client) error {
				_, err := c.CreateOffering(ctx, "proj1", CreateOfferingRequest{
					LookupKey: "default",
					Metadata:  map[string]string{"tier": "a"},
				})
				return err
			},
			wantMethod: "POST",
			wantPath:   "/v2/projects/proj1/offerings",
			wantBody: map[string]any{
				"lookup_key": "default",
				"metadata":   map[string]any{"tier": "a"},
			},
		},
		{
			name:     "mark offering current",
			response: Offering{ID: "ofrng1", IsCurrent: true},
			call: func(ctx context.Context, c *Client) error {
				_, err := c.UpdateOffering(ctx, "proj1", "ofrng1", UpdateOfferingRequest{IsCurrent: boolPtr(true)})
				return err
			},
			wantMethod: "POST",
			wantPath:   "/v2/projects/proj1/offerings/ofrng1",
			wantBody:   map[string]any{"is_current": true},
		},
		{
			name:     "create package under offering",
			response: Package{ID: "pkg1"},
			call: func(ctx context.Context, c *Client) error {
				_, err := c.CreatePackage(ctx, "proj1", "ofrng1", CreatePackageRequest{LookupKey: "monthly", Position: i64(1)})
				return err
			},
			wantMethod: "POST",
			wantPath:   "/v2/projects/proj1/offerings/ofrng1/packages",
			wantBody:   map[string]any{"lookup_key": "monthly", "position": float64(1)},
		},
		{
			name:     "get package",
			response: Package{ID: "pkg1"},
			call: func(ctx context.Context, c *Client) error {
				_, err := c.GetPackage(ctx, "proj1", "pkg1")
				return err
			},
			wantMethod: "GET",
			wantPath:   "/v2/projects/proj1/packages/pkg1",
		},
		{
			name: "attach products to entitlement",
			call: func(ctx context.Context, c *Client) error {
				return c.AttachProductsToEntitlement(ctx, "proj1", "entl1", []string{"prod1", "prod2"})
			},
			wantMethod: "POST",
			wantPath:   "/v2/projects/proj1/entitlements/entl1/actions/attach_products",
			wantBody:   map[string]any{"product_ids": []any{"prod1", "prod2"}},
		},
		{
			name: "detach products from entitlement",
			call: func(ctx context.Context, c *Client) error {
				return c.DetachProductsFromEntitlement(ctx, "proj1", "entl1", []string{"prod1"})
			},
			wantMethod: "POST",
			wantPath:   "/v2/projects/proj1/entitlements/entl1/actions/detach_products",
			wantBody:   map[string]any{"product_ids": []any{"prod1"}},
		},
		{
			name: "attach products to package",
			call: func(ctx context.Context, c *Client) error {
				return c.AttachProductsToPackage(ctx, "proj1", "pkg1", []PackageProduct{
					{ProductID: "prod1", EligibilityCriteria: "all"},
				})
			},
			wantMethod: "POST",
			wantPath:   "/v2/projects/proj1/packages/pkg1/actions/attach_products",
			wantBody: map[string]any{
				"products": []any{map[string]any{"product_id": "prod1", "eligibility_criteria": "all"}},
			},
		},
		{
			name: "detach products from package",
			call: func(ctx context.Context, c *Client) error {
				return c.DetachProductsFromPackage(ctx, "proj1", "pkg1", []string{"prod1"})
			},
			wantMethod: "POST",
			wantPath:   "/v2/projects/proj1/packages/pkg1/actions/detach_products",
			wantBody:   map[string]any{"product_ids": []any{"prod1"}},
		},
		{
			name:     "get project",
			response: Project{ID: "proj1"},
			call: func(ctx context.Context, c *Client) error {
				_, err := c.GetProject(ctx, "proj1")
				return err
			},
			wantMethod: "GET",
			wantPath:   "/v2/projects/proj1",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv, got := captureServer(t, http.StatusOK, tc.response)
			c, _ := newTestClient(t, srv)

			if err := tc.call(context.Background(), c); err != nil {
				t.Fatalf("call: %v", err)
			}
			if got.method != tc.wantMethod {
				t.Errorf("method = %q, want %q", got.method, tc.wantMethod)
			}
			if got.path != tc.wantPath {
				t.Errorf("path = %q, want %q", got.path, tc.wantPath)
			}
			if tc.wantBody != nil && !reflect.DeepEqual(got.body, tc.wantBody) {
				t.Errorf("body = %#v, want %#v", got.body, tc.wantBody)
			}
			if tc.wantBody == nil && len(got.body) != 0 {
				t.Errorf("body = %#v, want no body", got.body)
			}
		})
	}
}

func TestAttachWithNoProductsSkipsTheCall(t *testing.T) {
	srv, got := captureServer(t, http.StatusOK, nil)
	c, _ := newTestClient(t, srv)

	if err := c.AttachProductsToEntitlement(context.Background(), "proj1", "entl1", nil); err != nil {
		t.Fatalf("AttachProductsToEntitlement: %v", err)
	}
	if err := c.DetachProductsFromPackage(context.Background(), "proj1", "pkg1", nil); err != nil {
		t.Fatalf("DetachProductsFromPackage: %v", err)
	}
	if got.method != "" {
		t.Errorf("an empty product list still issued a %s request; want none", got.method)
	}
}

func TestListPackageProductsFlattensEntries(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(t, w, http.StatusOK, listResponse[packageProductEntry]{
			Items: []packageProductEntry{
				{EligibilityCriteria: "all", Product: &Product{ID: "prod1"}},
				{EligibilityCriteria: "google_sdk_lt_6", ProductID: "prod2"},
				{EligibilityCriteria: "all"}, // no identifiable product; must be dropped
			},
		})
	}))
	defer srv.Close()

	c, _ := newTestClient(t, srv)
	got, err := c.ListPackageProducts(context.Background(), "proj1", "pkg1")
	if err != nil {
		t.Fatalf("ListPackageProducts: %v", err)
	}

	want := []PackageProduct{
		{ProductID: "prod1", EligibilityCriteria: "all"},
		{ProductID: "prod2", EligibilityCriteria: "google_sdk_lt_6"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %#v, want %#v", got, want)
	}
}

func TestModelsRoundTrip(t *testing.T) {
	tests := []struct {
		name  string
		value any
	}{
		{"project", &Project{ID: "proj1", Name: "Acme", CreatedAt: 1700000000}},
		{"app", &App{ID: "app1", Name: "iOS", Type: AppTypeAppStore, ProjectID: "proj1", CreatedAt: 1}},
		{"product", &Product{ID: "prod1", StoreIdentifier: "com.example.m", Type: ProductTypeSubscription, DisplayName: "Monthly", AppID: "app1"}},
		{"entitlement", &Entitlement{ID: "entl1", LookupKey: "pro", DisplayName: "Pro", ProjectID: "proj1"}},
		{"offering", &Offering{ID: "ofrng1", LookupKey: "default", IsCurrent: true, Metadata: map[string]string{"k": "v"}}},
		{"package", &Package{ID: "pkg1", LookupKey: "monthly", OfferingID: "ofrng1"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			encoded, err := json.Marshal(tc.value)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			decoded := reflect.New(reflect.TypeOf(tc.value).Elem()).Interface()
			if err := json.Unmarshal(encoded, decoded); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if !reflect.DeepEqual(decoded, tc.value) {
				t.Errorf("round trip changed the value:\n got %#v\nwant %#v", decoded, tc.value)
			}
		})
	}
}
