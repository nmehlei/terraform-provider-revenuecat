package provider

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/nmehlei/terraform-provider-revenuecat/internal/revenuecat"
)

// readDataSource evaluates a data source against a fake server with the given
// configuration attributes. Attributes not named are null.
func readDataSource(
	t *testing.T,
	d datasource.DataSource,
	client *revenuecat.Client,
	values map[string]tftypes.Value,
) *datasource.ReadResponse {
	t.Helper()
	ctx := context.Background()

	var schemaResp datasource.SchemaResponse
	d.Schema(ctx, datasource.SchemaRequest{}, &schemaResp)
	if schemaResp.Diagnostics.HasError() {
		t.Fatalf("data source schema: %v", schemaResp.Diagnostics)
	}

	configurable, ok := d.(datasource.DataSourceWithConfigure)
	if !ok {
		t.Fatal("data source does not implement DataSourceWithConfigure")
	}
	configureResp := &datasource.ConfigureResponse{}
	configurable.Configure(ctx, datasource.ConfigureRequest{ProviderData: client}, configureResp)
	if configureResp.Diagnostics.HasError() {
		t.Fatalf("Configure: %v", configureResp.Diagnostics)
	}

	objectType := schemaResp.Schema.Type().TerraformType(ctx).(tftypes.Object)
	attrs := make(map[string]tftypes.Value, len(objectType.AttributeTypes))
	for name, attrType := range objectType.AttributeTypes {
		attrs[name] = tftypes.NewValue(attrType, nil)
	}
	for name, value := range values {
		attrs[name] = value
	}

	req := datasource.ReadRequest{
		Config: tfsdk.Config{
			Raw:    tftypes.NewValue(objectType, attrs),
			Schema: schemaResp.Schema,
		},
	}
	resp := &datasource.ReadResponse{
		State: tfsdk.State{Raw: tftypes.NewValue(objectType, attrs), Schema: schemaResp.Schema},
	}
	d.Read(ctx, req, resp)
	return resp
}

