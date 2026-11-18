package axiom

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/ephemeral"
	"github.com/hashicorp/terraform-plugin-framework/ephemeral/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/axiomhq/axiom-go/axiom"
)

var (
	_ ephemeral.EphemeralResource                   = &TokenEphemeralResource{}
	_ ephemeral.EphemeralResourceWithConfigure      = &TokenEphemeralResource{}
	_ ephemeral.EphemeralResourceWithValidateConfig = &TokenEphemeralResource{}
	_ ephemeral.EphemeralResourceWithClose          = &TokenEphemeralResource{}
)

// ephemeralTokenPrivateKey is the private data key that carries what Close
// needs to know about the token created by Open.
const ephemeralTokenPrivateKey = "token"

func NewTokenEphemeralResource() ephemeral.EphemeralResource {
	return &TokenEphemeralResource{}
}

// TokenEphemeralResource creates an API token whose value is never written to
// the Terraform state or plan.
type TokenEphemeralResource struct {
	client *axiom.Client
}

type TokenEphemeralResourceModel struct {
	ID                  types.String `tfsdk:"id"`
	Name                types.String `tfsdk:"name"`
	Description         types.String `tfsdk:"description"`
	ExpiresAt           types.String `tfsdk:"expires_at"`
	DeleteOnClose       types.Bool   `tfsdk:"delete_on_close"`
	DatasetCapabilities types.Map    `tfsdk:"dataset_capabilities"`
	OrgCapabilities     types.Object `tfsdk:"org_capabilities"`
	Token               types.String `tfsdk:"token"`
}

// deleteOnClose reports whether the token is deleted when Terraform closes
// the ephemeral resource. It defaults to true.
func (m TokenEphemeralResourceModel) deleteOnClose() bool {
	return m.DeleteOnClose.IsNull() || m.DeleteOnClose.ValueBool()
}

type ephemeralTokenPrivateData struct {
	ID            string `json:"id"`
	DeleteOnClose bool   `json:"delete_on_close"`
}

type tokenCapability struct {
	name        string
	description string
	actions     []string
}

var ephemeralTokenDatasetCapabilities = []tokenCapability{
	{"ingest", "Ability to ingest into the specified dataset", []string{Create}},
	{"query", "Ability to query the specified dataset", []string{Read}},
	{"starred_queries", "Ability to perform actions on starred queries for the specified dataset", []string{Create, Read, Update, Delete}},
	{"virtual_fields", "Ability to perform actions on virtual fields for the provided dataset", []string{Create, Read, Update, Delete}},
	{"data", "Ability to manage the data in a dataset", []string{Delete}},
	{"trim", "Ability to trim the data in a dataset", []string{Update}},
	{"vacuum", "Ability to vacuum the fields in a dataset", []string{Update}},
}

var ephemeralTokenOrgCapabilities = []tokenCapability{
	{"annotations", "Ability to perform actions on annotations", []string{Create, Read, Update, Delete}},
	{"api_tokens", "Ability to manage api tokens", []string{Create, Read, Update, Delete}},
	{"audit_log", "Ability to read the audit log", []string{Read}},
	{"billing", "Ability to manage billing information", []string{Read, Update}},
	{"dashboards", "Ability to manage dashboards", []string{Create, Read, Update, Delete}},
	{"datasets", "Ability to manage datasets", []string{Create, Read, Update, Delete}},
	{"endpoints", "Ability to manage endpoints", []string{Create, Read, Update, Delete}},
	{"flows", "Ability to manage flows", []string{Create, Read, Update, Delete}},
	{"integrations", "Ability to manage integrations", []string{Create, Read, Update, Delete}},
	{"monitors", "Ability to manage monitors", []string{Create, Read, Update, Delete}},
	{"notifiers", "Ability to manage notifiers", []string{Create, Read, Update, Delete}},
	{"rbac", "Ability to manage roles and groups", []string{Create, Read, Update, Delete}},
	{"shared_access_keys", "Ability to manage shared access keys", []string{Read, Update}},
	{"users", "Ability to manage users", []string{Create, Read, Update, Delete}},
}

