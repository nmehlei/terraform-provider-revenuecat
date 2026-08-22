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
	_ resource.Resource                = (*packageResource)(nil)
	_ resource.ResourceWithConfigure   = (*packageResource)(nil)
	_ resource.ResourceWithImportState = (*packageResource)(nil)
)

// NewPackageResource returns the revenuecat_package resource.
func NewPackageResource() resource.Resource {
	return &packageResource{}
}

type packageResource struct {
	client *revenuecat.Client
}

type packageModel struct {
	ID          types.String `tfsdk:"id"`
	ProjectID   types.String `tfsdk:"project_id"`
	OfferingID  types.String `tfsdk:"offering_id"`
	LookupKey   types.String `tfsdk:"lookup_key"`
	DisplayName types.String `tfsdk:"display_name"`
	Position    types.Int64  `tfsdk:"position"`
	CreatedAt   types.Int64  `tfsdk:"created_at"`
}

func (r *packageResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_package"
}

func (r *packageResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a package within a RevenueCat offering. A package groups the " +
			"equivalent products across stores, for example the monthly subscription on both the " +
			"App Store and the Play Store.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "RevenueCat identifier of the package.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"project_id": schema.StringAttribute{
				MarkdownDescription: "Identifier of the project the package belongs to. Changing this forces a new package.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"offering_id": schema.StringAttribute{
				MarkdownDescription: "Identifier of the offering the package belongs to. Changing this forces a new package.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"lookup_key": schema.StringAttribute{
				MarkdownDescription: "Key used to reference the package from the RevenueCat SDKs, for example `$rc_monthly`.",
				Required:            true,
			},
			"display_name": schema.StringAttribute{
				MarkdownDescription: "Human-readable name of the package, shown in the RevenueCat dashboard.",
				Optional:            true,
			},
			"position": schema.Int64Attribute{
				MarkdownDescription: "Position of the package within its offering, used to order packages on a paywall.",
				Optional:            true,
			},
			"created_at": schema.Int64Attribute{
				MarkdownDescription: "Creation time of the package, in milliseconds since the Unix epoch.",
				Computed:            true,
				PlanModifiers: []planmodifier.Int64{
					int64UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *packageResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureResourceClient(req, resp)
}

func (r *packageResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan packageModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	pkg, err := r.client.CreatePackage(ctx, plan.ProjectID.ValueString(), plan.OfferingID.ValueString(), revenuecat.CreatePackageRequest{
		LookupKey:   plan.LookupKey.ValueString(),
		DisplayName: optionalString(plan.DisplayName),
		Position:    optionalInt64(plan.Position),
	})
	if err != nil {
		resp.Diagnostics.AddError("Unable to create the RevenueCat package", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, packageToModel(plan.ProjectID.ValueString(), plan.OfferingID.ValueString(), pkg))...)
}

func (r *packageResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state packageModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	pkg, err := r.client.GetPackage(ctx, state.ProjectID.ValueString(), state.ID.ValueString())
	if err != nil {
		if revenuecat.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read the RevenueCat package", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, packageToModel(state.ProjectID.ValueString(), state.OfferingID.ValueString(), pkg))...)
}

func (r *packageResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state packageModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	lookupKey := plan.LookupKey.ValueString()
	pkg, err := r.client.UpdatePackage(ctx, state.ProjectID.ValueString(), state.ID.ValueString(), revenuecat.UpdatePackageRequest{
		LookupKey:   &lookupKey,
		DisplayName: optionalString(plan.DisplayName),
		Position:    optionalInt64(plan.Position),
	})
	if err != nil {
		resp.Diagnostics.AddError("Unable to update the RevenueCat package", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, packageToModel(state.ProjectID.ValueString(), state.OfferingID.ValueString(), pkg))...)
}

func (r *packageResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state packageModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeletePackage(ctx, state.ProjectID.ValueString(), state.ID.ValueString())
	if err != nil && !revenuecat.IsNotFound(err) {
		resp.Diagnostics.AddError("Unable to delete the RevenueCat package", err.Error())
	}
}

func (r *packageResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts, err := parseImportID(req.ID, "project_id", "offering_id", "package_id")
	if err != nil {
		importIDError(&resp.Diagnostics, err)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("offering_id"), parts[1])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[2])...)
}

func packageToModel(projectID, offeringID string, pkg *revenuecat.Package) packageModel {
	if pkg.OfferingID != "" {
		offeringID = pkg.OfferingID
	}
	return packageModel{
		ID:          types.StringValue(pkg.ID),
		ProjectID:   types.StringValue(projectID),
		OfferingID:  types.StringValue(offeringID),
		LookupKey:   types.StringValue(pkg.LookupKey),
		DisplayName: stringOrNull(pkg.DisplayName),
		Position:    int64OrNull(pkg.Position),
		CreatedAt:   types.Int64Value(pkg.CreatedAt),
	}
}
