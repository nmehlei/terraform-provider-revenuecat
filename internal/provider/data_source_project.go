package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/nmehlei/terraform-provider-revenuecat/internal/revenuecat"
)

var (
	_ datasource.DataSource              = (*projectDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*projectDataSource)(nil)
)

// NewProjectDataSource returns the revenuecat_project data source.
func NewProjectDataSource() datasource.DataSource {
	return &projectDataSource{}
}

type projectDataSource struct {
	client *revenuecat.Client
}

type projectDataSourceModel struct {
	ID        types.String `tfsdk:"id"`
	Name      types.String `tfsdk:"name"`
	CreatedAt types.Int64  `tfsdk:"created_at"`
}

func (d *projectDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project"
}

func (d *projectDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Looks up a RevenueCat project by identifier or by name. Exactly one of " +
			"`id` or `name` must be set.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Identifier of the project. Conflicts with `name`.",
				Optional:            true,
				Computed:            true,
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Name of the project. Must match exactly one project. Conflicts with `id`.",
				Optional:            true,
				Computed:            true,
			},
			"created_at": schema.Int64Attribute{
				MarkdownDescription: "Creation time of the project, in milliseconds since the Unix epoch.",
				Computed:            true,
			},
		},
	}
}

func (d *projectDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configureDataSourceClient(req, resp)
}

func (d *projectDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config projectDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	hasID := !config.ID.IsNull() && config.ID.ValueString() != ""
	hasName := !config.Name.IsNull() && config.Name.ValueString() != ""

	if err := requireExactlyOneSelector(hasID, hasName, "id", "name"); err != nil {
		resp.Diagnostics.AddError("Invalid project selector", err.Error())
		return
	}

	var project *revenuecat.Project
	if hasID {
		found, err := d.client.GetProject(ctx, config.ID.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Unable to read the RevenueCat project", err.Error())
			return
		}
		project = found
	} else {
		projects, err := d.client.ListProjects(ctx)
		if err != nil {
			resp.Diagnostics.AddError("Unable to list RevenueCat projects", err.Error())
			return
		}

		wanted := config.Name.ValueString()
		var matches []revenuecat.Project
		for _, candidate := range projects {
			if candidate.Name == wanted {
				matches = append(matches, candidate)
			}
		}

		switch len(matches) {
		case 0:
			resp.Diagnostics.AddError(
				"No matching RevenueCat project",
				fmt.Sprintf("No project named %q is accessible with the configured API key.", wanted),
			)
			return
		case 1:
			project = &matches[0]
		default:
			ids := make([]string, 0, len(matches))
			for _, match := range matches {
				ids = append(ids, match.ID)
			}
			resp.Diagnostics.AddError(
				"Ambiguous RevenueCat project name",
				fmt.Sprintf("The name %q matches %d projects (%v). Select one by `id` instead.", wanted, len(matches), ids),
			)
			return
		}
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, projectDataSourceModel{
		ID:        types.StringValue(project.ID),
		Name:      types.StringValue(project.Name),
		CreatedAt: types.Int64Value(project.CreatedAt),
	})...)
}

// requireExactlyOneSelector enforces the "look up by ID or by natural key, not
// both and not neither" rule the data sources share.
func requireExactlyOneSelector(hasID, hasKey bool, idName, keyName string) error {
	switch {
	case hasID && hasKey:
		return fmt.Errorf("`%s` and `%s` are mutually exclusive; set exactly one", idName, keyName)
	case !hasID && !hasKey:
		return fmt.Errorf("exactly one of `%s` or `%s` must be set", idName, keyName)
	default:
		return nil
	}
}
