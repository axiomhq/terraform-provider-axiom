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
var _ datasource.DataSource = &UserDataSource{}

func NewUserDataSource() datasource.DataSource {
	return &UserDataSource{}
}

type UserDataSource struct {
	client *axiom.Client
}

func (d *UserDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *UserDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user"
}

func (d *UserDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Example user",

		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				MarkdownDescription: "User name",
				Optional:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Users description",
				Optional:            true,
			},
			"id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Users identifier",
			},
		},
	}
}

func (d *UserDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data UsersResourceModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if d.client == nil {
		resp.Diagnostics.AddError("axiom client is nil", "looks like the client wasn't setup properly")
		return
	}

	tflog.Debug(ctx, "calling axiom to get user")
	tflog.Info(ctx, fmt.Sprintf("calling axiom to get user with id: %s", data.ID.ValueString()))
	ds, err := d.client.Users.Get(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("failed to read user", err.Error())
		tflog.Error(ctx, err.Error())
		return
	}
	data.ID = types.StringValue(ds.ID)
	data.Name = types.StringValue(ds.Name)
	data.Email = types.StringValue(ds.Email)
	data.Role = types.StringValue(ds.Role.ID)

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
