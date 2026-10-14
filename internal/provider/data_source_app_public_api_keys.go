package provider

import (
	"context"
	"sort"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/nmehlei/terraform-provider-revenuecat/internal/revenuecat"
)

var (
	_ datasource.DataSource              = (*appPublicAPIKeysDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*appPublicAPIKeysDataSource)(nil)
)

// NewAppPublicAPIKeysDataSource returns the revenuecat_app_public_api_keys data source.
func NewAppPublicAPIKeysDataSource() datasource.DataSource {
	return &appPublicAPIKeysDataSource{}
}

type appPublicAPIKeysDataSource struct {
	client *revenuecat.Client
}

type appPublicAPIKeysModel struct {
	ProjectID     types.String        `tfsdk:"project_id"`
	AppID         types.String        `tfsdk:"app_id"`
	ProductionKey types.String        `tfsdk:"production_key"`
	Keys          []publicAPIKeyModel `tfsdk:"keys"`
}

type publicAPIKeyModel struct {
	ID          types.String `tfsdk:"id"`
	Key         types.String `tfsdk:"key"`
	Environment types.String `tfsdk:"environment"`
	CreatedAt   types.Int64  `tfsdk:"created_at"`
}

func (d *appPublicAPIKeysDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_app_public_api_keys"
}

func (d *appPublicAPIKeysDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Reads the public SDK keys RevenueCat generated for an app. These keys are " +
			"public by design (they ship inside the app), so they are not marked sensitive.",
		Attributes: map[string]schema.Attribute{
			"project_id": schema.StringAttribute{
				MarkdownDescription: "Identifier of the project the app belongs to.",
				Required:            true,
			},
			"app_id": schema.StringAttribute{
				MarkdownDescription: "Identifier of the app.",
				Required:            true,
			},
			"production_key": schema.StringAttribute{
				MarkdownDescription: "The oldest production key, which is what an app configures its SDK " +
					"with. Null when the app has no production key.",
				Computed: true,
			},
			"keys": schema.ListNestedAttribute{
				MarkdownDescription: "Every public key of the app, oldest first.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id":          schema.StringAttribute{Computed: true, MarkdownDescription: "Identifier of the key."},
						"key":         schema.StringAttribute{Computed: true, MarkdownDescription: "The key itself."},
						"environment": schema.StringAttribute{Computed: true, MarkdownDescription: "`production` or `sandbox`."},
						"created_at":  schema.Int64Attribute{Computed: true, MarkdownDescription: "Creation time, in milliseconds since the Unix epoch."},
					},
				},
			},
		},
	}
}

func (d *appPublicAPIKeysDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configureDataSourceClient(req, resp)
}

func (d *appPublicAPIKeysDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config appPublicAPIKeysModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	keys, err := d.client.ListAppPublicAPIKeys(ctx, config.ProjectID.ValueString(), config.AppID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to read the RevenueCat app's public API keys", err.Error())
		return
	}

	sortPublicKeys(keys)
	config.ProductionKey = types.StringNull()
	config.Keys = make([]publicAPIKeyModel, 0, len(keys))
	for _, key := range keys {
		if key.Environment == revenuecat.EnvironmentProduction && config.ProductionKey.IsNull() {
			config.ProductionKey = types.StringValue(key.Key)
		}
		config.Keys = append(config.Keys, publicAPIKeyModel{
			ID:          types.StringValue(key.ID),
			Key:         types.StringValue(key.Key),
			Environment: types.StringValue(key.Environment),
			CreatedAt:   types.Int64Value(key.CreatedAt),
		})
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, config)...)
}

// sortPublicKeys orders keys oldest first, by identifier on a tie, so the
// chosen production key and the list are stable across reads.
func sortPublicKeys(keys []revenuecat.PublicAPIKey) {
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].CreatedAt != keys[j].CreatedAt {
			return keys[i].CreatedAt < keys[j].CreatedAt
		}
		return keys[i].ID < keys[j].ID
	})
}
