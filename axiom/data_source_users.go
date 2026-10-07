package axiom

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"

	"github.com/axiomhq/axiom-go/axiom"
)

// Ensure the implementation satisfies the desired interfaces.
var _ datasource.DataSource = &UsersDataSource{}

func NewUsersDataSource() datasource.DataSource {
	return &UsersDataSource{}
}

// UsersDataSource lists all users of the organization, so configurations can
// look up user IDs by email.
type UsersDataSource struct {
	client *axiom.Client
}

// UsersDataSourceModel describes the data source data model.
type UsersDataSourceModel struct {
	Users []UsersResourceModel `tfsdk:"users"`
}

func (d *UsersDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*axiom.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected datasource Configure Type",
			fmt.Sprintf("Expected *axiom.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	d.client = client
}

func (d *UsersDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_users"
}

func (d *UsersDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Lists all users of the organization. Use it to look up user IDs by email for `axiom_group` and `axiom_user_role`.",
		Attributes: map[string]schema.Attribute{
			"users": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Users of the organization",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "User identifier",
						},
						"name": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "User name",
						},
						"email": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "User email",
						},
						"role": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Base role of the user: a built-in role name or the ID of a custom role",
						},
					},
				},
			},
		},
	}
}

func (d *UsersDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	if d.client == nil {
		resp.Diagnostics.AddError("axiom client is nil", "looks like the client wasn't setup properly")
		return
	}

	users, err := d.client.Users.List(ctx)
	if err != nil {
		resp.Diagnostics.AddError("failed to list users", err.Error())
		return
	}

	state := UsersDataSourceModel{Users: make([]UsersResourceModel, 0, len(users))}
	for _, user := range users {
		state.Users = append(state.Users, flattenUser(user))
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}
