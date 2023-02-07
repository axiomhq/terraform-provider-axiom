package axiom

import (
	"context"
	"log"

	ax "github.com/axiomhq/axiom-go/axiom"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure the implementation satisfies the expected interfaces
var (
	_ provider.Provider = &axiomProvider{}
)

// New is a helper function to simplify provider server and testing implementation.
func New() provider.Provider {
	return &axiomProvider{}
}

// axiomProvider is the provider implementation.
type axiomProvider struct{}

// AxiomProviderModel describes the provider data model.
type AxiomProviderModel struct {
	Token types.String `tfsdk:"token"`
	OrgID types.String `tfsdk:"org_id"`
}

// Metadata returns the provider type name.
func (p *axiomProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "axiom"
}

// Schema defines the provider-level schema for configuration data.
func (p *axiomProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"token": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The Axiom API token.",
			},
			"org_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The Axiom organization ID.",
			},
		},
	}
}

// Configure prepares a Axiom API client for data sources and resources.
func (p *axiomProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data AxiomProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if data.Token.IsNull() {
		resp.Diagnostics.AddAttributeError(
			path.Root("token"),
			"Token is required",
			"Please set the token in the provider configuration block.",
		)
		return
	}

	client, err := ax.NewClient(
		ax.SetPersonalTokenConfig(data.Token.ValueString(), data.OrgID.ValueString()),
	)
	if err != nil {
		log.Println(err)
		return
	}

	resp.DataSourceData = client
	resp.ResourceData = client
}

// DataSources defines the data sources implemented in the provider.
func (p *axiomProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewDatasetDataSource,
	}
}

// Resources defines the resources implemented in the provider.
func (p *axiomProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewDatasetResource,
	}
}
