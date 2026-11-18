package axiom

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/axiomhq/axiom-go/axiom"
)

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ resource.Resource                = &TokenStatelessResource{}
	_ resource.ResourceWithImportState = &TokenStatelessResource{}
	_ resource.ResourceWithModifyPlan  = &TokenStatelessResource{}
)

func NewTokenStatelessResource() resource.Resource {
	return &TokenStatelessResource{}
}

// TokenStatelessResource manages an API token without keeping its value in the
// Terraform state. It behaves like TokenResource in every other respect.
type TokenStatelessResource struct {
	client *axiom.Client
}

// TokenStatelessResourceModel is TokensResourceModel without the token value.
type TokenStatelessResourceModel struct {
	ID                  types.String `tfsdk:"id"`
	Name                types.String `tfsdk:"name"`
	Description         types.String `tfsdk:"description"`
	ExpiresAt           types.String `tfsdk:"expires_at"`
	RotationGracePeriod types.String `tfsdk:"rotation_grace_period"`
	DatasetCapabilities types.Map    `tfsdk:"dataset_capabilities"`
	OrgCapabilities     types.Object `tfsdk:"org_capabilities"`
}

func (m TokenStatelessResourceModel) tokenModel() TokensResourceModel {
	return TokensResourceModel{
		ID:                  m.ID,
		Name:                m.Name,
		Description:         m.Description,
		ExpiresAt:           m.ExpiresAt,
		RotationGracePeriod: m.RotationGracePeriod,
		DatasetCapabilities: m.DatasetCapabilities,
		OrgCapabilities:     m.OrgCapabilities,
	}
}

// tokenStatelessModel drops the token value, which must not reach the state.
func tokenStatelessModel(m TokensResourceModel) TokenStatelessResourceModel {
	return TokenStatelessResourceModel{
		ID:                  m.ID,
		Name:                m.Name,
		Description:         m.Description,
		ExpiresAt:           m.ExpiresAt,
		RotationGracePeriod: m.RotationGracePeriod,
		DatasetCapabilities: m.DatasetCapabilities,
		OrgCapabilities:     m.OrgCapabilities,
	}
}

func (r *TokenStatelessResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_token_stateless"
}

func (r *TokenStatelessResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	// Same arguments as axiom_token, without the attribute that holds the value.
	(&TokenResource{}).Schema(ctx, req, resp)

	delete(resp.Schema.Attributes, "token")
	resp.Schema.Version = 0
	resp.Schema.MarkdownDescription = "Manages an API token without writing its value to the Terraform state or plan. " +
		"It takes the same arguments as `axiom_token`, but has no `token` attribute.\n\n" +
		"This resource is meant to be used together with the `axiom_token` ephemeral resource, which is the only way to get the token value: " +
		"set the `id` of the ephemeral resource to the `id` of this resource, and pass its `token` to a write-only argument of the resource that stores the token. " +
		"Without the ephemeral resource the value cannot be retrieved, because the Axiom API returns a token value only once.\n\n" +
		"The value is only available during the apply that creates or regenerates the token. " +
		"Change the version of the write-only argument in that same apply. If the write does not complete, regenerate the token again to issue a new value.\n\n" +
		"Changing `name`, `description`, `expires_at` or the capabilities regenerates the token: " +
		"it gets a new `id` and value, and the previous token stays valid for `rotation_grace_period`."
}

func (r *TokenStatelessResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *TokenStatelessResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() {
		return
	}

	var state TokenStatelessResourceModel
	var plan TokenStatelessResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !tokenWillRegenerate(plan.tokenModel(), state.tokenModel()) {
		return
	}

	// Regenerating issues a new ID. Planning it as unknown defers the
	// ephemeral resource that reads the value until the token exists.
	resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("id"), types.StringUnknown())...)
}

func (r *TokenStatelessResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan TokenStatelessResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.client == nil {
		resp.Diagnostics.AddError("Client Error", "Client is not set")
		return
	}

	tokenReq, diags := buildCreateTokenRequest(ctx, plan.tokenModel())
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	token, err := r.client.Tokens.Create(ctx, tokenReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create token, got error: %s", err))
		return
	}

	resp.Diagnostics.Append(r.setIssuedToken(ctx, token, &resp.State)...)
}

func (r *TokenStatelessResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state TokenStatelessResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiToken, err := r.client.Tokens.Get(ctx, state.ID.ValueString())
	if err != nil {
		if isNotFoundError(err) {
			resp.Diagnostics.AddWarning(
				"Token Not Found",
				fmt.Sprintf("Token with ID %s does not exist and will be recreated if still defined in the configuration.", state.ID.ValueString()),
			)
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read token", err.Error())
		return
	}

	token, diags := flattenToken(ctx, apiToken)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, tokenStatelessModel(token))...)
}

func (r *TokenStatelessResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan TokenStatelessResourceModel
	var state TokenStatelessResourceModel
	var config TokenStatelessResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.client == nil {
		resp.Diagnostics.AddError("Client Error", "Client is not set")
		return
	}

	newTokenReq, diags := buildCreateTokenRequest(ctx, plan.tokenModel())
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	gracePeriod, diags := extractRotationGracePeriod(config.tokenModel())
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	rotatedToken, err := r.client.Tokens.Regenerate(ctx, state.ID.ValueString(), axiom.RegenerateTokenRequest{
		ExistingTokenExpiresAt: time.Now().UTC().Add(gracePeriod),
		NewToken:               &newTokenReq,
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to regenerate token, got error: %s", err))
		return
	}

	resp.Diagnostics.Append(r.setIssuedToken(ctx, rotatedToken, &resp.State)...)
}

func (r *TokenStatelessResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state TokenStatelessResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.Tokens.Delete(ctx, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Failed to delete token", err.Error())
		return
	}
}

func (r *TokenStatelessResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

// setIssuedToken records a token that was just created or regenerated. The
// value goes to the ephemeral resource and everything else to the state.
func (r *TokenStatelessResource) setIssuedToken(ctx context.Context, token *axiom.CreateTokenResponse, state *tfsdk.State) diag.Diagnostics {
	issued, diags := flattenCreateTokenResponse(ctx, token)
	if diags.HasError() {
		return diags
	}

	tokenHandoff.put(issued.ID.ValueString(), issued.Token.ValueString())

	diags.Append(state.Set(ctx, tokenStatelessModel(issued))...)
	return diags
}
