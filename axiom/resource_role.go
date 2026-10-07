package axiom

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/mapvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/axiomhq/axiom-go/axiom"
)

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ resource.Resource                   = &RoleResource{}
	_ resource.ResourceWithImportState    = &RoleResource{}
	_ resource.ResourceWithValidateConfig = &RoleResource{}
)

func NewRoleResource() resource.Resource {
	return &RoleResource{}
}

// RoleResource defines the resource implementation.
type RoleResource struct {
	client *axiom.Client
}

// RoleResourceModel describes the resource data model.
type RoleResourceModel struct {
	ID                  types.String `tfsdk:"id"`
	Name                types.String `tfsdk:"name"`
	Description         types.String `tfsdk:"description"`
	DatasetCapabilities types.Map    `tfsdk:"dataset_capabilities"`
	ViewCapabilities    types.Map    `tfsdk:"view_capabilities"`
	OrgCapabilities     types.Object `tfsdk:"org_capabilities"`
	Members             types.Set    `tfsdk:"members"`
}

// roleCapability describes one capability attribute of a role and the field
// of the axiom-go capabilities struct T it maps to.
type roleCapability[T any] struct {
	name        string
	description string
	actions     []string
	field       func(*T) *[]axiom.Action
}

var roleDatasetCapabilities = []roleCapability[axiom.RoleDatasetCapabilities]{
	{"ingest", "Ingest data into the dataset", []string{Create}, func(c *axiom.RoleDatasetCapabilities) *[]axiom.Action { return &c.Ingest }},
	{"query", "Query the dataset", []string{Read}, func(c *axiom.RoleDatasetCapabilities) *[]axiom.Action { return &c.Query }},
	{"starred_queries", "Manage starred queries of the dataset", []string{Create, Read, Update, Delete}, func(c *axiom.RoleDatasetCapabilities) *[]axiom.Action { return &c.StarredQueries }},
	{"virtual_fields", "Manage virtual fields of the dataset", []string{Create, Read, Update, Delete}, func(c *axiom.RoleDatasetCapabilities) *[]axiom.Action { return &c.VirtualFields }},
	{"trim", "Trim the data in the dataset", []string{Update}, func(c *axiom.RoleDatasetCapabilities) *[]axiom.Action { return &c.Trim }},
	{"vacuum", "Vacuum the fields of the dataset", []string{Update}, func(c *axiom.RoleDatasetCapabilities) *[]axiom.Action { return &c.Vacuum }},
	{"share", "Share the dataset", []string{Create, Read, Delete}, func(c *axiom.RoleDatasetCapabilities) *[]axiom.Action { return &c.Share }},
}

var roleViewCapabilities = []roleCapability[axiom.RoleViewCapabilities]{
	{"query", "Query the view", []string{Read}, func(c *axiom.RoleViewCapabilities) *[]axiom.Action { return &c.Query }},
	{"share", "Share the view", []string{Create, Read, Delete}, func(c *axiom.RoleViewCapabilities) *[]axiom.Action { return &c.Share }},
}

