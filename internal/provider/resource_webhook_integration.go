package provider

import (
	"context"
	"fmt"
	"sort"

	"github.com/hashicorp/terraform-plugin-framework-validators/resourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
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
	_ resource.Resource                     = (*webhookIntegrationResource)(nil)
	_ resource.ResourceWithConfigure        = (*webhookIntegrationResource)(nil)
	_ resource.ResourceWithImportState      = (*webhookIntegrationResource)(nil)
	_ resource.ResourceWithConfigValidators = (*webhookIntegrationResource)(nil)
)

// NewWebhookIntegrationResource returns the revenuecat_webhook_integration resource.
func NewWebhookIntegrationResource() resource.Resource {
	return &webhookIntegrationResource{}
}

type webhookIntegrationResource struct {
	client *revenuecat.Client
}

type webhookIntegrationModel struct {
	ID          types.String `tfsdk:"id"`
	ProjectID   types.String `tfsdk:"project_id"`
	Name        types.String `tfsdk:"name"`
	URL         types.String `tfsdk:"url"`
	Environment types.String `tfsdk:"environment"`
	EventTypes  types.Set    `tfsdk:"event_types"`
	AppID       types.String `tfsdk:"app_id"`
	CreatedAt   types.Int64  `tfsdk:"created_at"`

	// HeaderWO is write-only: null in plan and state, readable only from the
	// configuration.
	HeaderWO        types.String `tfsdk:"authorization_header_wo"`
	HeaderWOVersion types.String `tfsdk:"authorization_header_wo_version"`
}

func (r *webhookIntegrationResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_webhook_integration"
}

func (r *webhookIntegrationResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a webhook RevenueCat calls with a project's customer events.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "RevenueCat identifier of the webhook integration.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"project_id": schema.StringAttribute{
				MarkdownDescription: "Identifier of the project the webhook belongs to. Changing this forces a new webhook.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Display name of the webhook.",
				Required:            true,
				Validators:          []validator.String{stringvalidator.LengthBetween(1, 1500)},
			},
			"url": schema.StringAttribute{
				MarkdownDescription: "HTTPS URL RevenueCat sends events to.",
				Required:            true,
				Validators:          []validator.String{stringvalidator.LengthBetween(1, 5000)},
			},
			"environment": schema.StringAttribute{
				MarkdownDescription: fmt.Sprintf("Only send events from this environment, one of `%s`. "+
					"Unset sends events from both.", joinBackticked(revenuecat.WebhookEnvironments)),
				Optional:   true,
				Validators: []validator.String{stringvalidator.OneOf(revenuecat.WebhookEnvironments...)},
			},
			"event_types": schema.SetAttribute{
				MarkdownDescription: "Only send these event types (for example `initial_purchase`, `renewal`, " +
					"`expiration`). Unset sends every event type.",
				ElementType: types.StringType,
				Optional:    true,
				Validators:  []validator.Set{setvalidator.SizeAtLeast(1)},
			},
			"app_id": schema.StringAttribute{
				MarkdownDescription: "Only send events for this app. Unset sends events for every app in the project.",
				Optional:            true,
			},
			"created_at": schema.Int64Attribute{
				MarkdownDescription: "Creation time of the webhook, in milliseconds since the Unix epoch.",
				Computed:            true,
				PlanModifiers: []planmodifier.Int64{
					int64UseStateForUnknown(),
				},
			},
			"authorization_header_wo": schema.StringAttribute{
				MarkdownDescription: "Value RevenueCat sends in the `Authorization` header of every webhook " +
					"request, so the receiver can reject calls that did not come from RevenueCat. Write-only: " +
					"Terraform sends it but never writes it to state or to a plan file, and RevenueCat never " +
					"returns it.\n\nRequires Terraform 1.11 or later, and requires " +
					"`authorization_header_wo_version` to be set alongside it.",
				Optional:  true,
				WriteOnly: true,
				Sensitive: true,
			},
			"authorization_header_wo_version": schema.StringAttribute{
				MarkdownDescription: "Arbitrary version marker for the write-only header. Terraform cannot detect " +
					"a change to a value it does not store, so changing this is what sends the header again — " +
					"bump it whenever the header is rotated.",
				Optional: true,
			},
		},
	}
}

// ConfigValidators rejects a header with no version: such a configuration
// would accept a rotated header and silently never send it.
func (r *webhookIntegrationResource) ConfigValidators(_ context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		resourcevalidator.RequiredTogether(
			path.MatchRoot("authorization_header_wo"),
			path.MatchRoot("authorization_header_wo_version"),
		),
	}
}

func (r *webhookIntegrationResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureResourceClient(req, resp)
}

