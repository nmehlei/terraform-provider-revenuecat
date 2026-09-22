package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/resourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/nmehlei/terraform-provider-revenuecat/internal/revenuecat"
)

var (
	_ resource.Resource                     = (*appResource)(nil)
	_ resource.ResourceWithConfigure        = (*appResource)(nil)
	_ resource.ResourceWithImportState      = (*appResource)(nil)
	_ resource.ResourceWithConfigValidators = (*appResource)(nil)
)

// NewAppResource returns the revenuecat_app resource.
func NewAppResource() resource.Resource {
	return &appResource{}
}

type appResource struct {
	client *revenuecat.Client
}

type appModel struct {
	ID          types.String `tfsdk:"id"`
	ProjectID   types.String `tfsdk:"project_id"`
	Name        types.String `tfsdk:"name"`
	Type        types.String `tfsdk:"type"`
	PackageName types.String `tfsdk:"package_name"`
	CreatedAt   types.Int64  `tfsdk:"created_at"`

	// CredentialsJSONWO is write-only: Terraform leaves it null in both plan
	// and state, so it is only ever readable from the configuration.
	CredentialsJSONWO        types.String `tfsdk:"play_service_account_credentials_json_wo"`
	CredentialsJSONWOVersion types.String `tfsdk:"play_service_account_credentials_json_wo_version"`
	CredentialsConfigured    types.Bool   `tfsdk:"play_service_account_credentials_configured"`
}

func (r *appResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_app"
}

func (r *appResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a store app within a RevenueCat project.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "RevenueCat identifier of the app.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"project_id": schema.StringAttribute{
				MarkdownDescription: "Identifier of the project the app belongs to. Changing this forces a new app.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Display name of the app.",
				Required:            true,
			},
			"type": schema.StringAttribute{
				MarkdownDescription: fmt.Sprintf(
					"Store the app belongs to. One of `%s` — RevenueCat's API nests type-specific "+
						"configuration under a key named after the type, and this provider only "+
						"implements that nesting for `play_store` so far. Changing this forces a new app.",
					joinBackticked(revenuecat.ImplementedAppTypes),
				),
				Required: true,
				Validators: []validator.String{
					stringvalidator.OneOf(revenuecat.ImplementedAppTypes...),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"package_name": schema.StringAttribute{
				MarkdownDescription: "Play Store package identifier (e.g. `com.example.app`). Required " +
					"because `type` currently only accepts `play_store`. Changing this forces a new app.",
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"created_at": schema.Int64Attribute{
				MarkdownDescription: "Creation time of the app, in milliseconds since the Unix epoch.",
				Computed:            true,
				PlanModifiers: []planmodifier.Int64{
					int64UseStateForUnknown(),
				},
			},
			"play_service_account_credentials_json_wo": schema.StringAttribute{
				MarkdownDescription: "Contents of the Google Cloud service account key file RevenueCat uses to " +
					"verify Play purchases server-side. Write-only: Terraform sends it but never writes it to " +
					"state or to a plan file, so pair it with an `ephemeral` source (for example " +
					"`ephemeral.azurerm_key_vault_secret`) to keep the key out of every artifact.\n\n" +
					"Without this credential RevenueCat cannot acknowledge a Play purchase, and Google " +
					"auto-refunds every unacknowledged purchase — so an app left without it appears to work " +
					"until purchases start silently reversing.\n\n" +
					"Requires Terraform 1.11 or later, and requires " +
					"`play_service_account_credentials_json_wo_version` to be set alongside it.",
				Optional:  true,
				WriteOnly: true,
				Sensitive: true,
			},
			"play_service_account_credentials_json_wo_version": schema.StringAttribute{
				MarkdownDescription: "Arbitrary version marker for the write-only credential. Terraform cannot " +
					"detect a change to a value it does not store, so changing this is what tells the provider " +
					"to send the credential again — bump it whenever the key is rotated. Any string works; a " +
					"date or an incrementing number is conventional.",
				Optional: true,
			},
			"play_service_account_credentials_configured": schema.BoolAttribute{
				MarkdownDescription: "Whether RevenueCat holds Play service account credentials for this app. " +
					"Reported by the API, and the only observable signal that the credential is present — a " +
					"`false` here is the difference between purchases being verified and being refunded.",
				Computed: true,
			},
		},
	}
}