func ephemeralTokenCapabilityAttributes(capabilities []tokenCapability) map[string]schema.Attribute {
	attributes := make(map[string]schema.Attribute, len(capabilities))
	for _, capability := range capabilities {
		attributes[capability.name] = schema.ListAttribute{
			Optional:    true,
			ElementType: types.StringType,
			Description: capability.description,
			Validators: []validator.List{
				listvalidator.ValueStringsAre(stringvalidator.OneOf(capability.actions...)),
			},
		}
	}
	return attributes
}

func (e *TokenEphemeralResource) Metadata(_ context.Context, req ephemeral.MetadataRequest, resp *ephemeral.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_token"
}

func (e *TokenEphemeralResource) Schema(_ context.Context, _ ephemeral.SchemaRequest, resp *ephemeral.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Creates an API token whose value is never written to the Terraform state or plan.\n\n" +
			"A new token is created every time Terraform opens the ephemeral resource, which happens during each plan and each apply. " +
			"By default the token is deleted when the run finishes, so it is only usable within that run, for example to configure another provider. " +
			"To hand the token to a secret store through a write-only argument, set `delete_on_close` to `false` together with `expires_at`: " +
			"tokens created by runs that do not store them then expire on their own.\n\n" +
			"Alternatively, set only `id` to the `id` of an `axiom_token_stateless` resource. No token is created in that case. " +
			"Instead, `token` carries the value of the managed token during the apply that creates or regenerates it, and is an empty string in every other run, " +
			"because the Axiom API returns a token value only once. Pass it to a write-only argument and change the version of that argument in the same apply that regenerates the token, " +
			"so that the value is only written when a new one was issued. If that write does not complete, regenerate the token again to issue a new value.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Optional: true,
				Computed: true,
				MarkdownDescription: "The unique identifier of the token. Set it to the `id` of an `axiom_token_stateless` resource to read the value of that token instead of creating a new one. " +
					"No other argument can be set together with `id`",
			},
			"token": schema.StringAttribute{
				Computed:            true,
				Sensitive:           true,
				MarkdownDescription: "The token value to be used in API calls. When `id` is set, this is an empty string unless the referenced `axiom_token_stateless` resource was created or regenerated during the same apply",
			},
			"name": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "The name of the token. Required unless `id` is set",
			},
			"description": schema.StringAttribute{
				Optional:    true,
				Description: "The description of the token",
			},
			"expires_at": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "The time when the token expires. Must be in RFC3339 format. Required when `delete_on_close` is `false`",
			},
			"delete_on_close": schema.BoolAttribute{
				Optional:            true,
				MarkdownDescription: "Whether the token is deleted when the Terraform run finishes. Defaults to `true`. When `false`, `expires_at` is required because every plan and apply leaves a token behind",
			},
			"dataset_capabilities": schema.MapNestedAttribute{
				Optional:            true,
				MarkdownDescription: "The capabilities available to the token for each dataset",
				NestedObject: schema.NestedAttributeObject{
					Attributes: ephemeralTokenCapabilityAttributes(ephemeralTokenDatasetCapabilities),
				},
			},
			"org_capabilities": schema.SingleNestedAttribute{
				Optional:    true,
				Description: "The organisation capabilities available to the token",
				Attributes:  ephemeralTokenCapabilityAttributes(ephemeralTokenOrgCapabilities),
			},
		},
	}
}