func (r *webhookIntegrationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan, config webhookIntegrationModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	eventTypes, diags := eventTypesFromSet(ctx, plan.EventTypes)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	hook, err := r.client.CreateWebhookIntegration(ctx, plan.ProjectID.ValueString(), revenuecat.CreateWebhookIntegrationRequest{
		Name:                plan.Name.ValueString(),
		URL:                 plan.URL.ValueString(),
		AuthorizationHeader: config.HeaderWO.ValueStringPointer(),
		Environment:         plan.Environment.ValueStringPointer(),
		EventTypes:          eventTypes,
		AppID:               plan.AppID.ValueStringPointer(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Unable to create the RevenueCat webhook integration", err.Error())
		return
	}

	model, diags := webhookToModel(plan.ProjectID.ValueString(), hook, plan.HeaderWOVersion)
	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}

func (r *webhookIntegrationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state webhookIntegrationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	hook, err := r.client.GetWebhookIntegration(ctx, state.ProjectID.ValueString(), state.ID.ValueString())
	if err != nil {
		if revenuecat.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read the RevenueCat webhook integration", err.Error())
		return
	}

	model, diags := webhookToModel(state.ProjectID.ValueString(), hook, state.HeaderWOVersion)
	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}

func (r *webhookIntegrationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state, config webhookIntegrationModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	eventTypes, diags := eventTypesFromSet(ctx, plan.EventTypes)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	update := revenuecat.UpdateWebhookIntegrationRequest{
		Name:        plan.Name.ValueString(),
		URL:         plan.URL.ValueString(),
		Environment: plan.Environment.ValueStringPointer(),
		EventTypes:  eventTypes,
		AppID:       plan.AppID.ValueStringPointer(),
	}
	// Resend the header only when its version moved, including a move to "no
	// header", which clears it.
	if !plan.HeaderWOVersion.Equal(state.HeaderWOVersion) {
		update.RotateAuthorizationHeader = true
		update.AuthorizationHeader = config.HeaderWO.ValueStringPointer()
	}

	hook, err := r.client.UpdateWebhookIntegration(ctx, state.ProjectID.ValueString(), state.ID.ValueString(), update)
	if err != nil {
		resp.Diagnostics.AddError("Unable to update the RevenueCat webhook integration", err.Error())
		return
	}

	model, diags := webhookToModel(state.ProjectID.ValueString(), hook, plan.HeaderWOVersion)
	resp.Diagnostics.Append(diags...)
	resp.Diagnostics.Append(resp.State.Set(ctx, model)...)
}

func (r *webhookIntegrationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state webhookIntegrationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteWebhookIntegration(ctx, state.ProjectID.ValueString(), state.ID.ValueString())
	if err != nil && !revenuecat.IsNotFound(err) {
		resp.Diagnostics.AddError("Unable to delete the RevenueCat webhook integration", err.Error())
	}
}

func (r *webhookIntegrationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts, err := parseImportID(req.ID, "project_id", "webhook_integration_id")
	if err != nil {
		importIDError(&resp.Diagnostics, err)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[1])...)
}

// webhookToModel maps an API webhook onto the resource model. An empty event
// type list and a missing one both mean "every event", so both map to null and
// cannot show up as drift against an unset attribute.
func webhookToModel(projectID string, hook *revenuecat.WebhookIntegration, headerVersion types.String) (webhookIntegrationModel, diag.Diagnostics) {
	if hook.ProjectID != "" {
		projectID = hook.ProjectID
	}

	eventTypes := types.SetNull(types.StringType)
	var diags diag.Diagnostics
	if len(hook.EventTypes) > 0 {
		sorted := append([]string(nil), hook.EventTypes...)
		sort.Strings(sorted)
		elements := make([]attr.Value, 0, len(sorted))
		for _, eventType := range sorted {
			elements = append(elements, types.StringValue(eventType))
		}
		eventTypes, diags = types.SetValue(types.StringType, elements)
	}

	return webhookIntegrationModel{
		ID:          types.StringValue(hook.ID),
		ProjectID:   types.StringValue(projectID),
		Name:        types.StringValue(hook.Name),
		URL:         types.StringValue(hook.URL),
		Environment: types.StringPointerValue(nonEmpty(hook.Environment)),
		EventTypes:  eventTypes,
		AppID:       types.StringPointerValue(nonEmpty(hook.AppID)),
		CreatedAt:   types.Int64Value(hook.CreatedAt),

		HeaderWO:        types.StringNull(),
		HeaderWOVersion: headerVersion,
	}, diags
}

// nonEmpty treats an empty string like null: the API documents "" as a
// possible environment value meaning "none".
func nonEmpty(value *string) *string {
	if value == nil || *value == "" {
		return nil
	}
	return value
}

func eventTypesFromSet(ctx context.Context, set types.Set) ([]string, diag.Diagnostics) {
	if set.IsNull() || set.IsUnknown() {
		return nil, nil
	}
	var eventTypes []string
	diags := set.ElementsAs(ctx, &eventTypes, false)
	sort.Strings(eventTypes)
	return eventTypes, diags
}
