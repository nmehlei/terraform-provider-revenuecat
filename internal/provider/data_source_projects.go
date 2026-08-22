package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/nmehlei/terraform-provider-revenuecat/internal/revenuecat"
)

var (
	_ datasource.DataSource              = (*projectsDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*projectsDataSource)(nil)
)

// NewProjectsDataSource returns the revenuecat_projects data source.
func NewProjectsDataSource() datasource.DataSource {
	return &projectsDataSource{}
}

type projectsDataSource struct {
	client *revenuecat.Client
}

type projectsDataSourceModel struct {
	Projects []projectDataSourceModel `tfsdk:"projects"`
}

func (d *projectsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_projects"
}

func (d *projectsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists every RevenueCat project accessible with the configured API key.",
		Attributes: map[string]schema.Attribute{
			"projects": schema.ListNestedAttribute{
				MarkdownDescription: "The accessible projects.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							MarkdownDescription: "Identifier of the project.",
							Computed:            true,
						},
						"name": schema.StringAttribute{
							MarkdownDescription: "Name of the project.",
							Computed:            true,
						},
						"created_at": schema.Int64Attribute{
							MarkdownDescription: "Creation time of the project, in milliseconds since the Unix epoch.",
							Computed:            true,
						},
					},
				},
			},
		},
	}
}

func (d *projectsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configureDataSourceClient(req, resp)
}

func (d *projectsDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	projects, err := d.client.ListProjects(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Unable to list RevenueCat projects", err.Error())
		return
	}

	state := projectsDataSourceModel{Projects: make([]projectDataSourceModel, 0, len(projects))}
	for _, project := range projects {
		state.Projects = append(state.Projects, projectDataSourceModel{
			ID:        types.StringValue(project.ID),
			Name:      types.StringValue(project.Name),
			CreatedAt: types.Int64Value(project.CreatedAt),
		})
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}
