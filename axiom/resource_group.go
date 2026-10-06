package axiom

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/axiomhq/axiom-go/axiom"
)

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ resource.Resource                = &GroupResource{}
	_ resource.ResourceWithImportState = &GroupResource{}
)

func NewGroupResource() resource.Resource {
	return &GroupResource{}
}

// GroupResource defines the resource implementation.
type GroupResource struct {
	client *axiom.Client
}

// GroupResourceModel describes the resource data model.
type GroupResourceModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	Roles       types.Set    `tfsdk:"roles"`
	Members     types.Set    `tfsdk:"members"`
	IsManaged   types.Bool   `tfsdk:"is_managed"`
}

func (r *GroupResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_group"
}

func (r *GroupResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a group. Members of a group receive the capabilities of all roles assigned to the group, " +
			"in addition to their base role. Requires the RBAC add-on.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Group identifier",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Group name. Must be unique in the organization.",
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"description": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Group description",
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"roles": schema.SetAttribute{
				Optional:            true,
				Computed:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "IDs of the roles assigned to the group. Defaults to no roles.",
				Default:             setdefault.StaticValue(emptyStringSet),
			},
			"members": schema.SetAttribute{
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
				MarkdownDescription: "IDs of the users in the group. When omitted, Terraform doesn't manage the members and keeps the current ones. " +
					"The members of a managed group can't be changed.",
				PlanModifiers: []planmodifier.Set{
					setplanmodifier.UseStateForUnknown(),
				},
			},
			"is_managed": schema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether the group is synced from an identity provider",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *GroupResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *GroupResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan GroupResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.client == nil {
		resp.Diagnostics.AddError("Client Error", "Client is not set")
		return
	}

	groupReq, diags := expandGroupRequest(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	group, err := r.client.Groups.Create(ctx, groupReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create group, got error: %s", err))
		return
	}

	state, diags := flattenGroup(group)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *GroupResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state GroupResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	group, err := r.client.Groups.Get(ctx, state.ID.ValueString())
	if err != nil {
		if isNotFoundError(err) {
			resp.Diagnostics.AddWarning(
				"Group Not Found",
				fmt.Sprintf("Group with ID %s does not exist and will be recreated if still defined in the configuration.", state.ID.ValueString()),
			)
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read group", err.Error())
		return
	}

	newState, diags := flattenGroup(group)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, newState)...)
}

func (r *GroupResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan GroupResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	groupReq, diags := expandGroupRequest(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	group, err := r.client.Groups.Update(ctx, plan.ID.ValueString(), groupReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update group, got error: %s", err))
		return
	}

	state, diags := flattenGroup(group)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// The API ignores member changes for managed groups instead of failing.
	if group.IsManaged && !plan.Members.Equal(state.Members) {
		resp.Diagnostics.AddAttributeError(
			path.Root("members"),
			"Members of a managed group can't be changed",
			fmt.Sprintf("Group %q is synced from an identity provider. Change its members there, and remove `members` from the configuration.", group.Name),
		)
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *GroupResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state GroupResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.Groups.Delete(ctx, state.ID.ValueString()); err != nil {
		if isNotFoundError(err) {
			return
		}
		resp.Diagnostics.AddError("Failed to delete group", err.Error())
		return
	}
}

func (r *GroupResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func expandGroupRequest(ctx context.Context, plan GroupResourceModel) (axiom.GroupRequest, diag.Diagnostics) {
	var diags diag.Diagnostics

	roles := []string{}
	if !plan.Roles.IsNull() && !plan.Roles.IsUnknown() {
		diags.Append(plan.Roles.ElementsAs(ctx, &roles, false)...)
	}

	// Unknown only on create when members are omitted: create the group
	// without members.
	members := []string{}
	if !plan.Members.IsNull() && !plan.Members.IsUnknown() {
		diags.Append(plan.Members.ElementsAs(ctx, &members, false)...)
	}

	return axiom.GroupRequest{
		Name:        plan.Name.ValueString(),
		Description: plan.Description.ValueString(),
		Roles:       roles,
		Members:     members,
	}, diags
}

func flattenGroup(group *axiom.Group) (GroupResourceModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	roles, d := flattenStringSet(group.Roles)
	diags.Append(d...)

	members, d := flattenStringSet(group.Members)
	diags.Append(d...)

	description := types.StringNull()
	if group.Description != "" {
		description = types.StringValue(group.Description)
	}

	return GroupResourceModel{
		ID:          types.StringValue(group.ID),
		Name:        types.StringValue(group.Name),
		Description: description,
		Roles:       roles,
		Members:     members,
		IsManaged:   types.BoolValue(group.IsManaged),
	}, diags
}
