package revenuecat

// App type values RevenueCat recognizes for a store app.
const (
	AppTypeAppStore    = "app_store"
	AppTypeMacAppStore = "mac_app_store"
	AppTypePlayStore   = "play_store"
	AppTypeAmazon      = "amazon"
	AppTypeStripe      = "stripe"
	AppTypeRCBilling   = "rc_billing"
	AppTypeRoku        = "roku"
	AppTypePaddle      = "paddle"
)

// AppTypes lists every accepted value of an app's type attribute.
var AppTypes = []string{
	AppTypeAppStore,
	AppTypeMacAppStore,
	AppTypePlayStore,
	AppTypeAmazon,
	AppTypeStripe,
	AppTypeRCBilling,
	AppTypeRoku,
	AppTypePaddle,
}

// Product type values RevenueCat recognizes.
const (
	ProductTypeSubscription = "subscription"
	ProductTypeOneTime      = "one_time"
)

// ProductTypes lists every accepted value of a product's type attribute.
var ProductTypes = []string{
	ProductTypeSubscription,
	ProductTypeOneTime,
}

// Project is a RevenueCat project.
type Project struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	CreatedAt int64  `json:"created_at"`
}

// App is a store app belonging to a project.
type App struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	ProjectID string `json:"project_id"`
	CreatedAt int64  `json:"created_at"`
}

// CreateAppRequest is the payload for creating an app.
type CreateAppRequest struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// UpdateAppRequest is the payload for updating an app. Only a name change is
// supported; type and project are part of the app's identity.
type UpdateAppRequest struct {
	Name *string `json:"name,omitempty"`
}

// Product is a store product belonging to an app within a project.
type Product struct {
	ID              string `json:"id"`
	StoreIdentifier string `json:"store_identifier"`
	Type            string `json:"type"`
	DisplayName     string `json:"display_name"`
	AppID           string `json:"app_id"`
	CreatedAt       int64  `json:"created_at"`
}

// CreateProductRequest is the payload for creating a product.
type CreateProductRequest struct {
	StoreIdentifier string  `json:"store_identifier"`
	AppID           string  `json:"app_id"`
	Type            string  `json:"type"`
	DisplayName     *string `json:"display_name,omitempty"`
}

// Entitlement is an entitlement belonging to a project.
type Entitlement struct {
	ID          string `json:"id"`
	LookupKey   string `json:"lookup_key"`
	DisplayName string `json:"display_name"`
	ProjectID   string `json:"project_id"`
	CreatedAt   int64  `json:"created_at"`
}

// CreateEntitlementRequest is the payload for creating an entitlement.
type CreateEntitlementRequest struct {
	LookupKey   string  `json:"lookup_key"`
	DisplayName *string `json:"display_name,omitempty"`
}

// UpdateEntitlementRequest is the payload for updating an entitlement.
type UpdateEntitlementRequest struct {
	LookupKey   *string `json:"lookup_key,omitempty"`
	DisplayName *string `json:"display_name,omitempty"`
}

// Offering is an offering belonging to a project.
type Offering struct {
	ID          string            `json:"id"`
	LookupKey   string            `json:"lookup_key"`
	DisplayName string            `json:"display_name"`
	IsCurrent   bool              `json:"is_current"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	ProjectID   string            `json:"project_id"`
	CreatedAt   int64             `json:"created_at"`
}

// CreateOfferingRequest is the payload for creating an offering.
type CreateOfferingRequest struct {
	LookupKey   string            `json:"lookup_key"`
	DisplayName *string           `json:"display_name,omitempty"`
	IsCurrent   *bool             `json:"is_current,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// UpdateOfferingRequest is the payload for updating an offering.
type UpdateOfferingRequest struct {
	LookupKey   *string           `json:"lookup_key,omitempty"`
	DisplayName *string           `json:"display_name,omitempty"`
	IsCurrent   *bool             `json:"is_current,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// Package is a package belonging to an offering.
type Package struct {
	ID          string `json:"id"`
	LookupKey   string `json:"lookup_key"`
	DisplayName string `json:"display_name"`
	Position    *int64 `json:"position,omitempty"`
	OfferingID  string `json:"offering_id"`
	CreatedAt   int64  `json:"created_at"`
}

// CreatePackageRequest is the payload for creating a package.
type CreatePackageRequest struct {
	LookupKey   string  `json:"lookup_key"`
	DisplayName *string `json:"display_name,omitempty"`
	Position    *int64  `json:"position,omitempty"`
}

// UpdatePackageRequest is the payload for updating a package.
type UpdatePackageRequest struct {
	LookupKey   *string `json:"lookup_key,omitempty"`
	DisplayName *string `json:"display_name,omitempty"`
	Position    *int64  `json:"position,omitempty"`
}

// PackageProduct associates a product with a package under an eligibility
// criteria.
type PackageProduct struct {
	ProductID           string `json:"product_id"`
	EligibilityCriteria string `json:"eligibility_criteria,omitempty"`
}

// packageProductEntry is the shape returned when listing a package's products:
// the product object nested alongside its eligibility criteria.
type packageProductEntry struct {
	EligibilityCriteria string   `json:"eligibility_criteria"`
	Product             *Product `json:"product"`
	ProductID           string   `json:"product_id"`
}

// attachProductsRequest is the payload for attaching products to an
// entitlement.
type attachProductsRequest struct {
	ProductIDs []string `json:"product_ids"`
}

// attachPackageProductsRequest is the payload for attaching products to a
// package, which carries per-product eligibility criteria.
type attachPackageProductsRequest struct {
	Products []PackageProduct `json:"products"`
}
