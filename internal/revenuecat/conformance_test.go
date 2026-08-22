package revenuecat_test

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/nmehlei/terraform-provider-revenuecat/internal/mockrevenuecat"
	"github.com/nmehlei/terraform-provider-revenuecat/internal/revenuecat"
)

// TestClientConformance drives the real client through a full catalog lifecycle
// against the stateful mock. Where the unit tests assert the shape of a single
// request, this asserts that a sequence of operations composes: what Create
// wrote, Read gets back; what Update changed, Read reflects; what Delete
// removed, Read reports gone.
//
// Both sides encode the same assumed API contract, so agreement proves internal
// consistency rather than fidelity to the real RevenueCat service.
func TestClientConformance(t *testing.T) {
	ctx := context.Background()

	mock := mockrevenuecat.New(mockrevenuecat.Options{PageSize: 2})
	server := httptest.NewServer(mock)
	defer server.Close()

	projectID := mock.CreateProject("Acme")

	client, err := revenuecat.New("sk-test", server.URL+"/v2")
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	t.Run("project", func(t *testing.T) {
		project, err := client.GetProject(ctx, projectID)
		if err != nil {
			t.Fatalf("GetProject: %v", err)
		}
		if project.Name != "Acme" {
			t.Errorf("name = %q, want Acme", project.Name)
		}

		projects, err := client.ListProjects(ctx)
		if err != nil {
			t.Fatalf("ListProjects: %v", err)
		}
		if len(projects) != 1 {
			t.Errorf("listed %d projects, want 1", len(projects))
		}
	})

	var appID string
	t.Run("app", func(t *testing.T) {
		app, err := client.CreateApp(ctx, projectID, revenuecat.CreateAppRequest{
			Name: "Acme iOS",
			Type: revenuecat.AppTypeAppStore,
		})
		if err != nil {
			t.Fatalf("CreateApp: %v", err)
		}
		if app.ID == "" || app.CreatedAt == 0 {
			t.Fatalf("created app is missing an id or timestamp: %+v", app)
		}
		appID = app.ID

		read, err := client.GetApp(ctx, projectID, appID)
		if err != nil {
			t.Fatalf("GetApp: %v", err)
		}
		if read.Name != "Acme iOS" || read.Type != revenuecat.AppTypeAppStore {
			t.Errorf("read back %+v, want the values that were created", read)
		}

		name := "Acme iOS Renamed"
		updated, err := client.UpdateApp(ctx, projectID, appID, revenuecat.UpdateAppRequest{Name: &name})
		if err != nil {
			t.Fatalf("UpdateApp: %v", err)
		}
		if updated.Name != name {
			t.Errorf("update returned name %q, want %q", updated.Name, name)
		}
		if updated.Type != revenuecat.AppTypeAppStore {
			t.Errorf("update changed the type to %q; it should be untouched", updated.Type)
		}
	})

	var productIDs []string
	t.Run("products", func(t *testing.T) {
		for _, identifier := range []string{"com.acme.monthly", "com.acme.annual", "com.acme.lifetime"} {
			displayName := identifier
			product, err := client.CreateProduct(ctx, projectID, revenuecat.CreateProductRequest{
				StoreIdentifier: identifier,
				AppID:           appID,
				Type:            revenuecat.ProductTypeSubscription,
				DisplayName:     &displayName,
			})
			if err != nil {
				t.Fatalf("CreateProduct(%s): %v", identifier, err)
			}
			productIDs = append(productIDs, product.ID)
		}

		// Three products with a page size of two exercises the pagination walk.
		products, err := client.ListProducts(ctx, projectID)
		if err != nil {
			t.Fatalf("ListProducts: %v", err)
		}
		if len(products) != 3 {
			t.Fatalf("listed %d products, want all 3 across pages", len(products))
		}

		read, err := client.GetProduct(ctx, projectID, productIDs[0])
		if err != nil {
			t.Fatalf("GetProduct: %v", err)
		}
		if read.StoreIdentifier != "com.acme.monthly" {
			t.Errorf("store_identifier = %q, want com.acme.monthly", read.StoreIdentifier)
		}
		if read.AppID != appID {
			t.Errorf("app_id = %q, want %q", read.AppID, appID)
		}
	})

	var entitlementID string
	t.Run("entitlement", func(t *testing.T) {
		displayName := "Pro"
		entitlement, err := client.CreateEntitlement(ctx, projectID, revenuecat.CreateEntitlementRequest{
			LookupKey:   "pro",
			DisplayName: &displayName,
		})
		if err != nil {
			t.Fatalf("CreateEntitlement: %v", err)
		}
		entitlementID = entitlement.ID

		newName := "Pro Plus"
		updated, err := client.UpdateEntitlement(ctx, projectID, entitlementID, revenuecat.UpdateEntitlementRequest{
			DisplayName: &newName,
		})
		if err != nil {
			t.Fatalf("UpdateEntitlement: %v", err)
		}
		if updated.DisplayName != newName {
			t.Errorf("display_name = %q, want %q", updated.DisplayName, newName)
		}
		if updated.LookupKey != "pro" {
			t.Errorf("lookup_key = %q, want it unchanged", updated.LookupKey)
		}
	})

	t.Run("entitlement attachments", func(t *testing.T) {
		if err := client.AttachProductsToEntitlement(ctx, projectID, entitlementID, productIDs[:2]); err != nil {
			t.Fatalf("AttachProductsToEntitlement: %v", err)
		}

		attached, err := client.ListEntitlementProducts(ctx, projectID, entitlementID)
		if err != nil {
			t.Fatalf("ListEntitlementProducts: %v", err)
		}
		if len(attached) != 2 {
			t.Fatalf("listed %d attached products, want 2", len(attached))
		}

		if err := client.DetachProductsFromEntitlement(ctx, projectID, entitlementID, productIDs[:1]); err != nil {
			t.Fatalf("DetachProductsFromEntitlement: %v", err)
		}

		attached, err = client.ListEntitlementProducts(ctx, projectID, entitlementID)
		if err != nil {
			t.Fatalf("ListEntitlementProducts: %v", err)
		}
		if len(attached) != 1 || attached[0].ID != productIDs[1] {
			t.Errorf("after detaching one, got %d products %v, want just %s", len(attached), attached, productIDs[1])
		}
	})

	var offeringID, packageID string
	t.Run("offering and package", func(t *testing.T) {
		isCurrent := true
		offering, err := client.CreateOffering(ctx, projectID, revenuecat.CreateOfferingRequest{
			LookupKey: "default",
			IsCurrent: &isCurrent,
			Metadata:  map[string]string{"variant": "a"},
		})
		if err != nil {
			t.Fatalf("CreateOffering: %v", err)
		}
		offeringID = offering.ID

		if !offering.IsCurrent {
			t.Error("is_current did not round-trip as true")
		}
		if offering.Metadata["variant"] != "a" {
			t.Errorf("metadata = %v, want variant=a", offering.Metadata)
		}

		position := int64(1)
		pkg, err := client.CreatePackage(ctx, projectID, offeringID, revenuecat.CreatePackageRequest{
			LookupKey: "$rc_monthly",
			Position:  &position,
		})
		if err != nil {
			t.Fatalf("CreatePackage: %v", err)
		}
		packageID = pkg.ID

		if pkg.Position == nil || *pkg.Position != 1 {
			t.Errorf("position = %v, want 1", pkg.Position)
		}

		read, err := client.GetPackage(ctx, projectID, packageID)
		if err != nil {
			t.Fatalf("GetPackage: %v", err)
		}
		if read.OfferingID != offeringID {
			t.Errorf("offering_id = %q, want %q", read.OfferingID, offeringID)
		}

		packages, err := client.ListPackages(ctx, projectID, offeringID)
		if err != nil {
			t.Fatalf("ListPackages: %v", err)
		}
		if len(packages) != 1 {
			t.Errorf("listed %d packages, want 1", len(packages))
		}
	})

	t.Run("package attachments", func(t *testing.T) {
		err := client.AttachProductsToPackage(ctx, projectID, packageID, []revenuecat.PackageProduct{
			{ProductID: productIDs[0], EligibilityCriteria: "all"},
		})
		if err != nil {
			t.Fatalf("AttachProductsToPackage: %v", err)
		}

		attached, err := client.ListPackageProducts(ctx, projectID, packageID)
		if err != nil {
			t.Fatalf("ListPackageProducts: %v", err)
		}
		if len(attached) != 1 {
			t.Fatalf("listed %d attached products, want 1", len(attached))
		}
		if attached[0].EligibilityCriteria != "all" {
			t.Errorf("eligibility_criteria = %q, want all", attached[0].EligibilityCriteria)
		}
	})

	t.Run("deletes", func(t *testing.T) {
		if err := client.DeletePackage(ctx, projectID, packageID); err != nil {
			t.Fatalf("DeletePackage: %v", err)
		}
		if _, err := client.GetPackage(ctx, projectID, packageID); !revenuecat.IsNotFound(err) {
			t.Errorf("reading a deleted package gave %v, want a not-found error", err)
		}

		if err := client.DeleteOffering(ctx, projectID, offeringID); err != nil {
			t.Fatalf("DeleteOffering: %v", err)
		}
		if err := client.DeleteEntitlement(ctx, projectID, entitlementID); err != nil {
			t.Fatalf("DeleteEntitlement: %v", err)
		}
		for _, id := range productIDs {
			if err := client.DeleteProduct(ctx, projectID, id); err != nil {
				t.Fatalf("DeleteProduct(%s): %v", id, err)
			}
		}
		if err := client.DeleteApp(ctx, projectID, appID); err != nil {
			t.Fatalf("DeleteApp: %v", err)
		}

		remaining, err := client.ListProducts(ctx, projectID)
		if err != nil {
			t.Fatalf("ListProducts: %v", err)
		}
		if len(remaining) != 0 {
			t.Errorf("%d products survived deletion", len(remaining))
		}
	})
}

