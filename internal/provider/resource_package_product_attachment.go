package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/nmehlei/terraform-provider-revenuecat/internal/revenuecat"
)

var (
	_ resource.Resource                = (*packageProductAttachmentResource)(nil)
	_ resource.ResourceWithConfigure   = (*packageProductAttachmentResource)(nil)
	_ resource.ResourceWithImportState = (*packageProductAttachmentResource)(nil)
)

// NewPackageProductAttachmentResource returns the
// revenuecat_package_product_attachment resource.
func NewPackageProductAttachmentResource() resource.Resource {
	return &packageProductAttachmentResource{}
}

type packageProductAttachmentResource struct {
	client *revenuecat.Client
}

type packageProductAttachmentModel struct {
	ID        types.String `tfsdk:"id"`
	ProjectID types.String `tfsdk:"project_id"`
	PackageID types.String `tfsdk:"package_id"`
	Products  types.Set    `tfsdk:"product"`
}

// packageProductBlockModel mirrors one product block.
type packageProductBlockModel struct {
	ProductID           types.String `tfsdk:"product_id"`
	EligibilityCriteria types.String `tfsdk:"eligibility_criteria"`
}

func (r *packageProductAttachmentResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_package_product_attachment"
}

func (r *packageProductAttachmentResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Attaches products to a RevenueCat package, each under an eligibility criteria.\n\n" +
			"~> **Only one attachment resource may manage a given package.** This resource owns the " +
			"entire set of products attached to `package_id`. Declaring a second " +
			"`revenuecat_package_product_attachment` for the same package makes the two resources " +
			"fight over that set, producing a permanent diff.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Synthetic identifier of the attachment, in the form `<project_id>:<package_id>`.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"project_id": schema.StringAttribute{
				MarkdownDescription: "Identifier of the project the package belongs to. Changing this forces a new attachment.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"package_id": schema.StringAttribute{
				MarkdownDescription: "Identifier of the package the products are attached to. Changing this forces a new attachment.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
		Blocks: map[string]schema.Block{
			"product": schema.SetNestedBlock{
				MarkdownDescription: "A product attached to the package. At least one is required; " +
					"destroy the resource instead of removing them all.",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"product_id": schema.StringAttribute{
							MarkdownDescription: "Identifier of the product to attach.",
							Required:            true,
						},
						"eligibility_criteria": schema.StringAttribute{
							MarkdownDescription: fmt.Sprintf(
								"Eligibility criteria the product is attached under. One of `%s`. The API "+
									"requires this on every attachment despite it looking optional; defaults "+
									"to `all` — eligible for every customer — when not set.",
								joinBackticked(revenuecat.EligibilityCriteriaValues),
							),
							Optional: true,
							Computed: true,
							Default:  stringdefault.StaticString(revenuecat.EligibilityCriteriaAll),
							Validators: []validator.String{
								stringvalidator.OneOf(revenuecat.EligibilityCriteriaValues...),
							},
						},
					},
				},
				Validators: []validator.Set{
					setvalidator.SizeAtLeast(1),
				},
			},
		},
	}
}

func (r *packageProductAttachmentResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configureResourceClient(req, resp)
}