// ConfigValidators rejects a credential with no version. Terraform cannot diff
// a write-only value, so such a configuration would accept a rotated key and
// silently never send it — the failure would surface as refunded purchases
// rather than as an error.
func (r *appResource) ConfigValidators(_ context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		resourcevalidator.RequiredTogether(
			path.MatchRoot("play_service_account_credentials_json_wo"),
			path.MatchRoot("play_service_account_credentials_json_wo_version"),
		),
	}
}

func (r *appResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureResourceClient(req, resp)
}

func (r *appResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan, config appModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	// Write-only attributes are null in the plan by design, so the credential
	// has to be read from the configuration.
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	app, err := r.client.CreateApp(ctx, plan.ProjectID.ValueString(), revenuecat.CreateAppRequest{
		Name: plan.Name.ValueString(),
		Type: plan.Type.ValueString(),
		PlayStore: &revenuecat.PlayStoreConfig{
			PackageName:                   plan.PackageName.ValueString(),
			ServiceAccountCredentialsJSON: config.CredentialsJSONWO.ValueString(),
		},
	})
	if err != nil {
		resp.Diagnostics.AddError("Unable to create the RevenueCat app", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, appToModel(plan.ProjectID.ValueString(), app, plan.CredentialsJSONWOVersion))...)
}

func (r *appResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state appModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	app, err := r.client.GetApp(ctx, state.ProjectID.ValueString(), state.ID.ValueString())
	if err != nil {
		if revenuecat.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read the RevenueCat app", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, appToModel(state.ProjectID.ValueString(), app, state.CredentialsJSONWOVersion))...)
}

func (r *appResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state, config appModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	name := plan.Name.ValueString()
	update := revenuecat.UpdateAppRequest{Name: &name}

	// Resend the credential only when its version moved. Sending it on every
	// update would push the key over the wire on an unrelated rename; never
	// sending it would make rotation impossible, because Terraform cannot see
	// that a write-only value changed.
	if !plan.CredentialsJSONWOVersion.Equal(state.CredentialsJSONWOVersion) && !config.CredentialsJSONWO.IsNull() {
		update.PlayStore = &revenuecat.PlayStoreConfig{
			ServiceAccountCredentialsJSON: config.CredentialsJSONWO.ValueString(),
		}
	}

	app, err := r.client.UpdateApp(ctx, state.ProjectID.ValueString(), state.ID.ValueString(), update)
	if err != nil {
		resp.Diagnostics.AddError("Unable to update the RevenueCat app", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, appToModel(state.ProjectID.ValueString(), app, plan.CredentialsJSONWOVersion))...)
}

func (r *appResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state appModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteApp(ctx, state.ProjectID.ValueString(), state.ID.ValueString())
	if err != nil && !revenuecat.IsNotFound(err) {
		resp.Diagnostics.AddError("Unable to delete the RevenueCat app", err.Error())
	}
}

func (r *appResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts, err := parseImportID(req.ID, "project_id", "app_id")
	if err != nil {
		importIDError(&resp.Diagnostics, err)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[1])...)
}

// appToModel maps an API app onto the resource model. The project ID comes from
// configuration because the API does not always echo it back, and so does the
// credential version: it is a marker the practitioner owns, which RevenueCat
// neither stores nor returns.
//
// The write-only credential itself is always left null. Terraform requires
// that, and it is the property the whole attribute exists for.
func appToModel(projectID string, app *revenuecat.App, credentialsVersion types.String) appModel {
	if app.ProjectID != "" {
		projectID = app.ProjectID
	}
	packageName := types.StringNull()
	credentialsConfigured := types.BoolValue(false)
	if app.PlayStore != nil {
		packageName = types.StringValue(app.PlayStore.PackageName)
		credentialsConfigured = types.BoolValue(app.PlayStore.ServiceAccountCredentialsConfigured)
	}
	return appModel{
		ID:          types.StringValue(app.ID),
		ProjectID:   types.StringValue(projectID),
		Name:        types.StringValue(app.Name),
		Type:        types.StringValue(app.Type),
		PackageName: packageName,
		CreatedAt:   types.Int64Value(app.CreatedAt),

		CredentialsJSONWO:        types.StringNull(),
		CredentialsJSONWOVersion: credentialsVersion,
		CredentialsConfigured:    credentialsConfigured,
	}
}
