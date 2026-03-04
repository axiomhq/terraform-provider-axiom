package axiom

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/axiomhq/axiom-go/axiom"
)

var _ datasource.DataSource = &DashboardDataSource{}

func NewDashboardDataSource() datasource.DataSource {
	return &DashboardDataSource{}
}

type DashboardDataSource struct {
	client *axiom.Client
}

type DashboardDataSourceModel struct {
	ID        types.String `tfsdk:"id"`
	UID       types.String `tfsdk:"uid"`
	Dashboard types.String `tfsdk:"dashboard"`
	Version   types.Int64  `tfsdk:"version"`
	CreatedAt types.String `tfsdk:"created_at"`
	UpdatedAt types.String `tfsdk:"updated_at"`
	CreatedBy types.String `tfsdk:"created_by"`
	UpdatedBy types.String `tfsdk:"updated_by"`
}

func (d *DashboardDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*axiom.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected datasource Configure Type",
			fmt.Sprintf("Expected *http.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	d.client = client
}

func (d *DashboardDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dashboard"
}

func (d *DashboardDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"uid": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Dashboard UID.",
			},
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Internal dashboard identifier returned by the API.",
			},
			"dashboard": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Dashboard document as normalized JSON.",
			},
			"version": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "Monotonic dashboard version.",
			},
			"created_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Creation timestamp returned by the API.",
			},
			"updated_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Last update timestamp returned by the API.",
			},
			"created_by": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Creator returned by the API.",
			},
			"updated_by": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Last updater returned by the API.",
			},
		},
	}
}

func (d *DashboardDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config DashboardDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if d.client == nil {
		resp.Diagnostics.AddError("Client Error", "Client is not set")
		return
	}

	raw, err := d.client.Dashboards.GetRaw(ctx, config.UID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to read dashboard", err.Error())
		return
	}

	dashboard, err := decodeDashboardResource(raw)
	if err != nil {
		resp.Diagnostics.AddError("Failed to read dashboard", fmt.Sprintf("Unable to decode API response: %s", err))
		return
	}

	dashboardJSON, err := normalizeDashboardRaw(dashboard.Dashboard, types.StringValue(string(dashboard.Dashboard)))
	if err != nil {
		resp.Diagnostics.AddError("Failed to read dashboard", fmt.Sprintf("Unable to normalize dashboard payload: %s", err))
		return
	}

	state := DashboardDataSourceModel{
		UID:       types.StringValue(dashboard.UID),
		ID:        types.StringValue(dashboard.ID),
		Dashboard: types.StringValue(dashboardJSON),
		Version:   types.Int64Value(dashboard.Version),
		CreatedAt: types.StringValue(dashboard.CreatedAt),
		UpdatedAt: types.StringValue(dashboard.UpdatedAt),
		CreatedBy: types.StringValue(dashboard.CreatedBy),
		UpdatedBy: types.StringValue(dashboard.UpdatedBy),
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
