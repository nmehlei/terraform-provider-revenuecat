package provider

import (
	"context"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	fwresource "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"

	"github.com/nmehlei/terraform-provider-revenuecat/internal/revenuecat"
)

// fillValue produces a plausible non-null value for a Terraform type, so a
// resource's state can be synthesized from its schema alone. This keeps the
// not-found tests generic: adding a resource does not mean writing a new
// fixture by hand.
func fillValue(typ tftypes.Type) tftypes.Value {
	switch {
	case typ.Is(tftypes.String):
		return tftypes.NewValue(tftypes.String, "x")
	case typ.Is(tftypes.Number):
		return tftypes.NewValue(tftypes.Number, big.NewFloat(1))
	case typ.Is(tftypes.Bool):
		return tftypes.NewValue(tftypes.Bool, false)
	}

	switch concrete := typ.(type) {
	case tftypes.Set:
		return tftypes.NewValue(concrete, []tftypes.Value{fillValue(concrete.ElementType)})
	case tftypes.List:
		return tftypes.NewValue(concrete, []tftypes.Value{fillValue(concrete.ElementType)})
	case tftypes.Map:
		return tftypes.NewValue(concrete, map[string]tftypes.Value{"k": fillValue(concrete.ElementType)})
	case tftypes.Object:
		attrs := make(map[string]tftypes.Value, len(concrete.AttributeTypes))
		for name, attrType := range concrete.AttributeTypes {
			attrs[name] = fillValue(attrType)
		}
		return tftypes.NewValue(concrete, attrs)
	}

	return tftypes.NewValue(typ, nil)
}

// stateFor builds a fully populated state value for a resource schema.
func stateFor(ctx context.Context, s fwresource.Schema) tfsdk.State {
	objectType := s.Type().TerraformType(ctx).(tftypes.Object)
	return tfsdk.State{
		Raw:    fillValue(objectType),
		Schema: s,
	}
}

// notFoundServer answers every request with a 404 error body.
func notFoundServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"code":"not_found","message":"no such object"}`)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func clientFor(t *testing.T, srv *httptest.Server) *revenuecat.Client {
	t.Helper()
	client, err := revenuecat.New("sk-test", srv.URL+"/v2", revenuecat.WithMaxRetries(0))
	if err != nil {
		t.Fatalf("building the test client: %v", err)
	}
	return client
}

// eachResource yields every registered resource, already configured against
// the given client.
func eachResource(t *testing.T, client *revenuecat.Client, fn func(t *testing.T, typeName string, r resource.Resource, s fwresource.Schema)) {
	t.Helper()
	ctx := context.Background()

	for _, newResource := range New("test")().Resources(ctx) {
		r := newResource()

		var metadata resource.MetadataResponse
		r.Metadata(ctx, resource.MetadataRequest{ProviderTypeName: "revenuecat"}, &metadata)

		var schemaResp resource.SchemaResponse
		r.Schema(ctx, resource.SchemaRequest{}, &schemaResp)

		configurable, ok := r.(resource.ResourceWithConfigure)
		if !ok {
			t.Fatalf("%s does not implement ResourceWithConfigure", metadata.TypeName)
		}
		configureResp := &resource.ConfigureResponse{}
		configurable.Configure(ctx, resource.ConfigureRequest{ProviderData: client}, configureResp)
		if configureResp.Diagnostics.HasError() {
			t.Fatalf("%s Configure: %v", metadata.TypeName, configureResp.Diagnostics)
		}

		t.Run(metadata.TypeName, func(t *testing.T) {
			fn(t, metadata.TypeName, r, schemaResp.Schema)
		})
	}
}

// TestReadRemovesResourceWhenGone covers the drift case that matters most: an
// object deleted outside Terraform must leave state rather than fail the plan.
func TestReadRemovesResourceWhenGone(t *testing.T) {
	ctx := context.Background()
	client := clientFor(t, notFoundServer(t))

	eachResource(t, client, func(t *testing.T, typeName string, r resource.Resource, s fwresource.Schema) {
		state := stateFor(ctx, s)
		resp := &resource.ReadResponse{State: tfsdk.State{Raw: state.Raw, Schema: s}}

		r.Read(ctx, resource.ReadRequest{State: state}, resp)

		if resp.Diagnostics.HasError() {
			t.Fatalf("Read returned an error for a deleted object: %v", resp.Diagnostics)
		}
		if !resp.State.Raw.IsNull() {
			t.Error("Read left the resource in state; a deleted object must be removed so Terraform plans its recreation")
		}
	})
}

// TestDeleteToleratesAlreadyDeleted covers destroying something that is already
// gone, which must be a no-op rather than an error.
func TestDeleteToleratesAlreadyDeleted(t *testing.T) {
	ctx := context.Background()
	client := clientFor(t, notFoundServer(t))

	eachResource(t, client, func(t *testing.T, typeName string, r resource.Resource, s fwresource.Schema) {
		state := stateFor(ctx, s)
		resp := &resource.DeleteResponse{State: tfsdk.State{Raw: state.Raw, Schema: s}}

		r.Delete(ctx, resource.DeleteRequest{State: state}, resp)

		if resp.Diagnostics.HasError() {
			t.Errorf("Delete of an already-deleted object returned an error: %v", resp.Diagnostics)
		}
	})
}

// TestReadReflectsDriftedValues checks the other half of refresh: a value
// changed remotely is written back to state so the next plan shows the drift.
func TestReadReflectsDriftedValues(t *testing.T) {
	ctx := context.Background()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":"entl1","lookup_key":"pro","display_name":"Renamed Remotely","project_id":"proj1","created_at":42}`)
	}))
	defer srv.Close()

	r := NewEntitlementResource()
	configureResp := &resource.ConfigureResponse{}
	r.(resource.ResourceWithConfigure).Configure(ctx, resource.ConfigureRequest{ProviderData: clientFor(t, srv)}, configureResp)

	var schemaResp resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &schemaResp)

	state := stateFor(ctx, schemaResp.Schema)
	resp := &resource.ReadResponse{State: tfsdk.State{Raw: state.Raw, Schema: schemaResp.Schema}}
	r.Read(ctx, resource.ReadRequest{State: state}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("Read: %v", resp.Diagnostics)
	}

	var refreshed entitlementModel
	if diags := resp.State.Get(ctx, &refreshed); diags.HasError() {
		t.Fatalf("reading refreshed state: %v", diags)
	}
	if got := refreshed.DisplayName.ValueString(); got != "Renamed Remotely" {
		t.Errorf("display_name = %q, want the remote value so the drift is visible in the next plan", got)
	}
	if got := refreshed.CreatedAt.ValueInt64(); got != 42 {
		t.Errorf("created_at = %d, want 42", got)
	}
}