func (r *packageProductAttachmentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan packageProductAttachmentModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	desired := packageProductsFromFramework(ctx, plan.Products, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := plan.ProjectID.ValueString()
	packageID := plan.PackageID.ValueString()

	if err := r.client.AttachProductsToPackage(ctx, projectID, packageID, toAPIPackageProducts(desired)); err != nil {
		resp.Diagnostics.AddError("Unable to attach products to the RevenueCat package", err.Error())
		return
	}

	plan.ID = types.StringValue(attachmentID(projectID, packageID))
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *packageProductAttachmentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state packageProductAttachmentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := state.ProjectID.ValueString()
	packageID := state.PackageID.ValueString()

	attached, err := r.client.ListPackageProducts(ctx, projectID, packageID)
	if err != nil {
		if revenuecat.IsNotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read the products attached to the RevenueCat package", err.Error())
		return
	}

	if len(attached) == 0 {
		resp.State.RemoveResource(ctx)
		return
	}

	products, diags := packageProductsToFramework(ctx, attached)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	state.ID = types.StringValue(attachmentID(projectID, packageID))
	state.Products = products
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *packageProductAttachmentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state packageProductAttachmentModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	current := packageProductsFromFramework(ctx, state.Products, &resp.Diagnostics)
	desired := packageProductsFromFramework(ctx, plan.Products, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID := state.ProjectID.ValueString()
	packageID := state.PackageID.ValueString()
	toAttach, toDetach := diffPackageProducts(current, desired)

	if err := r.client.AttachProductsToPackage(ctx, projectID, packageID, toAPIPackageProducts(toAttach)); err != nil {
		resp.Diagnostics.AddError("Unable to attach products to the RevenueCat package", err.Error())
		return
	}
	if err := r.client.DetachProductsFromPackage(ctx, projectID, packageID, toDetach); err != nil {
		resp.Diagnostics.AddError("Unable to detach products from the RevenueCat package", err.Error())
		return
	}

	plan.ID = types.StringValue(attachmentID(projectID, packageID))
	resp.Diagnostics.Append(resp.State.Set(ctx, plan)...)
}

func (r *packageProductAttachmentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state packageProductAttachmentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	current := packageProductsFromFramework(ctx, state.Products, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	productIDs := make([]string, 0, len(current))
	for _, entry := range current {
		productIDs = append(productIDs, entry.ProductID)
	}

	err := r.client.DetachProductsFromPackage(ctx, state.ProjectID.ValueString(), state.PackageID.ValueString(), productIDs)
	if err != nil && !revenuecat.IsNotFound(err) {
		resp.Diagnostics.AddError("Unable to detach products from the RevenueCat package", err.Error())
	}
}

func (r *packageProductAttachmentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts, err := parseImportID(req.ID, "project_id", "package_id")
	if err != nil {
		importIDError(&resp.Diagnostics, err)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("package_id"), parts[1])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), attachmentID(parts[0], parts[1]))...)
}

func packageProductsFromFramework(ctx context.Context, value types.Set, diags *diag.Diagnostics) []packageProductSpec {
	if value.IsNull() || value.IsUnknown() {
		return nil
	}

	var blocks []packageProductBlockModel
	diags.Append(value.ElementsAs(ctx, &blocks, false)...)
	if diags.HasError() {
		return nil
	}

	specs := make([]packageProductSpec, 0, len(blocks))
	for _, block := range blocks {
		specs = append(specs, packageProductSpec{
			ProductID:           block.ProductID.ValueString(),
			EligibilityCriteria: block.EligibilityCriteria.ValueString(),
		})
	}
	return specs
}

func packageProductsToFramework(ctx context.Context, products []revenuecat.PackageProduct) (types.Set, diag.Diagnostics) {
	blocks := make([]packageProductBlockModel, 0, len(products))
	for _, product := range products {
		blocks = append(blocks, packageProductBlockModel{
			ProductID:           types.StringValue(product.ProductID),
			EligibilityCriteria: stringOrNull(product.EligibilityCriteria),
		})
	}

	return types.SetValueFrom(ctx, packageProductObjectType(), blocks)
}

func packageProductObjectType() types.ObjectType {
	return types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"product_id":           types.StringType,
			"eligibility_criteria": types.StringType,
		},
	}
}

func toAPIPackageProducts(specs []packageProductSpec) []revenuecat.PackageProduct {
	products := make([]revenuecat.PackageProduct, 0, len(specs))
	for _, spec := range specs {
		products = append(products, revenuecat.PackageProduct{
			ProductID:           spec.ProductID,
			EligibilityCriteria: spec.EligibilityCriteria,
		})
	}
	return products
}
