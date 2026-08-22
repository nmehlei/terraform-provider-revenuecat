package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/nmehlei/terraform-provider-revenuecat/internal/revenuecat"
)

var (
	_ resource.Resource                = (*entitlementResource)(nil)
	_ resource.ResourceWithConfigure   = (*entitlementResource)(nil)
	_ resource.ResourceWithImportState = (*entitlementResource)(nil)
)

// NewEntitlementResource returns the revenuecat_entitlement resource.
func NewEntitlementResource() resource.Resource {
	return &entitlementResource{}
}

type entitlementResource struct {
	client *revenuecat.Client
}

type entitlementModel struct {
	ID          types.String `tfsdk:"id"`
	ProjectID   types.String `tfsdk:"project_id"`
	LookupKey   types.String `tfsdk:"lookup_key"`
	DisplayName types.String `tfsdk:"display_name"`
	CreatedAt   types.Int64  `tfsdk:"created_at"`
}

func (r *entitlementResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_entitlement"
}

func (r *entitlementResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an entitlement within a RevenueCat project. An entitlement is " +
			"the level of access a customer receives after purchasing an attached product.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "RevenueCat identifier of the entitlement.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"project_id": schema.StringAttribute{
				MarkdownDescription: "Identifier of the project the entitlement belongs to. Changing this forces a new entitlement.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"lookup_key": schema.StringAttribute{
				MarkdownDescription: "Key used to reference the entitlement from the RevenueCat SDKs, for example `pro`.",
				Required:            true,
			},
			"display_name": schema.StringAttribute{
				MarkdownDescription: "Human-readable name of the entitlement, shown in the RevenueCat dashboard.",
				Optional:            true,
			},
			"created_at": schema.Int64Attribute{
				MarkdownDescription: "Creation time of the entitlement, in milliseconds since the Unix epoch.",
				Computed:            true,
				PlanModifiers: []planmodifier.Int64{
					int64UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *entitlementResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureResourceClient(req, resp)
}

func (r *entitlementResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan entitlementModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	entitlement, err := r.client.CreateEntitlement(ctx, plan.ProjectID.ValueString(), revenuecat.CreateEntitlementRequest{
		LookupKey:   plan.LookupKey.ValueString(),
		DisplayName: optionalString(plan.DisplayName),
	})
	if err != nil {
		resp.Diagnostics.AddError("Unable to create the RevenueCat entitlement", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, entitlementToModel(plan.ProjectID.ValueString(), entitlement))...)
}

func (r *entitlementResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state entitlementModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	entitlement, err := r.client.GetEntitlement(ctx, state.ProjectID.ValueString(), state.ID.ValueString())
	if err != nil {
		if revenuecat.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read the RevenueCat entitlement", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, entitlementToModel(state.ProjectID.ValueString(), entitlement))...)
}

func (r *entitlementResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state entitlementModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	lookupKey := plan.LookupKey.ValueString()
	entitlement, err := r.client.UpdateEntitlement(ctx, state.ProjectID.ValueString(), state.ID.ValueString(), revenuecat.UpdateEntitlementRequest{
		LookupKey:   &lookupKey,
		DisplayName: optionalString(plan.DisplayName),
	})
	if err != nil {
		resp.Diagnostics.AddError("Unable to update the RevenueCat entitlement", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, entitlementToModel(state.ProjectID.ValueString(), entitlement))...)
}

func (r *entitlementResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state entitlementModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteEntitlement(ctx, state.ProjectID.ValueString(), state.ID.ValueString())
	if err != nil && !revenuecat.IsNotFound(err) {
		resp.Diagnostics.AddError("Unable to delete the RevenueCat entitlement", err.Error())
	}
}

func (r *entitlementResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts, err := parseImportID(req.ID, "project_id", "entitlement_id")
	if err != nil {
		importIDError(&resp.Diagnostics, err)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[1])...)
}

func entitlementToModel(projectID string, entitlement *revenuecat.Entitlement) entitlementModel {
	if entitlement.ProjectID != "" {
		projectID = entitlement.ProjectID
	}
	return entitlementModel{
		ID:          types.StringValue(entitlement.ID),
		ProjectID:   types.StringValue(projectID),
		LookupKey:   types.StringValue(entitlement.LookupKey),
		DisplayName: stringOrNull(entitlement.DisplayName),
		CreatedAt:   types.Int64Value(entitlement.CreatedAt),
	}
}