// TestClientRetriesAgainstMock exercises the retry path end to end: the mock
// rate-limits the first request, and the client must recover without the caller
// seeing an error.
func TestClientRetriesAgainstMock(t *testing.T) {
	ctx := context.Background()

	mock := mockrevenuecat.New(mockrevenuecat.Options{RateLimitRequests: 2})
	server := httptest.NewServer(mock)
	defer server.Close()

	projectID := mock.CreateProject("Acme")

	client, err := revenuecat.New("sk-test", server.URL+"/v2", revenuecat.WithMaxRetries(5))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	project, err := client.GetProject(ctx, projectID)
	if err != nil {
		t.Fatalf("GetProject did not recover from rate limiting: %v", err)
	}
	if project.Name != "Acme" {
		t.Errorf("name = %q, want Acme", project.Name)
	}
	if got := mock.RequestCount(); got != 3 {
		t.Errorf("mock served %d requests, want 3 (two rate-limited then one success)", got)
	}
}

// TestClientRejectedWithoutCredential confirms the mock's authentication
// tripwire is wired to the client's real behavior.
func TestClientRejectedWithoutCredential(t *testing.T) {
	mock := mockrevenuecat.New(mockrevenuecat.Options{APIKey: "sk-expected"})
	server := httptest.NewServer(mock)
	defer server.Close()

	projectID := mock.CreateProject("Acme")

	client, err := revenuecat.New("sk-wrong", server.URL+"/v2")
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	_, err = client.GetProject(context.Background(), projectID)
	if err == nil {
		t.Fatal("GetProject succeeded with the wrong key; want an error")
	}
	if got := revenuecat.StatusCode(err); got != 401 {
		t.Errorf("status = %d, want 401", got)
	}
}
