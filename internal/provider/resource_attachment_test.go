package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// recordedCall is one request the resource made while under test.
type recordedCall struct {
	Path       string
	ProductIDs []string
}

// recordingServer collects attach and detach calls so a test can assert on
// exactly which products were touched.
type recordingServer struct {
	mu    sync.Mutex
	calls []recordedCall
}

func (s *recordingServer) handler(t *testing.T) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)

		var payload struct {
			ProductIDs []string `json:"product_ids"`
			Products   []struct {
				ProductID string `json:"product_id"`
			} `json:"products"`
		}
		_ = json.Unmarshal(body, &payload)

		ids := payload.ProductIDs
		for _, product := range payload.Products {
			ids = append(ids, product.ProductID)
		}

		s.mu.Lock()
		s.calls = append(s.calls, recordedCall{Path: r.URL.Path, ProductIDs: ids})
		s.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{}`)
	}
}

// productIDsFor returns the product IDs sent to the first call whose path ends
// with the given action.
func (s *recordingServer) productIDsFor(action string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, call := range s.calls {
		if strings.HasSuffix(call.Path, action) {
			return call.ProductIDs
		}
	}
	return nil
}

func (s *recordingServer) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.calls)
}

// attachmentStateWith builds an entitlement attachment state carrying the given
// product IDs.
func attachmentStateWith(ctx context.Context, t *testing.T, s tfsdk.State, productIDs []string) tfsdk.State {
	t.Helper()

	elements := make([]tftypes.Value, 0, len(productIDs))
	for _, id := range productIDs {
		elements = append(elements, tftypes.NewValue(tftypes.String, id))
	}

	object := s.Raw.Type().(tftypes.Object)

	decoded := map[string]tftypes.Value{}
	if err := s.Raw.As(&decoded); err != nil {
		t.Fatalf("decoding state: %v", err)
	}

	// As hands back the value's own map rather than a copy, so building two
	// states from one base would otherwise have the second overwrite the first.
	attrs := make(map[string]tftypes.Value, len(decoded))
	for name, value := range decoded {
		attrs[name] = value
	}
	attrs["product_ids"] = tftypes.NewValue(tftypes.Set{ElementType: tftypes.String}, elements)

	return tfsdk.State{Raw: tftypes.NewValue(object, attrs), Schema: s.Schema}
}

func TestEntitlementAttachmentUpdateSendsOnlyTheDifference(t *testing.T) {
	ctx := context.Background()
	recorder := &recordingServer{}
	srv := httptest.NewServer(recorder.handler(t))
	defer srv.Close()

	r := NewEntitlementProductAttachmentResource()
	configureResp := &resource.ConfigureResponse{}
	r.(resource.ResourceWithConfigure).Configure(ctx, resource.ConfigureRequest{ProviderData: clientFor(t, srv)}, configureResp)

	var schemaResp resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &schemaResp)

	base := stateFor(ctx, schemaResp.Schema)
	state := attachmentStateWith(ctx, t, base, []string{"prodA", "prodB"})
	plan := attachmentStateWith(ctx, t, base, []string{"prodB", "prodC"})

	resp := &resource.UpdateResponse{State: tfsdk.State{Raw: state.Raw, Schema: schemaResp.Schema}}
	r.Update(ctx, resource.UpdateRequest{
		Plan:  tfsdk.Plan{Raw: plan.Raw, Schema: schemaResp.Schema},
		State: state,
	}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("Update: %v", resp.Diagnostics)
	}

	attached := recorder.productIDsFor("attach_products")
	detached := recorder.productIDsFor("detach_products")

	if !reflect.DeepEqual(attached, []string{"prodC"}) {
		t.Errorf("attached %v, want only [prodC]", attached)
	}
	if !reflect.DeepEqual(detached, []string{"prodA"}) {
		t.Errorf("detached %v, want only [prodA]", detached)
	}
	for _, id := range append(append([]string{}, attached...), detached...) {
		if id == "prodB" {
			t.Error("prodB was touched; an unchanged product must be left attached")
		}
	}
}

func TestEntitlementAttachmentUpdateWithNoChangeMakesNoCall(t *testing.T) {
	ctx := context.Background()
	recorder := &recordingServer{}
	srv := httptest.NewServer(recorder.handler(t))
	defer srv.Close()

	r := NewEntitlementProductAttachmentResource()
	configureResp := &resource.ConfigureResponse{}
	r.(resource.ResourceWithConfigure).Configure(ctx, resource.ConfigureRequest{ProviderData: clientFor(t, srv)}, configureResp)

	var schemaResp resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &schemaResp)

	base := stateFor(ctx, schemaResp.Schema)
	// Same products, different order: the set is unchanged.
	state := attachmentStateWith(ctx, t, base, []string{"prodA", "prodB"})
	plan := attachmentStateWith(ctx, t, base, []string{"prodB", "prodA"})

	resp := &resource.UpdateResponse{State: tfsdk.State{Raw: state.Raw, Schema: schemaResp.Schema}}
	r.Update(ctx, resource.UpdateRequest{
		Plan:  tfsdk.Plan{Raw: plan.Raw, Schema: schemaResp.Schema},
		State: state,
	}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("Update: %v", resp.Diagnostics)
	}
	if got := recorder.callCount(); got != 0 {
		t.Errorf("made %d API calls for a reordered but unchanged set, want 0", got)
	}
}

func TestEntitlementAttachmentDeleteDetachesOnlyItsProducts(t *testing.T) {
	ctx := context.Background()
	recorder := &recordingServer{}
	srv := httptest.NewServer(recorder.handler(t))
	defer srv.Close()

	r := NewEntitlementProductAttachmentResource()
	configureResp := &resource.ConfigureResponse{}
	r.(resource.ResourceWithConfigure).Configure(ctx, resource.ConfigureRequest{ProviderData: clientFor(t, srv)}, configureResp)

	var schemaResp resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &schemaResp)

	base := stateFor(ctx, schemaResp.Schema)
	state := attachmentStateWith(ctx, t, base, []string{"prodA", "prodB"})

	resp := &resource.DeleteResponse{State: tfsdk.State{Raw: state.Raw, Schema: schemaResp.Schema}}
	r.Delete(ctx, resource.DeleteRequest{State: state}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("Delete: %v", resp.Diagnostics)
	}

	recorder.mu.Lock()
	defer recorder.mu.Unlock()

	if len(recorder.calls) != 1 {
		t.Fatalf("made %d calls, want exactly 1 detach", len(recorder.calls))
	}
	call := recorder.calls[0]
	if !strings.HasSuffix(call.Path, "detach_products") {
		t.Errorf("called %q, want the detach action", call.Path)
	}
	// Destroying the relationship must not destroy the objects it connects.
	if strings.Contains(call.Path, "/products/") {
		t.Errorf("called %q; destroying an attachment must not delete products", call.Path)
	}
}

// TestPackageProductBlockConversionRoundTrips checks the block-to-spec mapping
// the package attachment relies on, including a null eligibility criteria.
func TestPackageProductBlockConversionRoundTrips(t *testing.T) {
	ctx := context.Background()

	blocks := []packageProductBlockModel{
		{ProductID: types.StringValue("prodA"), EligibilityCriteria: types.StringValue("all")},
		{ProductID: types.StringValue("prodB"), EligibilityCriteria: types.StringNull()},
	}

	set, diags := types.SetValueFrom(ctx, packageProductObjectType(), blocks)
	if diags.HasError() {
		t.Fatalf("building the set: %v", diags)
	}

	var convertDiags diag.Diagnostics
	specs := packageProductsFromFramework(ctx, set, &convertDiags)
	if convertDiags.HasError() {
		t.Fatalf("converting: %v", convertDiags)
	}

	want := map[string]string{"prodA": "all", "prodB": ""}
	if len(specs) != len(want) {
		t.Fatalf("got %d specs, want %d", len(specs), len(want))
	}
	for _, spec := range specs {
		criteria, ok := want[spec.ProductID]
		if !ok {
			t.Errorf("unexpected product %q", spec.ProductID)
			continue
		}
		if spec.EligibilityCriteria != criteria {
			t.Errorf("product %q criteria = %q, want %q", spec.ProductID, spec.EligibilityCriteria, criteria)
		}
	}
}

func TestPackageProductObjectTypeMatchesTheBlockSchema(t *testing.T) {
	want := map[string]attr.Type{
		"product_id":           types.StringType,
		"eligibility_criteria": types.StringType,
	}
	if got := packageProductObjectType().AttrTypes; !reflect.DeepEqual(got, want) {
		t.Errorf("attribute types = %#v, want %#v", got, want)
	}
}
