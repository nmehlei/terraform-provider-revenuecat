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
	_ resource.Resource                = (*productResource)(nil)
	_ resource.ResourceWithConfigure   = (*productResource)(nil)
	_ resource.ResourceWithImportState = (*productResource)(nil)
)

// NewProductResource returns the revenuecat_product resource.
func NewProductResource() resource.Resource {
	return &productResource{}
}

type productResource struct {
	client *revenuecat.Client
}

type productModel struct {
	ID              types.String `tfsdk:"id"`
	ProjectID       types.String `tfsdk:"project_id"`
	AppID           types.String `tfsdk:"app_id"`
	StoreIdentifier types.String `tfsdk:"store_identifier"`
	Type            types.String `tfsdk:"type"`
	DisplayName     types.String `tfsdk:"display_name"`
	CreatedAt       types.Int64  `tfsdk:"created_at"`
}

func (r *productResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_product"
}

func (r *productResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a store product within a RevenueCat project.\n\n" +
			"RevenueCat does not support changing a product's identity, so every attribute other " +
			"than `display_name` forces a new product when changed.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "RevenueCat identifier of the product.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"project_id": schema.StringAttribute{
				MarkdownDescription: "Identifier of the project the product belongs to. Changing this forces a new product.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"app_id": schema.StringAttribute{
				MarkdownDescription: "Identifier of the app the product belongs to. Changing this forces a new product.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"store_identifier": schema.StringAttribute{
				MarkdownDescription: "Product identifier in the underlying store, for example the App Store " +
					"product ID. For a Play Store subscription this is not the bare subscription ID — " +
					"the API 400s with \"must follow the format 'subscriptionId:basePlanId'\" unless it's " +
					"`<subscription ID>:<base plan ID>`, reflecting that a Play Billing subscription is one " +
					"subscription with multiple base plans, each a separate RevenueCat product. Changing " +
					"this forces a new product.",
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"type": schema.StringAttribute{
				MarkdownDescription: fmt.Sprintf(
					"Type of the product. One of `%s`. Changing this forces a new product.",
					joinBackticked(revenuecat.ProductTypes),
				),
				Required: true,
				Validators: []validator.String{
					stringvalidator.OneOf(revenuecat.ProductTypes...),
				},
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"display_name": schema.StringAttribute{
				MarkdownDescription: "Human-readable name of the product, shown in the RevenueCat dashboard.",
				Optional:            true,
			},
			"created_at": schema.Int64Attribute{
				MarkdownDescription: "Creation time of the product, in milliseconds since the Unix epoch.",
				Computed:            true,
				PlanModifiers: []planmodifier.Int64{
					int64UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *productResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureResourceClient(req, resp)
}

func (r *productResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan productModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	product, err := r.client.CreateProduct(ctx, plan.ProjectID.ValueString(), revenuecat.CreateProductRequest{
		StoreIdentifier: plan.StoreIdentifier.ValueString(),
		AppID:           plan.AppID.ValueString(),
		Type:            plan.Type.ValueString(),
		DisplayName:     optionalString(plan.DisplayName),
	})
	if err != nil {
		resp.Diagnostics.AddError("Unable to create the RevenueCat product", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, productToModel(plan.ProjectID.ValueString(), product))...)
}

func (r *productResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state productModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	product, err := r.client.GetProduct(ctx, state.ProjectID.ValueString(), state.ID.ValueString())
	if err != nil {
		if revenuecat.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read the RevenueCat product", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, productToModel(state.ProjectID.ValueString(), product))...)
}

// Update handles the only in-place change the API supports. Every other
// attribute carries RequiresReplace, so a plan reaching Update can only be a
// display name change; RevenueCat exposes no product update endpoint, so the
// new value is recorded in state directly.
func (r *productResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state productModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	plan.ID = state.ID
	plan.CreatedAt = state.CreatedAt
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *productResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state productModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteProduct(ctx, state.ProjectID.ValueString(), state.ID.ValueString())
	if err != nil && !revenuecat.IsNotFound(err) {
		resp.Diagnostics.AddError("Unable to delete the RevenueCat product", err.Error())
	}
}

func (r *productResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts, err := parseImportID(req.ID, "project_id", "product_id")
	if err != nil {
		importIDError(&resp.Diagnostics, err)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[1])...)
}

func productToModel(projectID string, product *revenuecat.Product) productModel {
	return productModel{
		ID:              types.StringValue(product.ID),
		ProjectID:       types.StringValue(projectID),
		AppID:           types.StringValue(product.AppID),
		StoreIdentifier: types.StringValue(product.StoreIdentifier),
		Type:            types.StringValue(product.Type),
		DisplayName:     stringOrNull(product.DisplayName),
		CreatedAt:       types.Int64Value(product.CreatedAt),
	}
}
