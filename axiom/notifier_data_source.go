package axiom

import (
	"context"
	"fmt"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/axiomhq/axiom-go/axiom"
)

// Ensure the implementation satisfies the desired interfaces.
var _ datasource.DataSource = &NotifierDataSource{}

func NewNotifierDataSource() datasource.DataSource {
	return &NotifierDataSource{}
}

type NotifierDataSource struct {
	client *axiom.Client
}

func (d *NotifierDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *NotifierDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_notifier"
}

func (d *NotifierDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Example Notifier",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Notifier identifier",
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Notifier name",
				Required:            true,
			},
			"type": schema.StringAttribute{
				MarkdownDescription: "The type of notifier",
				Required:            true,
			},
			"properties": schema.ObjectAttribute{
				MarkdownDescription: "The properties of the notifier",
				Required:            true,
				AttributeTypes: map[string]attr.Type{
					"slack": types.ObjectType{
						AttrTypes: map[string]attr.Type{
							"slack_url": types.StringType,
						},
					},
				},
			},
		},
	}
}

func (d *NotifierDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data NotifierResourceModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if d.client == nil {
		resp.Diagnostics.AddError("axiom client is nil", "looks like the client wasn't setup properly")
		return
	}

	tflog.Debug(ctx, "calling axiom to get Notifier")
	tflog.Info(ctx, fmt.Sprintf("calling axiom to get Notifier with id: %s", data.ID.ValueString()))
	ds, err := d.client.Notifiers.Get(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("failed to read Notifier", err.Error())
		tflog.Error(ctx, err.Error())
		return
	}

	data.ID = types.StringValue(ds.ID)
	data.Name = types.StringValue(ds.Name)
	data.ID = types.StringValue(ds.ID)
	data.Name = types.StringValue(ds.Name)
	//data.Properties = types.StringValue(ds.Properties)
	data.Type = types.StringValue(ds.Type)

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
