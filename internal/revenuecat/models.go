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

// AppTypes lists every value RevenueCat itself accepts for an app's type.
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

// ImplementedAppTypes lists the subset of AppTypes this provider can actually
// create — the ones whose nested type-specific config object it knows how to
// send. Narrower than AppTypes on purpose: sending "type" alone without that
// object is a guaranteed 400 (see CreateAppRequest), so allowing a value here
// before its config struct exists would just move the failure from plan time
// to apply time.
var ImplementedAppTypes = []string{
	AppTypePlayStore,
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

// PlayStoreConfig is the Play Store-specific configuration nested under an
// app's "play_store" key, both on create and in the API's own response.
type PlayStoreConfig struct {
	PackageName string `json:"package_name"`
}

// App is a store app belonging to a project. The API nests store-specific
// configuration under a key named after the store type (e.g. "play_store");
// this provider currently only implements that nesting for Play Store apps
// (see AppTypes vs. ImplementedAppTypes) — every other type's config object
// (app_store, amazon, stripe, rc_billing, roku, paddle) still needs adding.
type App struct {
	ID        string           `json:"id"`
	Name      string           `json:"name"`
	Type      string           `json:"type"`
	ProjectID string           `json:"project_id"`
	CreatedAt int64            `json:"created_at"`
	PlayStore *PlayStoreConfig `json:"play_store,omitempty"`
}

// CreateAppRequest is the payload for creating an app. The API rejects the
// request with "'play_store' is a required property" unless the type-specific
// object is present alongside "type" — a discriminated union, not a flat enum.
type CreateAppRequest struct {
	Name      string           `json:"name"`
	Type      string           `json:"type"`
	PlayStore *PlayStoreConfig `json:"play_store,omitempty"`
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
// display_name is required by the API here (unlike most other resources'
// display_name, which is optional) — omitting it is a 400.
type CreateEntitlementRequest struct {
	LookupKey   string `json:"lookup_key"`
	DisplayName string `json:"display_name"`
}

// UpdateEntitlementRequest is the payload for updating an entitlement. The
// API accepts only display_name here — lookup_key is part of the
// entitlement's identity and cannot be changed after creation; sending it
// 400s as an unexpected property, so it has no field here at all.
type UpdateEntitlementRequest struct {
	DisplayName string `json:"display_name"`
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

// CreateOfferingRequest is the payload for creating an offering. The API
// rejects "is_current" here ("Additional properties are not allowed") — a
// freshly created offering always starts non-current; making it current is
// only accepted on update (UpdateOfferingRequest), never on create.
type CreateOfferingRequest struct {
	LookupKey   string            `json:"lookup_key"`
	DisplayName *string           `json:"display_name,omitempty"`
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

// CreatePackageRequest is the payload for creating a package. display_name
// is required by the API here; position stays optional on create (unlike on
// update, see UpdatePackageRequest).
type CreatePackageRequest struct {
	LookupKey   string `json:"lookup_key"`
	DisplayName string `json:"display_name"`
	Position    *int64 `json:"position,omitempty"`
}

// UpdatePackageRequest is the payload for updating a package. Unlike create,
// the API requires both display_name and position here, and rejects
// lookup_key entirely — part of the package's identity, not updatable — so
// it has no field here at all.
type UpdatePackageRequest struct {
	DisplayName string `json:"display_name"`
	Position    int64  `json:"position"`
}

// Eligibility criteria values RevenueCat recognizes for a package product
// attachment. Unlike EligibilityCriteria's json tag suggests, the API
// requires this field on every attach_products entry — it is never actually
// optional.
const (
	EligibilityCriteriaAll           = "all"
	EligibilityCriteriaGoogleSDKLt6  = "google_sdk_lt_6"
	EligibilityCriteriaGoogleSDKGte6 = "google_sdk_ge_6"
)

// EligibilityCriteriaValues lists every accepted eligibility_criteria value.
var EligibilityCriteriaValues = []string{
	EligibilityCriteriaAll,
	EligibilityCriteriaGoogleSDKLt6,
	EligibilityCriteriaGoogleSDKGte6,
}

// PackageProduct associates a product with a package under an eligibility
// criteria. EligibilityCriteria is required by the API on every attach —
// omitting it is a 400, despite the json tag below looking optional; the
// omitempty exists only so a zero-value PackageProduct doesn't marshal an
// empty string when a caller has a real bug, not to make the field elidable.
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
