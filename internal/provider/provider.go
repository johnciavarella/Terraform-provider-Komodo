package provider

import (
	"context"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/moghtech/terraform-provider-komodo/internal/client"
)

var _ provider.Provider = &KomodoProvider{}

type KomodoProvider struct {
	version string
}

type KomodoProviderModel struct {
	APIKey    types.String `tfsdk:"api_key"`
	APISecret types.String `tfsdk:"api_secret"`
	BaseURL   types.String `tfsdk:"base_url"`
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &KomodoProvider{version: version}
	}
}

func (p *KomodoProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "komodo"
	resp.Version = p.version
}

func (p *KomodoProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Interact with the Komodo build and deployment system.",
		Attributes: map[string]schema.Attribute{
			"api_key": schema.StringAttribute{
				Description: "API key for authentication. Can also be set via the KOMODO_API_KEY environment variable.",
				Optional:    true,
				Sensitive:   true,
			},
			"api_secret": schema.StringAttribute{
				Description: "API secret for authentication. Can also be set via the KOMODO_API_SECRET environment variable.",
				Optional:    true,
				Sensitive:   true,
			},
			"base_url": schema.StringAttribute{
				Description: "Base URL of the Komodo Core API (e.g., https://komodo.example.com). Can also be set via the KOMODO_ADDRESS environment variable.",
				Optional:    true,
			},
		},
	}
}

func (p *KomodoProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config KomodoProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Resolve API key.
	apiKey := os.Getenv("KOMODO_API_KEY")
	if !config.APIKey.IsNull() {
		apiKey = config.APIKey.ValueString()
	}
	if apiKey == "" {
		resp.Diagnostics.AddError(
			"Missing API Key",
			"Set the api_key provider attribute or the KOMODO_API_KEY environment variable.",
		)
		return
	}

	// Resolve API secret.
	apiSecret := os.Getenv("KOMODO_API_SECRET")
	if !config.APISecret.IsNull() {
		apiSecret = config.APISecret.ValueString()
	}
	if apiSecret == "" {
		resp.Diagnostics.AddError(
			"Missing API Secret",
			"Set the api_secret provider attribute or the KOMODO_API_SECRET environment variable.",
		)
		return
	}

	// Resolve base URL.
	baseURL := os.Getenv("KOMODO_ADDRESS")
	if !config.BaseURL.IsNull() {
		baseURL = config.BaseURL.ValueString()
	}
	if baseURL == "" {
		resp.Diagnostics.AddError(
			"Missing Base URL",
			"Set the base_url provider attribute or the KOMODO_ADDRESS environment variable.",
		)
		return
	}

	c := client.NewClient(baseURL, apiKey, apiSecret, "terraform-provider-komodo/"+p.version)

	resp.DataSourceData = c
	resp.ResourceData = c
}

// resolveServerID looks up the server by name/ID and returns its canonical ID.
// It first tries a direct GetServer call (works for IDs and named servers),
// then falls back to listing all servers and matching by name (handles cases
// where the API doesn't support GetServer by name for special values like "Local").
// If resolution fails entirely, the original value is returned.
func resolveServerID(ctx context.Context, c *client.Client, serverID string) string {
	if c == nil || serverID == "" {
		return serverID
	}
	server, err := c.GetServer(ctx, serverID)
	if err == nil {
		return server.ID
	}
	// Fallback: list all servers and match by name.
	servers, err := c.ListServers(ctx)
	if err != nil {
		return serverID
	}
	for _, s := range servers {
		if s.Name == serverID {
			return s.ID
		}
	}
	return serverID
}

// mapTagsToModel converts an API tags slice to the Terraform model field.
// When the API returns no tags, it preserves the current shape (null stays null,
// empty list stays empty) to avoid perpetual plan diffs for tags = [].
func mapTagsToModel(tags []string, current types.List) types.List {
	if len(tags) > 0 {
		list, _ := types.ListValueFrom(context.Background(), types.StringType, tags)
		return list
	}
	if current.IsNull() || current.IsUnknown() {
		return types.ListNull(types.StringType)
	}
	list, _ := types.ListValueFrom(context.Background(), types.StringType, []string{})
	return list
}

// applyResourceTags calls UpdateResourceMeta to set tags on a resource.
// Skips the API call when tags are null or unknown (not managed by config).
func applyResourceTags(ctx context.Context, c *client.Client, targetType, id string, tags types.List, diags *diag.Diagnostics) {
	if tags.IsNull() || tags.IsUnknown() {
		return
	}
	var tagStrings []string
	tags.ElementsAs(ctx, &tagStrings, false)
	if err := c.UpdateResourceMeta(ctx, targetType, id, tagStrings); err != nil {
		diags.AddError("Error updating resource tags", err.Error())
	}
}

func (p *KomodoProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewServerResource,
		NewStackResource,
		NewDeploymentResource,
		NewBuildResource,
		NewRepoResource,
		NewTagResource,
		NewBuilderResource,
		NewUserResource,
		NewApiKeyResource,
		NewGitProviderAccountResource,
	}
}

func (p *KomodoProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewServerDataSource,
		NewStackDataSource,
		NewDeploymentDataSource,
		NewBuildDataSource,
		NewRepoDataSource,
		NewTagDataSource,
		NewBuilderDataSource,
	}
}