func (e *TokenEphemeralResource) Configure(_ context.Context, req ephemeral.ConfigureRequest, resp *ephemeral.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*axiom.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Ephemeral Resource Configure Type",
			fmt.Sprintf("Expected *axiom.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	e.client = client
}

func (e *TokenEphemeralResource) ValidateConfig(ctx context.Context, req ephemeral.ValidateConfigRequest, resp *ephemeral.ValidateConfigResponse) {
	var config TokenEphemeralResourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if config.ID.IsUnknown() || config.Name.IsUnknown() {
		return
	}

	if !config.ID.IsNull() {
		for name, isSet := range map[string]bool{
			"name":                 !config.Name.IsNull(),
			"description":          !config.Description.IsNull(),
			"expires_at":           !config.ExpiresAt.IsNull(),
			"delete_on_close":      !config.DeleteOnClose.IsNull(),
			"dataset_capabilities": !config.DatasetCapabilities.IsNull(),
			"org_capabilities":     !config.OrgCapabilities.IsNull(),
		} {
			if isSet {
				resp.Diagnostics.AddAttributeError(
					path.Root(name),
					"Conflicting Token Arguments",
					fmt.Sprintf("%s cannot be set together with id. With id, the ephemeral resource reads the value of an existing axiom_token_stateless resource and does not create a token.", name),
				)
			}
		}
		return
	}

	if config.Name.IsNull() {
		resp.Diagnostics.AddAttributeError(
			path.Root("name"),
			"Missing Token Name",
			"name is required to create a token. To read the value of an existing axiom_token_stateless resource instead, set id.",
		)
		return
	}

	if config.DeleteOnClose.IsUnknown() || config.ExpiresAt.IsUnknown() {
		return
	}

	if !config.deleteOnClose() && config.ExpiresAt.IsNull() {
		resp.Diagnostics.AddAttributeError(
			path.Root("expires_at"),
			"Missing Token Expiry",
			"expires_at is required when delete_on_close is false. A new token is created during every plan and apply, "+
				"so tokens that are not deleted must expire to avoid accumulating indefinitely.",
		)
	}
}

func (e *TokenEphemeralResource) Open(ctx context.Context, req ephemeral.OpenRequest, resp *ephemeral.OpenResponse) {
	var config TokenEphemeralResourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !config.ID.IsNull() {
		// The value only exists in this process if the axiom_token_stateless resource
		// issued it during this apply. An empty string, rather than null,
		// keeps write-only arguments that must be set valid in other runs.
		value, _ := tokenHandoff.get(config.ID.ValueString())
		config.Token = types.StringValue(value)

		resp.Diagnostics.Append(resp.Result.Set(ctx, config)...)
		return
	}

	if e.client == nil {
		resp.Diagnostics.AddError("Client Error", "Client is not set")
		return
	}

	datasetCapabilities := config.DatasetCapabilities
	if datasetCapabilities.IsNull() {
		datasetCapabilities = types.MapValueMust(
			types.ObjectType{AttrTypes: DatasetCapabilities{}.Types()},
			map[string]attr.Value{},
		)
	}

	tokenReq, diags := buildCreateTokenRequest(ctx, TokensResourceModel{
		Name:                config.Name,
		Description:         config.Description,
		ExpiresAt:           config.ExpiresAt,
		DatasetCapabilities: datasetCapabilities,
		OrgCapabilities:     config.OrgCapabilities,
	})
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	token, err := e.client.Tokens.Create(ctx, tokenReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create token, got error: %s", err))
		return
	}

	// Record the token for Close before anything else can fail, so a token
	// that was created is not left behind.
	privateData, err := json.Marshal(ephemeralTokenPrivateData{
		ID:            token.ID,
		DeleteOnClose: config.deleteOnClose(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to encode token private data, got error: %s", err))
		return
	}
	resp.Diagnostics.Append(resp.Private.SetKey(ctx, ephemeralTokenPrivateKey, privateData)...)

	config.ID = types.StringValue(token.ID)
	config.Token = types.StringValue(token.Token)

	resp.Diagnostics.Append(resp.Result.Set(ctx, config)...)
}

func (e *TokenEphemeralResource) Close(ctx context.Context, req ephemeral.CloseRequest, resp *ephemeral.CloseResponse) {
	raw, diags := req.Private.GetKey(ctx, ephemeralTokenPrivateKey)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() || len(raw) == 0 {
		return
	}

	var privateData ephemeralTokenPrivateData
	if err := json.Unmarshal(raw, &privateData); err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to decode token private data, got error: %s", err))
		return
	}

	if !privateData.DeleteOnClose || privateData.ID == "" {
		return
	}

	if e.client == nil {
		resp.Diagnostics.AddError("Client Error", "Client is not set")
		return
	}

	if err := e.client.Tokens.Delete(ctx, privateData.ID); err != nil && !isNotFoundError(err) {
		resp.Diagnostics.AddError(
			"Failed to delete token",
			fmt.Sprintf("The ephemeral token %s could not be deleted and remains valid until it expires or is deleted manually: %s", privateData.ID, err),
		)
	}
}
