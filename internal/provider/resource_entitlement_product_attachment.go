package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
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
	_ resource.Resource                = (*entitlementProductAttachmentResource)(nil)
	_ resource.ResourceWithConfigure   = (*entitlementProductAttachmentResource)(nil)
	_ resource.ResourceWithImportState = (*entitlementProductAttachmentResource)(nil)
)

// NewEntitlementProductAttachmentResource returns the
// revenuecat_entitlement_product_attachment resource.
func NewEntitlementProductAttachmentResource() resource.Resource {
	return &entitlementProductAttachmentResource{}
}

type entitlementProductAttachmentResource struct {
	client *revenuecat.Client
}

type entitlementProductAttachmentModel struct {
	ID            types.String `tfsdk:"id"`
	ProjectID     types.String `tfsdk:"project_id"`
	EntitlementID types.String `tfsdk:"entitlement_id"`
	ProductIDs    types.Set    `tfsdk:"product_ids"`
}

func (r *entitlementProductAttachmentResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_entitlement_product_attachment"
}

func (r *entitlementProductAttachmentResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Attaches products to a RevenueCat entitlement, granting the entitlement " +
			"to customers who purchase them.\n\n" +
			"~> **Only one attachment resource may manage a given entitlement.** This resource owns " +
			"the entire set of products attached to `entitlement_id`. Declaring a second " +
			"`revenuecat_entitlement_product_attachment` for the same entitlement makes the two " +
			"resources fight over that set, producing a permanent diff.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Synthetic identifier of the attachment, in the form `<project_id>:<entitlement_id>`.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"project_id": schema.StringAttribute{
				MarkdownDescription: "Identifier of the project the entitlement belongs to. Changing this forces a new attachment.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"entitlement_id": schema.StringAttribute{
				MarkdownDescription: "Identifier of the entitlement the products are attached to. Changing this forces a new attachment.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"product_ids": schema.SetAttribute{
				MarkdownDescription: "Identifiers of the products attached to the entitlement. Must not be empty; " +
					"destroy the resource instead of emptying the set.",
				Required:    true,
				ElementType: types.StringType,
				Validators: []validator.Set{
					setvalidator.SizeAtLeast(1),
				},
			},
		},
	}
}

func (r *entitlementProductAttachmentResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureResourceClient(req, resp)
}

func (r *entitlementProductAttachmentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan entitlementProductAttachmentModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	productIDs := stringSetFromFramework(ctx, plan.ProductIDs, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := plan.ProjectID.ValueString()
	entitlementID := plan.EntitlementID.ValueString()

	if err := r.client.AttachProductsToEntitlement(ctx, projectID, entitlementID, productIDs); err != nil {
		resp.Diagnostics.AddError("Unable to attach products to the RevenueCat entitlement", err.Error())
		return
	}

	plan.ID = types.StringValue(attachmentID(projectID, entitlementID))
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *entitlementProductAttachmentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state entitlementProductAttachmentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := state.ProjectID.ValueString()
	entitlementID := state.EntitlementID.ValueString()

	products, err := r.client.ListEntitlementProducts(ctx, projectID, entitlementID)
	if err != nil {
		if revenuecat.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read the products attached to the RevenueCat entitlement", err.Error())
		return
	}

	attached := make([]string, 0, len(products))
	for _, product := range products {
		attached = append(attached, product.ID)
	}

	// An entitlement with nothing attached no longer corresponds to anything
	// this resource manages, so drop it and let Terraform plan a fresh attach.
	if len(attached) == 0 {
		resp.State.RemoveResource(ctx)
		return
	}

	productIDs, diags := types.SetValueFrom(ctx, types.StringType, attached)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	state.ID = types.StringValue(attachmentID(projectID, entitlementID))
	state.ProductIDs = productIDs
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *entitlementProductAttachmentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state entitlementProductAttachmentModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	current := stringSetFromFramework(ctx, state.ProductIDs, &resp.Diagnostics)
	desired := stringSetFromFramework(ctx, plan.ProductIDs, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := state.ProjectID.ValueString()
	entitlementID := state.EntitlementID.ValueString()
	toAttach, toDetach := diffProductIDs(current, desired)

	if err := r.client.AttachProductsToEntitlement(ctx, projectID, entitlementID, toAttach); err != nil {
		resp.Diagnostics.AddError("Unable to attach products to the RevenueCat entitlement", err.Error())
		return
	}
	if err := r.client.DetachProductsFromEntitlement(ctx, projectID, entitlementID, toDetach); err != nil {
		resp.Diagnostics.AddError("Unable to detach products from the RevenueCat entitlement", err.Error())
		return
	}

	plan.ID = types.StringValue(attachmentID(projectID, entitlementID))
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *entitlementProductAttachmentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state entitlementProductAttachmentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	productIDs := stringSetFromFramework(ctx, state.ProductIDs, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	// Only the relationship is destroyed here; the products and the entitlement
	// outlive it.
	err := r.client.DetachProductsFromEntitlement(ctx, state.ProjectID.ValueString(), state.EntitlementID.ValueString(), productIDs)
	if err != nil && !revenuecat.IsNotFound(err) {
		resp.Diagnostics.AddError("Unable to detach products from the RevenueCat entitlement", err.Error())
	}
}

func (r *entitlementProductAttachmentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts, err := parseImportID(req.ID, "project_id", "entitlement_id")
	if err != nil {
		importIDError(&resp.Diagnostics, err)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("entitlement_id"), parts[1])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), attachmentID(parts[0], parts[1]))...)
}

// attachmentID builds the synthetic ID for an attachment, which has no identity
// of its own beyond the pair it connects.
func attachmentID(parentID, childID string) string {
	return fmt.Sprintf("%s:%s", parentID, childID)
}
