package axiom

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"

	ax "github.com/axiomhq/axiom-go/axiom"
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
	Token   types.String `tfsdk:"token"`
	OrgID   types.String `tfsdk:"org_id"`
	BaseUrl types.String `tfsdk:"base_url"`
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
			"base_url": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "The base url of the axiom api.",
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

	ops := []ax.Option{
		ax.SetPersonalTokenConfig(data.Token.ValueString(), data.OrgID.ValueString()),
	}

	if !data.BaseUrl.IsNull() {
		ops = append(ops, ax.SetURL(data.BaseUrl.ValueString()))
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
	}
}

// Resources defines the resources implemented in the provider.
func (p *axiomProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewDatasetResource,
		NewMonitorResource,
		NewNotifierResource,
		NewUserResource,
	}
}
