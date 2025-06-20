package axiom

import (
	"context"
	"os"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	ax "github.com/axiomhq/axiom-go/axiom"
)

const (
	providerUserAgent = "terraform-provider-axiom/v1.4.6"
)

// Ensure the implementation satisfies the expected interfaces
var (
	_ provider.Provider = &axiomProvider{}
)

// AxiomProviderModel describes the provider data model.
type AxiomProviderModel struct {
	ApiToken types.String `tfsdk:"api_token"`
	BaseUrl  types.String `tfsdk:"base_url"`
}

// NewAxiomProvider is a helper function to simplify provider server and testing implementation.
func NewAxiomProvider() provider.Provider {
	return &axiomProvider{}
}

// axiomProvider is the provider implementation.
type axiomProvider struct{}

// Metadata returns the provider type name.
func (p *axiomProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "axiom"
}

// Schema defines the provider-level schema for configuration data.
func (p *axiomProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"api_token": schema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				MarkdownDescription: "The Axiom API token.",
			},
			"base_url": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "The base url of the axiom api.",
			},
		},
	}
}

// Configure prepares a Axiom API client for data sources and resources.
func (p *axiomProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config AxiomProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiToken := os.Getenv("AXIOM_API_TOKEN")
	baseUrl := os.Getenv("AXIOM_BASE_URL")

	if !config.ApiToken.IsNull() {
		apiToken = config.ApiToken.ValueString()
	}
	if !config.BaseUrl.IsNull() {
		baseUrl = config.BaseUrl.ValueString()
	}

	if apiToken == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("token"),
			"ApiToken is required",
			"Please set the token in the provider configuration block.",
		)
		return
	}

	if !strings.HasPrefix(apiToken, "xaat-") {
		resp.Diagnostics.AddError("invalid api token", "Please set a valid advanced api token in the provider configuration block.")
		return
	}

	ops := []ax.Option{
		ax.SetAPITokenConfig(apiToken),
		ax.SetUserAgent(providerUserAgent),
	}

	if baseUrl != "" {
		ops = append(ops, ax.SetURL(baseUrl))
	}

	client, err := ax.NewClient(ops...)
	if err != nil {
		resp.Diagnostics.AddError("unable to create axiom client", err.Error())
	}

	resp.DataSourceData = client
	resp.ResourceData = client
}

// DataSources defines the data sources implemented in the provider.
func (p *axiomProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewDatasetDataSource,
		NewMonitorDataSource,
		NewNotifierDataSource,
		NewUserDataSource,
		NewTokenDataSource,
		NewVirtualFieldDataSource,
	}
}

// Resources defines the resources implemented in the provider.
func (p *axiomProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewDatasetResource,
		NewMonitorResource,
		NewNotifierResource,
		NewUserResource,
		NewTokenResource,
		NewVirtualFieldResource,
	}
}
