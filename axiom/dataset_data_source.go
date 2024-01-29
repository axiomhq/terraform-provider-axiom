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
var _ datasource.DataSource = &DatasetDataSource{}

func NewDatasetDataSource() datasource.DataSource {
	return &DatasetDataSource{}
}

type DatasetDataSource struct {
	client *axiom.Client
}

func (d *DatasetDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *DatasetDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dataset"
}

func (d *DatasetDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Example dataset",

		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				MarkdownDescription: "Dataset name",
				Optional:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Dataset description",
				Optional:            true,
			},
			"id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Dataset identifier",
			},
		},
	}
}

func (d *DatasetDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data DatasetResourceModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if d.client == nil {
		resp.Diagnostics.AddError("axiom client is nil", "looks like the client wasn't setup properly")
		return
	}

	tflog.Debug(ctx, "calling axiom to get dataset")
	tflog.Info(ctx, fmt.Sprintf("calling axiom to get dataset with id: %s", data.ID.ValueString()))
	ds, err := d.client.Datasets.Get(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("failed to read dataset", err.Error())
		tflog.Error(ctx, err.Error())
		return
	}
	data.ID = types.StringValue(ds.ID)
	data.Name = types.StringValue(ds.Name)
	data.Description = types.StringValue(ds.Description)

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
