package axiom

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/resource"
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

func (d *DatasetDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *DatasetDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dataset"
}

func (d *DatasetDataSource) Schema(ctx context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	var r DatasetResource
	var resourceResp resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &resourceResp)

	resp.Schema = frameworkDatasourceSchemaFromFrameworkResourceSchema(resourceResp.Schema)
}

func (d *DatasetDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var plan DatasetResourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if d.client == nil {
		resp.Diagnostics.AddError("axiom client is nil", "looks like the client wasn't setup properly")
		return
	}

	ds, err := d.client.Datasets.Get(ctx, plan.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("failed to read dataset", err.Error())
		tflog.Error(ctx, err.Error())
		return
	}

	plan = *flattenDataset(ds)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}
