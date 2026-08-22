package provider

import (
	"context"
	"fmt"

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
	_ resource.Resource                = (*appResource)(nil)
	_ resource.ResourceWithConfigure   = (*appResource)(nil)
	_ resource.ResourceWithImportState = (*appResource)(nil)
)

// NewAppResource returns the revenuecat_app resource.
func NewAppResource() resource.Resource {
	return &appResource{}
}

type appResource struct {
	client *revenuecat.Client
}

type appModel struct {
	ID        types.String `tfsdk:"id"`
	ProjectID types.String `tfsdk:"project_id"`
	Name      types.String `tfsdk:"name"`
	Type      types.String `tfsdk:"type"`
	CreatedAt types.Int64  `tfsdk:"created_at"`
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
					"Store the app belongs to. One of `%s`. Changing this forces a new app.",
					joinBackticked(revenuecat.AppTypes),
				),
				Required: true,
				Validators: []validator.String{
					stringvalidator.OneOf(revenuecat.AppTypes...),
				},
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
		},
	}
}

func (r *appResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureResourceClient(req, resp)
}

func (r *appResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan appModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	app, err := r.client.CreateApp(ctx, plan.ProjectID.ValueString(), revenuecat.CreateAppRequest{
		Name: plan.Name.ValueString(),
		Type: plan.Type.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Unable to create the RevenueCat app", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, appToModel(plan.ProjectID.ValueString(), app))...)
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

	resp.Diagnostics.Append(resp.State.Set(ctx, appToModel(state.ProjectID.ValueString(), app))...)
}

func (r *appResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state appModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	name := plan.Name.ValueString()
	app, err := r.client.UpdateApp(ctx, state.ProjectID.ValueString(), state.ID.ValueString(), revenuecat.UpdateAppRequest{
		Name: &name,
	})
	if err != nil {
		resp.Diagnostics.AddError("Unable to update the RevenueCat app", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, appToModel(state.ProjectID.ValueString(), app))...)
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
// configuration because the API does not always echo it back.
func appToModel(projectID string, app *revenuecat.App) appModel {
	if app.ProjectID != "" {
		projectID = app.ProjectID
	}
	return appModel{
		ID:        types.StringValue(app.ID),
		ProjectID: types.StringValue(projectID),
		Name:      types.StringValue(app.Name),
		Type:      types.StringValue(app.Type),
		CreatedAt: types.Int64Value(app.CreatedAt),
	}
}
