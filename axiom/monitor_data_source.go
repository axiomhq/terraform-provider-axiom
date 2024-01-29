package axiom

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/axiomhq/axiom-go/axiom"
)

// Ensure the implementation satisfies the desired interfaces.
var _ datasource.DataSource = &MonitorDataSource{}

func NewMonitorDataSource() datasource.DataSource {
	return &MonitorDataSource{}
}

type MonitorDataSource struct {
	client *axiom.Client
}

func (d *MonitorDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
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

func (d *MonitorDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_monitor"
}

func (d *MonitorDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Example Monitor",

		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				MarkdownDescription: "Monitor name",
				Optional:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Monitor description",
				Optional:            true,
			},
			"id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Monitor identifier",
			},
		},
	}
}

func (d *MonitorDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data MonitorResourceModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if d.client == nil {
		resp.Diagnostics.AddError("axiom client is nil", "looks like the client wasn't setup properly")
		return
	}

	tflog.Debug(ctx, "calling axiom to get Monitor")
	tflog.Info(ctx, fmt.Sprintf("calling axiom to get Monitor with id: %s", data.ID.ValueString()))
	ds, err := d.client.Monitors.Get(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("failed to read Monitor", err.Error())
		tflog.Error(ctx, err.Error())
		return
	}
	data.ID = types.StringValue(ds.ID)
	data.Name = types.StringValue(ds.Name)
	data.Description = types.StringValue(ds.Description)
	data.AlertOnNoData = types.BoolValue(ds.AlertOnNoData)
	data.AplQuery = types.StringValue(ds.AplQuery)
	data.Disabled = types.BoolValue(ds.Disabled)
	data.IntervalMinutes = types.Int64Value(ds.IntervalMinutes)
	data.MatchEveryN = types.Int64Value(ds.MatchEveryN)
	data.MatchValue = types.StringValue(ds.MatchValue)

	for _, item := range ds.NotifierIds {
		data.NotifierIds = append(data.NotifierIds, types.StringValue(item))
	}

	data.Operator = types.StringValue(ds.Operator)
	data.RangeMinutes = types.Int64Value(ds.RangeMinutes)
	data.Threshold = types.Float64Value(ds.Threshold)

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