func jsonServer(t *testing.T, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestProjectDataSourceByID(t *testing.T) {
	srv := jsonServer(t, `{"id":"proj1","name":"Acme","created_at":42}`)

	resp := readDataSource(t, NewProjectDataSource(), clientFor(t, srv), map[string]tftypes.Value{
		"id": str("proj1"),
	})
	if resp.Diagnostics.HasError() {
		t.Fatalf("Read: %v", resp.Diagnostics)
	}

	var state projectDataSourceModel
	if diags := resp.State.Get(context.Background(), &state); diags.HasError() {
		t.Fatalf("reading state: %v", diags)
	}
	if state.Name.ValueString() != "Acme" {
		t.Errorf("name = %q, want Acme", state.Name.ValueString())
	}
	if state.CreatedAt.ValueInt64() != 42 {
		t.Errorf("created_at = %d, want 42", state.CreatedAt.ValueInt64())
	}
}

func TestProjectDataSourceByName(t *testing.T) {
	srv := jsonServer(t, `{"object":"list","items":[{"id":"proj1","name":"Acme"},{"id":"proj2","name":"Other"}]}`)

	resp := readDataSource(t, NewProjectDataSource(), clientFor(t, srv), map[string]tftypes.Value{
		"name": str("Other"),
	})
	if resp.Diagnostics.HasError() {
		t.Fatalf("Read: %v", resp.Diagnostics)
	}

	var state projectDataSourceModel
	if diags := resp.State.Get(context.Background(), &state); diags.HasError() {
		t.Fatalf("reading state: %v", diags)
	}
	if state.ID.ValueString() != "proj2" {
		t.Errorf("id = %q, want proj2", state.ID.ValueString())
	}
}

func TestProjectDataSourceNameMatchesNothing(t *testing.T) {
	srv := jsonServer(t, `{"object":"list","items":[{"id":"proj1","name":"Acme"}]}`)

	resp := readDataSource(t, NewProjectDataSource(), clientFor(t, srv), map[string]tftypes.Value{
		"name": str("Absent"),
	})
	if !resp.Diagnostics.HasError() {
		t.Fatal("Read succeeded for a name matching no project; want an error")
	}
	if detail := resp.Diagnostics.Errors()[0].Detail(); !strings.Contains(detail, "Absent") {
		t.Errorf("detail = %q, want it to name the project that was searched for", detail)
	}
}

func TestProjectDataSourceNameIsAmbiguous(t *testing.T) {
	srv := jsonServer(t, `{"object":"list","items":[{"id":"proj1","name":"Acme"},{"id":"proj2","name":"Acme"}]}`)

	resp := readDataSource(t, NewProjectDataSource(), clientFor(t, srv), map[string]tftypes.Value{
		"name": str("Acme"),
	})
	if !resp.Diagnostics.HasError() {
		t.Fatal("Read succeeded for an ambiguous name; want an error rather than an arbitrary pick")
	}

	detail := resp.Diagnostics.Errors()[0].Detail()
	for _, want := range []string{"proj1", "proj2"} {
		if !strings.Contains(detail, want) {
			t.Errorf("detail = %q, want it to list the candidate %q", detail, want)
		}
	}
}

func TestProjectDataSourceRequiresExactlyOneSelector(t *testing.T) {
	srv := jsonServer(t, `{}`)

	t.Run("neither", func(t *testing.T) {
		resp := readDataSource(t, NewProjectDataSource(), clientFor(t, srv), nil)
		if !resp.Diagnostics.HasError() {
			t.Fatal("Read succeeded with no selector; want an error")
		}
	})

	t.Run("both", func(t *testing.T) {
		resp := readDataSource(t, NewProjectDataSource(), clientFor(t, srv), map[string]tftypes.Value{
			"id":   str("proj1"),
			"name": str("Acme"),
		})
		if !resp.Diagnostics.HasError() {
			t.Fatal("Read succeeded with both selectors; want an error")
		}
		if detail := resp.Diagnostics.Errors()[0].Detail(); !strings.Contains(detail, "mutually exclusive") {
			t.Errorf("detail = %q, want it to report the selectors as mutually exclusive", detail)
		}
	})
}

func TestProjectsDataSourceListsEverything(t *testing.T) {
	srv := jsonServer(t, `{"object":"list","items":[{"id":"proj1","name":"Acme"},{"id":"proj2","name":"Other"}]}`)

	resp := readDataSource(t, NewProjectsDataSource(), clientFor(t, srv), nil)
	if resp.Diagnostics.HasError() {
		t.Fatalf("Read: %v", resp.Diagnostics)
	}

	var state projectsDataSourceModel
	if diags := resp.State.Get(context.Background(), &state); diags.HasError() {
		t.Fatalf("reading state: %v", diags)
	}
	if len(state.Projects) != 2 {
		t.Fatalf("got %d projects, want 2", len(state.Projects))
	}
	if state.Projects[0].ID.ValueString() != "proj1" {
		t.Errorf("first project = %q, want proj1", state.Projects[0].ID.ValueString())
	}
}

func TestEntitlementDataSourceResolvesLookupKeyByListing(t *testing.T) {
	srv := jsonServer(t, `{"object":"list","items":[
		{"id":"entl1","lookup_key":"basic","display_name":"Basic"},
		{"id":"entl2","lookup_key":"pro","display_name":"Pro","created_at":7}
	]}`)

	resp := readDataSource(t, NewEntitlementDataSource(), clientFor(t, srv), map[string]tftypes.Value{
		"project_id": str("proj1"),
		"lookup_key": str("pro"),
	})
	if resp.Diagnostics.HasError() {
		t.Fatalf("Read: %v", resp.Diagnostics)
	}

	var state entitlementModel
	if diags := resp.State.Get(context.Background(), &state); diags.HasError() {
		t.Fatalf("reading state: %v", diags)
	}
	if state.ID.ValueString() != "entl2" {
		t.Errorf("id = %q, want entl2", state.ID.ValueString())
	}
	if state.DisplayName.ValueString() != "Pro" {
		t.Errorf("display_name = %q, want Pro", state.DisplayName.ValueString())
	}
	if state.CreatedAt.ValueInt64() != 7 {
		t.Errorf("created_at = %d, want 7", state.CreatedAt.ValueInt64())
	}
}

func TestCatalogDataSourceLookupKeyMissMentionsTheKey(t *testing.T) {
	srv := jsonServer(t, `{"object":"list","items":[{"id":"entl1","lookup_key":"basic"}]}`)

	resp := readDataSource(t, NewEntitlementDataSource(), clientFor(t, srv), map[string]tftypes.Value{
		"project_id": str("proj1"),
		"lookup_key": str("absent-key"),
	})
	if !resp.Diagnostics.HasError() {
		t.Fatal("Read succeeded for an absent lookup key; want an error")
	}
	if detail := resp.Diagnostics.Errors()[0].Detail(); !strings.Contains(detail, "absent-key") {
		t.Errorf("detail = %q, want it to name the key that was searched for", detail)
	}
}

func TestPackageDataSourceResolvesWithinItsOffering(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"object":"list","items":[{"id":"pkg1","lookup_key":"$rc_monthly"}]}`)
	}))
	defer srv.Close()

	resp := readDataSource(t, NewPackageDataSource(), clientFor(t, srv), map[string]tftypes.Value{
		"project_id":  str("proj1"),
		"offering_id": str("ofrng1"),
		"lookup_key":  str("$rc_monthly"),
	})
	if resp.Diagnostics.HasError() {
		t.Fatalf("Read: %v", resp.Diagnostics)
	}

	if want := "/v2/projects/proj1/offerings/ofrng1/packages"; gotPath != want {
		t.Errorf("listed %q, want the packages of the named offering %q", gotPath, want)
	}

	var state packageModel
	if diags := resp.State.Get(context.Background(), &state); diags.HasError() {
		t.Fatalf("reading state: %v", diags)
	}
	if state.ID.ValueString() != "pkg1" {
		t.Errorf("id = %q, want pkg1", state.ID.ValueString())
	}
}

func TestCatalogDataSourcesRejectBothSelectors(t *testing.T) {
	srv := jsonServer(t, `{}`)
	client := clientFor(t, srv)

	tests := []struct {
		name       string
		dataSource datasource.DataSource
		values     map[string]tftypes.Value
	}{
		{
			name:       "app",
			dataSource: NewAppDataSource(),
			values:     map[string]tftypes.Value{"project_id": str("p"), "id": str("a"), "name": str("n")},
		},
		{
			name:       "product",
			dataSource: NewProductDataSource(),
			values:     map[string]tftypes.Value{"project_id": str("p"), "id": str("a"), "store_identifier": str("s")},
		},
		{
			name:       "entitlement",
			dataSource: NewEntitlementDataSource(),
			values:     map[string]tftypes.Value{"project_id": str("p"), "id": str("a"), "lookup_key": str("k")},
		},
		{
			name:       "offering",
			dataSource: NewOfferingDataSource(),
			values:     map[string]tftypes.Value{"project_id": str("p"), "id": str("a"), "lookup_key": str("k")},
		},
		{
			name:       "package",
			dataSource: NewPackageDataSource(),
			values:     map[string]tftypes.Value{"project_id": str("p"), "offering_id": str("o"), "id": str("a"), "lookup_key": str("k")},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp := readDataSource(t, tc.dataSource, client, tc.values)
			if !resp.Diagnostics.HasError() {
				t.Fatal("Read succeeded with both selectors set; want an error")
			}
			if detail := resp.Diagnostics.Errors()[0].Detail(); !strings.Contains(detail, "mutually exclusive") {
				t.Errorf("detail = %q, want it to report the selectors as mutually exclusive", detail)
			}
		})
	}
}

// TestDataSourcesOnlyIssueReads guards the read-only contract: a data source
// must never mutate anything on the RevenueCat side.
func TestDataSourcesOnlyIssueReads(t *testing.T) {
	var methods []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methods = append(methods, r.Method)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"object":"list","items":[{"id":"x","name":"x","lookup_key":"k","store_identifier":"k"}]}`)
	}))
	defer srv.Close()

	client := clientFor(t, srv)

	cases := []struct {
		dataSource datasource.DataSource
		values     map[string]tftypes.Value
	}{
		{NewProjectDataSource(), map[string]tftypes.Value{"id": str("p")}},
		{NewProjectsDataSource(), nil},
		{NewAppDataSource(), map[string]tftypes.Value{"project_id": str("p"), "name": str("x")}},
		{NewProductDataSource(), map[string]tftypes.Value{"project_id": str("p"), "store_identifier": str("k")}},
		{NewEntitlementDataSource(), map[string]tftypes.Value{"project_id": str("p"), "lookup_key": str("k")}},
		{NewOfferingDataSource(), map[string]tftypes.Value{"project_id": str("p"), "lookup_key": str("k")}},
		{NewPackageDataSource(), map[string]tftypes.Value{"project_id": str("p"), "offering_id": str("o"), "lookup_key": str("k")}},
	}

	for _, tc := range cases {
		readDataSource(t, tc.dataSource, client, tc.values)
	}

	if len(methods) == 0 {
		t.Fatal("no requests were made; the test is not exercising anything")
	}
	for _, method := range methods {
		if method != http.MethodGet {
			t.Errorf("a data source issued a %s request; data sources must only read", method)
		}
	}
}