var roleOrgCapabilities = []roleCapability[axiom.RoleOrgCapabilities]{
	{"annotations", "Manage annotations", []string{Create, Read, Update, Delete}, func(c *axiom.RoleOrgCapabilities) *[]axiom.Action { return &c.Annotations }},
	{"api_tokens", "Manage API tokens", []string{Create, Read, Update, Delete}, func(c *axiom.RoleOrgCapabilities) *[]axiom.Action { return &c.APITokens }},
	{"audit_log", "Read the audit log", []string{Read}, func(c *axiom.RoleOrgCapabilities) *[]axiom.Action { return &c.AuditLog }},
	{"billing", "Manage billing", []string{Read, Update}, func(c *axiom.RoleOrgCapabilities) *[]axiom.Action { return &c.Billing }},
	{"dashboards", "Manage dashboards", []string{Create, Read, Update, Delete}, func(c *axiom.RoleOrgCapabilities) *[]axiom.Action { return &c.Dashboards }},
	{"datasets", "Manage all datasets", []string{Create, Read, Update, Delete}, func(c *axiom.RoleOrgCapabilities) *[]axiom.Action { return &c.Datasets }},
	{"endpoints", "Manage endpoints", []string{Create, Read, Update, Delete}, func(c *axiom.RoleOrgCapabilities) *[]axiom.Action { return &c.Endpoints }},
	{"integrations", "Manage integrations", []string{Create, Read, Update, Delete}, func(c *axiom.RoleOrgCapabilities) *[]axiom.Action { return &c.Integrations }},
	{"labels", "Manage labels", []string{Create, Read, Update, Delete}, func(c *axiom.RoleOrgCapabilities) *[]axiom.Action { return &c.Labels }},
	{"monitors", "Manage monitors", []string{Create, Read, Update, Delete}, func(c *axiom.RoleOrgCapabilities) *[]axiom.Action { return &c.Monitors }},
	{"notifiers", "Manage notifiers", []string{Create, Read, Update, Delete}, func(c *axiom.RoleOrgCapabilities) *[]axiom.Action { return &c.Notifiers }},
	{"rbac", "Manage roles and groups", []string{Create, Read, Update, Delete}, func(c *axiom.RoleOrgCapabilities) *[]axiom.Action { return &c.RBAC }},
	{"shared_access_keys", "Manage shared access keys", []string{Read, Update}, func(c *axiom.RoleOrgCapabilities) *[]axiom.Action { return &c.SharedAccessKeys }},
	{"users", "Manage users", []string{Create, Read, Update, Delete}, func(c *axiom.RoleOrgCapabilities) *[]axiom.Action { return &c.Users }},
	{"views", "Manage views", []string{Create, Read, Update, Delete}, func(c *axiom.RoleOrgCapabilities) *[]axiom.Action { return &c.Views }},
}

var emptyStringSet = types.SetValueMust(types.StringType, []attr.Value{})

func roleCapabilityAttributes[T any](capabilities []roleCapability[T]) map[string]schema.Attribute {
	attributes := make(map[string]schema.Attribute, len(capabilities))
	for _, c := range capabilities {
		attributes[c.name] = schema.SetAttribute{
			Optional:            true,
			Computed:            true,
			ElementType:         types.StringType,
			MarkdownDescription: fmt.Sprintf("%s. Allowed actions: `%s`.", c.description, strings.Join(c.actions, "`, `")),
			Validators: []validator.Set{
				setvalidator.ValueStringsAre(stringvalidator.OneOf(c.actions...)),
			},
			Default: setdefault.StaticValue(emptyStringSet),
		}
	}
	return attributes
}

func roleCapabilityTypes[T any](capabilities []roleCapability[T]) map[string]attr.Type {
	attrTypes := make(map[string]attr.Type, len(capabilities))
	for _, c := range capabilities {
		attrTypes[c.name] = types.SetType{ElemType: types.StringType}
	}
	return attrTypes
}

func (r *RoleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_role"
}

func (r *RoleResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	orgCapabilityTypes := roleCapabilityTypes(roleOrgCapabilities)
	emptyOrgCapabilities := make(map[string]attr.Value, len(orgCapabilityTypes))
	for name := range orgCapabilityTypes {
		emptyOrgCapabilities[name] = emptyStringSet
	}

	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages a custom role. A role grants capabilities on individual datasets and views, and across the organization. " +
			"Assign a role to users with `axiom_group`, or set it as a user's base role with `axiom_user_role`. Requires the RBAC add-on.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Role identifier",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Role name. Must be unique in the organization and must not start with `axiom-`.",
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"description": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Role description",
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"dataset_capabilities": schema.MapNestedAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Capabilities on individual datasets, keyed by dataset name. The key `*` applies to all datasets. Each entry must have at least one capability.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: roleCapabilityAttributes(roleDatasetCapabilities),
				},
				Default: mapdefault.StaticValue(types.MapValueMust(
					types.ObjectType{AttrTypes: roleCapabilityTypes(roleDatasetCapabilities)},
					map[string]attr.Value{},
				)),
			},
			"view_capabilities": schema.MapNestedAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Capabilities on individual views, keyed by view name. Each entry must have at least one capability. The wildcard `*` isn't allowed.",
				NestedObject: schema.NestedAttributeObject{
					Attributes: roleCapabilityAttributes(roleViewCapabilities),
				},
				Validators: []validator.Map{
					mapvalidator.KeysAre(stringvalidator.NoneOf("*")),
				},
				Default: mapdefault.StaticValue(types.MapValueMust(
					types.ObjectType{AttrTypes: roleCapabilityTypes(roleViewCapabilities)},
					map[string]attr.Value{},
				)),
			},
			"org_capabilities": schema.SingleNestedAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Organization-wide capabilities",
				Attributes:          roleCapabilityAttributes(roleOrgCapabilities),
				Default: objectdefault.StaticValue(types.ObjectValueMust(
					orgCapabilityTypes,
					emptyOrgCapabilities,
				)),
			},
			"members": schema.SetAttribute{
				Computed:    true,
				ElementType: types.StringType,
				MarkdownDescription: "IDs of the users whose base role is this role. Users who receive the role through a group aren't included. " +
					"Change a user's base role with `axiom_user_role`.",
			},
		},
	}
}

