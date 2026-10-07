package axiom

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/axiomhq/axiom-go/axiom"
)

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ resource.Resource                = &UserRoleResource{}
	_ resource.ResourceWithImportState = &UserRoleResource{}
)

func NewUserRoleResource() resource.Resource {
	return &UserRoleResource{}
}

// UserRoleResource manages the base role of an existing user without owning
// the user's membership in the organization.
type UserRoleResource struct {
	client *axiom.Client
}

// UserRoleResourceModel describes the resource data model.
type UserRoleResourceModel struct {
	ID     types.String `tfsdk:"id"`
	UserID types.String `tfsdk:"user_id"`
	Role   types.String `tfsdk:"role"`
}

func (r *UserRoleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user_role"
}

func (r *UserRoleResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages the base role of an existing user, for example a user provisioned through single sign-on. " +
			"Unlike `axiom_user`, this resource doesn't invite or remove the user. Destroying it leaves the user's current role unchanged. " +
			"Don't manage the same user's role with both `axiom_user` and `axiom_user_role`.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Same as `user_id`",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"user_id": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "User identifier",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"role": schema.StringAttribute{
				Required: true,
				MarkdownDescription: "Base role of the user: `none`, `read-only`, `user`, `admin`, `owner`, or the ID of an `axiom_role`. " +
					"Use `none` when the user should only receive access through groups. An API token can't set or remove the `owner` role.",
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
		},
	}
}

func (r *UserRoleResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*axiom.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *axiom.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = client
}

func (r *UserRoleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan UserRoleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.client == nil {
		resp.Diagnostics.AddError("Client Error", "Client is not set")
		return
	}

	user, err := r.client.Users.UpdateUsersRole(ctx, plan.UserID.ValueString(), axiom.UpdateUserRoleRequest{
		Role: plan.Role.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to set user role, got error: %s", err))
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, flattenUserRole(user))...)
}

func (r *UserRoleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state UserRoleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	user, err := r.client.Users.Get(ctx, state.UserID.ValueString())
	if err != nil {
		if isNotFoundError(err) {
			resp.Diagnostics.AddWarning(
				"User Not Found",
				fmt.Sprintf("User with ID %s is not a member of the organization.", state.UserID.ValueString()),
			)
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read user", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, flattenUserRole(user))...)
}

func (r *UserRoleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan UserRoleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	user, err := r.client.Users.UpdateUsersRole(ctx, plan.UserID.ValueString(), axiom.UpdateUserRoleRequest{
		Role: plan.Role.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to set user role, got error: %s", err))
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, flattenUserRole(user))...)
}

// Delete only removes the resource from the state. Resetting the role would
// need a value to reset to, and removing the user is what axiom_user is for.
func (r *UserRoleResource) Delete(_ context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
}

func (r *UserRoleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("user_id"), req, resp)
}

func flattenUserRole(user *axiom.User) UserRoleResourceModel {
	return UserRoleResourceModel{
		ID:     types.StringValue(user.ID),
		UserID: types.StringValue(user.ID),
		Role:   types.StringValue(user.Role.ID),
	}
}