// TestAttachmentReadRemovesResourceWhenNothingAttached covers the case where
// the parent still exists but every product was detached elsewhere.
func TestAttachmentReadRemovesResourceWhenNothingAttached(t *testing.T) {
	ctx := context.Background()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"object":"list","items":[]}`)
	}))
	defer srv.Close()

	r := NewEntitlementProductAttachmentResource()
	configureResp := &resource.ConfigureResponse{}
	r.(resource.ResourceWithConfigure).Configure(ctx, resource.ConfigureRequest{ProviderData: clientFor(t, srv)}, configureResp)

	var schemaResp resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &schemaResp)

	state := stateFor(ctx, schemaResp.Schema)
	resp := &resource.ReadResponse{State: tfsdk.State{Raw: state.Raw, Schema: schemaResp.Schema}}
	r.Read(ctx, resource.ReadRequest{State: state}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("Read: %v", resp.Diagnostics)
	}
	if !resp.State.Raw.IsNull() {
		t.Error("an attachment whose products were all detached elsewhere must be removed from state")
	}
}

// TestAttachmentReadReflectsPartialDetach checks that a product detached
// outside Terraform shows up as drift rather than being ignored.
func TestAttachmentReadReflectsPartialDetach(t *testing.T) {
	ctx := context.Background()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"object":"list","items":[{"id":"prod1"}]}`)
	}))
	defer srv.Close()

	r := NewEntitlementProductAttachmentResource()
	configureResp := &resource.ConfigureResponse{}
	r.(resource.ResourceWithConfigure).Configure(ctx, resource.ConfigureRequest{ProviderData: clientFor(t, srv)}, configureResp)

	var schemaResp resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &schemaResp)

	state := stateFor(ctx, schemaResp.Schema)
	resp := &resource.ReadResponse{State: tfsdk.State{Raw: state.Raw, Schema: schemaResp.Schema}}
	r.Read(ctx, resource.ReadRequest{State: state}, resp)

	if resp.Diagnostics.HasError() {
		t.Fatalf("Read: %v", resp.Diagnostics)
	}

	var refreshed entitlementProductAttachmentModel
	if diags := resp.State.Get(ctx, &refreshed); diags.HasError() {
		t.Fatalf("reading refreshed state: %v", diags)
	}

	var ids []string
	if diags := refreshed.ProductIDs.ElementsAs(ctx, &ids, false); diags.HasError() {
		t.Fatalf("reading product_ids: %v", diags)
	}
	if len(ids) != 1 || ids[0] != "prod1" {
		t.Errorf("product_ids = %v, want [prod1] reflecting what is actually attached", ids)
	}
}