func (r *RoleResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// ValidateConfig rejects dataset and view entries without capabilities. The
// API drops such entries, which would make the applied state differ from the
// configuration.
func (r *RoleResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var config RoleResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(validateRoleCapabilityMap(config.DatasetCapabilities, path.Root("dataset_capabilities"), "dataset")...)
	resp.Diagnostics.Append(validateRoleCapabilityMap(config.ViewCapabilities, path.Root("view_capabilities"), "view")...)
}

func validateRoleCapabilityMap(m types.Map, p path.Path, kind string) diag.Diagnostics {
	var diags diag.Diagnostics
	if m.IsNull() || m.IsUnknown() {
		return diags
	}

	for key, value := range m.Elements() {
		obj, ok := value.(types.Object)
		if !ok || obj.IsUnknown() {
			continue
		}

		hasCapability := false
		for _, v := range obj.Attributes() {
			set, ok := v.(types.Set)
			if !ok || set.IsUnknown() || (!set.IsNull() && len(set.Elements()) > 0) {
				hasCapability = true
				break
			}
		}

		if !hasCapability {
			diags.AddAttributeError(
				p.AtMapKey(key),
				"Missing capability",
				fmt.Sprintf("The %s %q must have at least one capability. Remove the entry to grant no access.", kind, key),
			)
		}
	}

	return diags
}

func (r *RoleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan RoleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.client == nil {
		resp.Diagnostics.AddError("Client Error", "Client is not set")
		return
	}

	roleReq, diags := expandRoleRequest(plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	role, err := r.client.Roles.Create(ctx, roleReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create role, got error: %s", err))
		return
	}

	state, diags := flattenRole(role)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *RoleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state RoleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	role, err := r.client.Roles.Get(ctx, state.ID.ValueString())
	if err != nil {
		if isNotFoundError(err) {
			resp.Diagnostics.AddWarning(
				"Role Not Found",
				fmt.Sprintf("Role with ID %s does not exist and will be recreated if still defined in the configuration.", state.ID.ValueString()),
			)
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read role", err.Error())
		return
	}

	newState, diags := flattenRole(role)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, newState)...)
}

func (r *RoleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan RoleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	roleReq, diags := expandRoleRequest(plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	role, err := r.client.Roles.Update(ctx, plan.ID.ValueString(), roleReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update role, got error: %s", err))
		return
	}

	state, diags := flattenRole(role)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *RoleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state RoleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.Roles.Delete(ctx, state.ID.ValueString()); err != nil {
		if isNotFoundError(err) {
			return
		}
		resp.Diagnostics.AddError("Failed to delete role", err.Error())
		return
	}
}

func (r *RoleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func expandRoleRequest(plan RoleResourceModel) (axiom.RoleRequest, diag.Diagnostics) {
	var diags diag.Diagnostics

	datasetCapabilities, d := expandRoleCapabilityMap(plan.DatasetCapabilities, roleDatasetCapabilities)
	diags.Append(d...)

	viewCapabilities, d := expandRoleCapabilityMap(plan.ViewCapabilities, roleViewCapabilities)
	diags.Append(d...)

	var orgCapabilities axiom.RoleOrgCapabilities
	if !plan.OrgCapabilities.IsNull() && !plan.OrgCapabilities.IsUnknown() {
		orgCapabilities, d = expandRoleCapabilities(plan.OrgCapabilities, roleOrgCapabilities)
		diags.Append(d...)
	}

	return axiom.RoleRequest{
		Name:                plan.Name.ValueString(),
		Description:         plan.Description.ValueString(),
		DatasetCapabilities: datasetCapabilities,
		ViewCapabilities:    viewCapabilities,
		OrgCapabilities:     orgCapabilities,
	}, diags
}

func expandRoleCapabilityMap[T any](m types.Map, capabilities []roleCapability[T]) (map[string]T, diag.Diagnostics) {
	var diags diag.Diagnostics
	if m.IsNull() || m.IsUnknown() {
		return nil, diags
	}

	result := make(map[string]T, len(m.Elements()))
	for key, value := range m.Elements() {
		obj, ok := value.(types.Object)
		if !ok {
			diags.AddError("Unexpected capability type", fmt.Sprintf("Expected an object for %q, got %T", key, value))
			continue
		}

		c, d := expandRoleCapabilities(obj, capabilities)
		diags.Append(d...)
		result[key] = c
	}

	return result, diags
}

func expandRoleCapabilities[T any](obj types.Object, capabilities []roleCapability[T]) (T, diag.Diagnostics) {
	var (
		result T
		diags  diag.Diagnostics
	)

	attributes := obj.Attributes()
	for _, c := range capabilities {
		set, ok := attributes[c.name].(types.Set)
		if !ok || set.IsNull() || set.IsUnknown() {
			continue
		}

		var actions []axiom.Action
		for _, v := range set.Elements() {
			s, ok := v.(types.String)
			if !ok {
				diags.AddError("Unexpected action type", fmt.Sprintf("Expected a string in %q, got %T", c.name, v))
				continue
			}

			action, err := axiomActionFromString(s.ValueString())
			if err != nil {
				diags.AddError("Invalid action", fmt.Sprintf("%s: %s", c.name, err))
				continue
			}
			actions = append(actions, action)
		}

		*c.field(&result) = actions
	}

	return result, diags
}

func flattenRole(role *axiom.Role) (RoleResourceModel, diag.Diagnostics) {
	var diags diag.Diagnostics

	datasetCapabilities, d := flattenRoleCapabilityMap(role.DatasetCapabilities, roleDatasetCapabilities)
	diags.Append(d...)

	viewCapabilities, d := flattenRoleCapabilityMap(role.ViewCapabilities, roleViewCapabilities)
	diags.Append(d...)

	orgCapabilities, d := flattenRoleCapabilities(role.OrgCapabilities, roleOrgCapabilities)
	diags.Append(d...)

	members, d := flattenStringSet(role.Members)
	diags.Append(d...)

	description := types.StringNull()
	if role.Description != "" {
		description = types.StringValue(role.Description)
	}

	return RoleResourceModel{
		ID:                  types.StringValue(role.ID),
		Name:                types.StringValue(role.Name),
		Description:         description,
		DatasetCapabilities: datasetCapabilities,
		ViewCapabilities:    viewCapabilities,
		OrgCapabilities:     orgCapabilities,
		Members:             members,
	}, diags
}

func flattenRoleCapabilityMap[T any](m map[string]T, capabilities []roleCapability[T]) (types.Map, diag.Diagnostics) {
	var diags diag.Diagnostics
	objectType := types.ObjectType{AttrTypes: roleCapabilityTypes(capabilities)}

	elements := make(map[string]attr.Value, len(m))
	for key, c := range m {
		obj, d := flattenRoleCapabilities(c, capabilities)
		diags.Append(d...)
		elements[key] = obj
	}

	result, d := types.MapValue(objectType, elements)
	diags.Append(d...)

	return result, diags
}

func flattenRoleCapabilities[T any](value T, capabilities []roleCapability[T]) (types.Object, diag.Diagnostics) {
	var diags diag.Diagnostics

	attributes := make(map[string]attr.Value, len(capabilities))
	for _, c := range capabilities {
		actions := *c.field(&value)

		names := make([]string, 0, len(actions))
		for _, a := range actions {
			names = append(names, a.String())
		}

		set, d := flattenStringSet(names)
		diags.Append(d...)
		attributes[c.name] = set
	}

	result, d := types.ObjectValue(roleCapabilityTypes(capabilities), attributes)
	diags.Append(d...)

	return result, diags
}

// flattenStringSet converts values to a set, dropping duplicates. A nil slice
// becomes an empty set.
func flattenStringSet(values []string) (types.Set, diag.Diagnostics) {
	elements := make([]attr.Value, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, v := range values {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		elements = append(elements, types.StringValue(v))
	}

	return types.SetValue(types.StringType, elements)
}
