package provider

import (
	"context"
	"os"

	"github.com/coderabbitai/terraform-provider-coderabbit/internal/client"
	"github.com/coderabbitai/terraform-provider-coderabbit/internal/resources"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ provider.Provider = &CodeRabbitProvider{}

type CodeRabbitProvider struct {
	version string
}

type CodeRabbitProviderModel struct {
	APIKey  types.String `tfsdk:"api_key"`
	BaseURL types.String `tfsdk:"base_url"`
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &CodeRabbitProvider{
			version: version,
		}
	}
}

func (p *CodeRabbitProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "coderabbit"
	resp.Version = p.version
}

func (p *CodeRabbitProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Terraform provider for managing CodeRabbit resources including seat assignments.",
		Attributes: map[string]schema.Attribute{
			"api_key": schema.StringAttribute{
				Description: "CodeRabbit API key for authentication. Can also be set via CODERABBITAI_API_KEY environment variable.",
				Optional:    true,
				Sensitive:   true,
			},
			"base_url": schema.StringAttribute{
				Description: "Base URL for CodeRabbit API. Defaults to https://api.coderabbit.ai. Can also be set via CODERABBIT_BASE_URL environment variable.",
				Optional:    true,
			},
		},
	}
}

func (p *CodeRabbitProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config CodeRabbitProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get API key from config or environment variable
	apiKey := os.Getenv("CODERABBITAI_API_KEY")
	if !config.APIKey.IsNull() {
		apiKey = config.APIKey.ValueString()
	}

	if apiKey == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("api_key"),
			"Missing CodeRabbit API Key",
			"The provider cannot create the CodeRabbit API client because the API key is missing. "+
				"Set the api_key attribute in the provider configuration or set the CODERABBITAI_API_KEY environment variable.",
		)
		return
	}

	// Get base URL from config or environment variable
	baseURL := os.Getenv("CODERABBIT_BASE_URL")
	if !config.BaseURL.IsNull() {
		baseURL = config.BaseURL.ValueString()
	}
	if baseURL == "" {
		baseURL = "https://api.coderabbit.ai"
	}

	// Create API client
	c := client.NewClient(apiKey, baseURL)

	// Make the client available to resources and data sources
	resp.DataSourceData = c
	resp.ResourceData = c
}

func (p *CodeRabbitProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		resources.NewSeatsResource,
	}
}

func (p *CodeRabbitProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		resources.NewSeatsDataSource,
	}
}
