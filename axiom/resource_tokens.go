package axiom

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/axiomhq/axiom-go/axiom"
)

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ resource.Resource                = &TokenResource{}
	_ resource.ResourceWithImportState = &TokenResource{}
)

const (
	Create = "create"
	Read   = "read"
	Update = "update"
	Delete = "delete"
)

func NewTokenResource() resource.Resource {
	return &TokenResource{}
}

// TokenResource defines the resource implementation.
type TokenResource struct {
	client *axiom.Client
}

// TokensResourceModel describes the resource data model.
type TokensResourceModel struct {
	ID                  types.String    `tfsdk:"id"`
	Name                types.String    `tfsdk:"name"`
	Description         types.String    `tfsdk:"description"`
	ExpiresAt           types.String    `tfsdk:"expires_at"`
	DatasetCapabilities types.Map       `tfsdk:"dataset_capabilities"`
	OrgCapabilities     OrgCapabilities `tfsdk:"org_capabilities"`
	Token               types.String    `tfsdk:"token"`
}

type DatasetCapabilities struct {
	Ingest         types.List `tfsdk:"ingest"`
	Query          types.List `tfsdk:"query"`
	StarredQueries types.List `tfsdk:"starred_queries"`
	VirtualFields  types.List `tfsdk:"virtual_fields"`
}

func (m DatasetCapabilities) Types() map[string]attr.Type {
	return map[string]attr.Type{
		"ingest":          types.ListType{ElemType: types.StringType},
		"query":           types.ListType{ElemType: types.StringType},
		"starred_queries": types.ListType{ElemType: types.StringType},
		"virtual_fields":  types.ListType{ElemType: types.StringType},
	}
}

type OrgCapabilities struct {
	Annotations      types.List `tfsdk:"annotations"`
	APITokens        types.List `tfsdk:"api_tokens"`
	Billing          types.List `tfsdk:"billing"`
	Dashboards       types.List `tfsdk:"dashboards"`
	Datasets         types.List `tfsdk:"datasets"`
	Endpoints        types.List `tfsdk:"endpoints"`
	Flows            types.List `tfsdk:"flows"`
	Integrations     types.List `tfsdk:"integrations"`
	Monitors         types.List `tfsdk:"monitors"`
	Notifiers        types.List `tfsdk:"notifiers"`
	Rbac             types.List `tfsdk:"rbac"`
	SharedAccessKeys types.List `tfsdk:"shared_access_keys"`
	Users            types.List `tfsdk:"users"`
}

func (r *TokenResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_token"
}

func (r *TokenResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Version: 1,
		Attributes: map[string]schema.Attribute{
			"token": schema.StringAttribute{
				Computed:    true,
				Sensitive:   true,
				Description: "The token value to be used in API calls",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The unique identifier of the token to be used when interacting with the token",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "The name of the token",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"description": schema.StringAttribute{
				Optional:    true,
				Description: "The description of the token",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"expires_at": schema.StringAttribute{
				Optional:    true,
				Description: "The time when the token expires. If not set, the token will not expire. Must be in RFC3339 format",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"dataset_capabilities": schema.MapNestedAttribute{
				MarkdownDescription: "The capabilities available to the token for each dataset",
				Optional:            true,
				PlanModifiers: []planmodifier.Map{
					mapplanmodifier.RequiresReplace(),
				},
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"ingest": schema.ListAttribute{
							Optional:    true,
							ElementType: types.StringType,
							Description: "Ability to ingest into the specified dataset",
							Validators: []validator.List{
								listvalidator.ValueStringsAre(
									stringvalidator.OneOf(Create),
								),
							},
							PlanModifiers: []planmodifier.List{
								listplanmodifier.RequiresReplace(),
							},
						},
						"query": schema.ListAttribute{
							Optional:    true,
							ElementType: types.StringType,
							Description: "Ability to query the specified dataset",
							Validators: []validator.List{
								listvalidator.ValueStringsAre(
									stringvalidator.OneOf(Read),
								),
							},
							PlanModifiers: []planmodifier.List{
								listplanmodifier.RequiresReplace(),
							},
						},
						"starred_queries": schema.ListAttribute{
							Optional:    true,
							ElementType: types.StringType,
							Description: "Ability to perform actions on starred queries for the specified dataset",
							Validators: []validator.List{
								listvalidator.ValueStringsAre(
									stringvalidator.OneOf(Create, Read, Update, Delete),
								),
							},
							PlanModifiers: []planmodifier.List{
								listplanmodifier.RequiresReplace(),
							},
						},
						"virtual_fields": schema.ListAttribute{
							Optional:    true,
							ElementType: types.StringType,
							Description: "Ability to perform actions on virtual fields for the provided dataset",
							Validators: []validator.List{
								listvalidator.ValueStringsAre(
									stringvalidator.OneOf(Create, Read, Update, Delete),
								),
							},
							PlanModifiers: []planmodifier.List{
								listplanmodifier.RequiresReplace(),
							},
						},
					},
				},
			},
			"org_capabilities": schema.SingleNestedAttribute{
				Description: "The organisation capabilities available to the token",
				Optional:    true,
				Attributes: map[string]schema.Attribute{
					"annotations": schema.ListAttribute{
						Optional:    true,
						ElementType: types.StringType,
						Description: "Ability to perform actions on annotations",
						Validators: []validator.List{
							listvalidator.ValueStringsAre(
								stringvalidator.OneOf(Create, Read, Update, Delete),
							),
						},
						PlanModifiers: []planmodifier.List{
							listplanmodifier.RequiresReplace(),
						},
					},
					"api_tokens": schema.ListAttribute{
						Optional:    true,
						ElementType: types.StringType,
						Description: "Ability to manage api tokens",
						Validators: []validator.List{
							listvalidator.ValueStringsAre(
								stringvalidator.OneOf(Create, Read, Update, Delete),
							),
						},
						PlanModifiers: []planmodifier.List{
							listplanmodifier.RequiresReplace(),
						},
					},
					"billing": schema.ListAttribute{
						Optional:    true,
						ElementType: types.StringType,
						Description: "Ability to manage billing information",
						Validators: []validator.List{
							listvalidator.ValueStringsAre(
								stringvalidator.OneOf(Read, Update),
							),
						},
						PlanModifiers: []planmodifier.List{
							listplanmodifier.RequiresReplace(),
						},
					},
					"dashboards": schema.ListAttribute{
						Optional:    true,
						ElementType: types.StringType,
						Description: "Ability to manage dashboards",
						Validators: []validator.List{
							listvalidator.ValueStringsAre(
								stringvalidator.OneOf(Create, Read, Update, Delete),
							),
						},
						PlanModifiers: []planmodifier.List{
							listplanmodifier.RequiresReplace(),
						},
					},
					"datasets": schema.ListAttribute{
						Optional:    true,
						ElementType: types.StringType,
						Description: "Ability to manage datasets",
						Validators: []validator.List{
							listvalidator.ValueStringsAre(
								stringvalidator.OneOf(Create, Read, Update, Delete),
							),
						},
						PlanModifiers: []planmodifier.List{
							listplanmodifier.RequiresReplace(),
						},
					},
					"endpoints": schema.ListAttribute{
						Optional:    true,
						ElementType: types.StringType,
						Description: "Ability to manage endpoints",
						Validators: []validator.List{
							listvalidator.ValueStringsAre(
								stringvalidator.OneOf(Create, Read, Update, Delete),
							),
						},
						PlanModifiers: []planmodifier.List{
							listplanmodifier.RequiresReplace(),
						},
					},
					"flows": schema.ListAttribute{
						Optional:    true,
						ElementType: types.StringType,
						Description: "Ability to manage flows",
						Validators: []validator.List{
							listvalidator.ValueStringsAre(
								stringvalidator.OneOf(Create, Read, Update, Delete),
							),
						},
						PlanModifiers: []planmodifier.List{
							listplanmodifier.RequiresReplace(),
						},
					},
					"integrations": schema.ListAttribute{
						Optional:    true,
						ElementType: types.StringType,
						Description: "Ability to manage integrations",
						Validators: []validator.List{
							listvalidator.ValueStringsAre(
								stringvalidator.OneOf(Create, Read, Update, Delete),
							),
						},
						PlanModifiers: []planmodifier.List{
							listplanmodifier.RequiresReplace(),
						},
					},
					"monitors": schema.ListAttribute{
						Optional:    true,
						ElementType: types.StringType,
						Description: "Ability to manage monitors",
						Validators: []validator.List{
							listvalidator.ValueStringsAre(
								stringvalidator.OneOf(Create, Read, Update, Delete),
							),
						},
						PlanModifiers: []planmodifier.List{
							listplanmodifier.RequiresReplace(),
						},
					},
					"notifiers": schema.ListAttribute{
						Optional:    true,
						ElementType: types.StringType,
						Description: "Ability to manage notifiers",
						Validators: []validator.List{
							listvalidator.ValueStringsAre(
								stringvalidator.OneOf(Create, Read, Update, Delete),
							),
						},
						PlanModifiers: []planmodifier.List{
							listplanmodifier.RequiresReplace(),
						},
					},
					"rbac": schema.ListAttribute{
						Optional:    true,
						ElementType: types.StringType,
						Description: "Ability to manage roles and groups",
						Validators: []validator.List{
							listvalidator.ValueStringsAre(
								stringvalidator.OneOf(Create, Read, Update, Delete),
							),
						},
						PlanModifiers: []planmodifier.List{
							listplanmodifier.RequiresReplace(),
						},
					},
					"shared_access_keys": schema.ListAttribute{
						Optional:    true,
						ElementType: types.StringType,
						Description: "Ability to manage shared access keys",
						Validators: []validator.List{
							listvalidator.ValueStringsAre(
								stringvalidator.OneOf(Read, Update),
							),
						},
						PlanModifiers: []planmodifier.List{
							listplanmodifier.RequiresReplace(),
						},
					},
					"users": schema.ListAttribute{
						Optional:    true,
						ElementType: types.StringType,
						Description: "Ability to manage users",
						Validators: []validator.List{
							listvalidator.ValueStringsAre(
								stringvalidator.OneOf(Create, Read, Update, Delete),
							),
						},
						PlanModifiers: []planmodifier.List{
							listplanmodifier.RequiresReplace(),
						},
					},
				},
			},
		},
	}
}

func (r *TokenResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*axiom.Client)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *http.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	r.client = client
}

func (r *TokenResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan TokensResourceModel

	// Read Terraform plan into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if r.client == nil {
		resp.Diagnostics.AddError("Client Error", "Client is not set")
		return
	}

	datasetCapabilities, diags := extractDatasetCapabilities(ctx, plan)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	orgCapabilities, diags := extractOrgCapabilities(ctx, plan)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	tokenExpiry := time.Time{}
	var err error
	if !plan.ExpiresAt.IsNull() {
		tokenExpiry, err = time.Parse(time.RFC3339, plan.ExpiresAt.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to parse token expiry time, got error: %s", err))
			return
		}
		tokenExpiry = tokenExpiry.UTC()
	}

	token, err := r.client.Tokens.Create(ctx, axiom.CreateTokenRequest{
		Name:                     plan.Name.ValueString(),
		Description:              plan.Description.ValueString(),
		ExpiresAt:                tokenExpiry,
		DatasetCapabilities:      datasetCapabilities,
		OrganisationCapabilities: *orgCapabilities,
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create token, got error: %s", err))
		return
	}

	createTokenResponse, diagnostics := flattenCreateTokenResponse(token)
	if diagnostics.HasError() {
		resp.Diagnostics.Append(diagnostics...)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, createTokenResponse)...)
}

func (r *TokenResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var plan TokensResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &plan)...)

	if resp.Diagnostics.HasError() {
		return
	}

	apiToken, err := r.client.Tokens.Get(ctx, plan.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to read token", err.Error())
		return
	}

	token, diagnostics := flattenToken(apiToken)
	if diagnostics.HasError() {
		resp.Diagnostics.Append(diagnostics...)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, token)...)
}

func (r *TokenResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Update not supported", "Tokens cannot be updated. Please delete and recreate the token.")
}

func (r *TokenResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var plan TokensResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &plan)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.Tokens.Delete(ctx, plan.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Failed to delete token", err.Error())
		return
	}
}

func (r *TokenResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func flattenToken(token *axiom.APIToken) (TokensResourceModel, diag.Diagnostics) {
	dsCapabilities, diags := flattenDatasetCapabilities(context.Background(), token.DatasetCapabilities)
	if diags.HasError() {
		return TokensResourceModel{}, diags
	}

	t := TokensResourceModel{
		ID:                  types.StringValue(token.ID),
		Name:                types.StringValue(token.Name),
		Description:         types.StringValue(token.Description),
		DatasetCapabilities: dsCapabilities,
		OrgCapabilities:     flattenOrgCapabilities(token.OrganisationCapabilities),
	}

	if !token.ExpiresAt.IsZero() {
		t.ExpiresAt = types.StringValue(token.ExpiresAt.UTC().Format(time.RFC3339))
	}
	return t, nil
}

func flattenCreateTokenResponse(token *axiom.CreateTokenResponse) (TokensResourceModel, diag.Diagnostics) {
	dsCapabilities, diags := flattenDatasetCapabilities(context.Background(), token.DatasetCapabilities)
	if diags.HasError() {
		return TokensResourceModel{}, diags
	}

	t := TokensResourceModel{
		ID:                  types.StringValue(token.ID),
		Name:                types.StringValue(token.Name),
		Description:         types.StringValue(token.Description),
		DatasetCapabilities: dsCapabilities,
		OrgCapabilities:     flattenOrgCapabilities(token.OrganisationCapabilities),
		Token:               types.StringValue(token.Token),
	}

	if !token.ExpiresAt.IsZero() {
		t.ExpiresAt = types.StringValue(token.ExpiresAt.Format(time.RFC3339))
	}
	return t, nil
}

func flattenOrgCapabilities(orgCapabilities axiom.OrganisationCapabilities) OrgCapabilities {
	return OrgCapabilities{
		Annotations:      flattenAxiomActionSlice(orgCapabilities.Annotations),
		APITokens:        flattenAxiomActionSlice(orgCapabilities.APITokens),
		Billing:          flattenAxiomActionSlice(orgCapabilities.Billing),
		Dashboards:       flattenAxiomActionSlice(orgCapabilities.Dashboards),
		Datasets:         flattenAxiomActionSlice(orgCapabilities.Datasets),
		Endpoints:        flattenAxiomActionSlice(orgCapabilities.Endpoints),
		Flows:            flattenAxiomActionSlice(orgCapabilities.Flows),
		Integrations:     flattenAxiomActionSlice(orgCapabilities.Integrations),
		Monitors:         flattenAxiomActionSlice(orgCapabilities.Monitors),
		Notifiers:        flattenAxiomActionSlice(orgCapabilities.Notifiers),
		Rbac:             flattenAxiomActionSlice(orgCapabilities.RBAC),
		SharedAccessKeys: flattenAxiomActionSlice(orgCapabilities.SharedAccessKeys),
		Users:            flattenAxiomActionSlice(orgCapabilities.Users),
	}
}

func flattenDatasetCapabilities(ctx context.Context, datasetCapabilities map[string]axiom.DatasetCapabilities) (types.Map, diag.Diagnostics) {
	dsCapabilities := map[string]DatasetCapabilities{}

	if len(datasetCapabilities) == 0 {
		return types.MapNull(types.ObjectType{
			AttrTypes: DatasetCapabilities{}.Types(),
		}), nil
	}

	for dataset, capabilities := range datasetCapabilities {
		dsCapabilities[dataset] = DatasetCapabilities{
			Ingest:         flattenAxiomActionSlice(capabilities.Ingest),
			Query:          flattenAxiomActionSlice(capabilities.Query),
			StarredQueries: flattenAxiomActionSlice(capabilities.StarredQueries),
			VirtualFields:  flattenAxiomActionSlice(capabilities.VirtualFields),
		}
	}

	t, dg := types.MapValueFrom(
		ctx,
		types.ObjectType{
			AttrTypes: DatasetCapabilities{}.Types(),
		},
		dsCapabilities,
	)
	if dg.HasError() {
		return types.Map{}, dg
	}
	return t, nil
}

func extractDatasetCapabilities(ctx context.Context, plan TokensResourceModel) (map[string]axiom.DatasetCapabilities, diag.Diagnostics) {
	datasetCapabilities := map[string]axiom.DatasetCapabilities{}

	dsCapabilities := map[string]DatasetCapabilities{}

	dg := plan.DatasetCapabilities.ElementsAs(ctx, &dsCapabilities, false)
	if dg.HasError() {
		return nil, dg
	}

	for name, capabilities := range dsCapabilities {
		dc := axiom.DatasetCapabilities{}

		values, diags := typeAxiomActionSliceToStringSlice(ctx, capabilities.Ingest.Elements())
		if diags.HasError() {
			return nil, diags
		}
		dc.Ingest = values

		values, diags = typeAxiomActionSliceToStringSlice(ctx, capabilities.Query.Elements())
		if diags.HasError() {
			return nil, diags
		}
		dc.Query = values

		values, diags = typeAxiomActionSliceToStringSlice(ctx, capabilities.StarredQueries.Elements())
		if diags.HasError() {
			return nil, diags
		}
		dc.StarredQueries = values

		values, diags = typeAxiomActionSliceToStringSlice(ctx, capabilities.VirtualFields.Elements())
		if diags.HasError() {
			return nil, diags
		}
		dc.VirtualFields = values

		datasetCapabilities[name] = dc
	}

	return datasetCapabilities, nil
}

func extractOrgCapabilities(ctx context.Context, plan TokensResourceModel) (*axiom.OrganisationCapabilities, diag.Diagnostics) {
	orgCapabilities := &axiom.OrganisationCapabilities{}
	values, diags := typeAxiomActionSliceToStringSlice(ctx, plan.OrgCapabilities.Annotations.Elements())
	if diags.HasError() {
		return nil, diags
	}
	orgCapabilities.Annotations = values

	values, diags = typeAxiomActionSliceToStringSlice(ctx, plan.OrgCapabilities.APITokens.Elements())
	if diags.HasError() {
		return nil, diags
	}
	orgCapabilities.APITokens = values

	values, diags = typeAxiomActionSliceToStringSlice(ctx, plan.OrgCapabilities.Billing.Elements())
	if diags.HasError() {
		return nil, diags
	}
	orgCapabilities.Billing = values

	values, diags = typeAxiomActionSliceToStringSlice(ctx, plan.OrgCapabilities.Dashboards.Elements())
	if diags.HasError() {
		return nil, diags
	}
	orgCapabilities.Dashboards = values

	values, diags = typeAxiomActionSliceToStringSlice(ctx, plan.OrgCapabilities.Datasets.Elements())
	if diags.HasError() {
		return nil, diags
	}
	orgCapabilities.Datasets = values

	values, diags = typeAxiomActionSliceToStringSlice(ctx, plan.OrgCapabilities.Endpoints.Elements())
	if diags.HasError() {
		return nil, diags
	}
	orgCapabilities.Endpoints = values

	values, diags = typeAxiomActionSliceToStringSlice(ctx, plan.OrgCapabilities.Flows.Elements())
	if diags.HasError() {
		return nil, diags
	}
	orgCapabilities.Flows = values

	values, diags = typeAxiomActionSliceToStringSlice(ctx, plan.OrgCapabilities.Integrations.Elements())
	if diags.HasError() {
		return nil, diags
	}
	orgCapabilities.Integrations = values

	values, diags = typeAxiomActionSliceToStringSlice(ctx, plan.OrgCapabilities.Monitors.Elements())
	if diags.HasError() {
		return nil, diags
	}
	orgCapabilities.Monitors = values

	values, diags = typeAxiomActionSliceToStringSlice(ctx, plan.OrgCapabilities.Notifiers.Elements())
	if diags.HasError() {
		return nil, diags
	}
	orgCapabilities.Notifiers = values

	values, diags = typeAxiomActionSliceToStringSlice(ctx, plan.OrgCapabilities.Rbac.Elements())
	if diags.HasError() {
		return nil, diags
	}
	orgCapabilities.RBAC = values

	values, diags = typeAxiomActionSliceToStringSlice(ctx, plan.OrgCapabilities.SharedAccessKeys.Elements())
	if diags.HasError() {
		return nil, diags
	}
	orgCapabilities.SharedAccessKeys = values

	values, diags = typeAxiomActionSliceToStringSlice(ctx, plan.OrgCapabilities.Users.Elements())
	if diags.HasError() {
		return nil, diags
	}
	orgCapabilities.Users = values

	return orgCapabilities, nil
}

func flattenAxiomActionSlice(values []axiom.Action) types.List {
	if len(values) == 0 {
		return types.ListNull(types.StringType)
	}

	listElements := make([]attr.Value, 0, len(values))
	for _, value := range values {
		columnElement := types.StringValue(value.String())
		listElements = append(listElements, columnElement)
	}

	return types.ListValueMust(types.StringType, listElements)
}

func typeAxiomActionSliceToStringSlice(ctx context.Context, s []attr.Value) ([]axiom.Action, diag.Diagnostics) {
	result := make([]axiom.Action, 0, len(s))
	var diags diag.Diagnostics
	for _, v := range s {
		val, err := v.ToTerraformValue(ctx)
		if err != nil {
			diags.AddError("Failed to convert value to Terraform", err.Error())
			continue
		}
		var str string
		if err = val.As(&str); err != nil {
			diags.AddError("Failed to convert value to Terraform", err.Error())
			continue
		}

		action, err := axiomActionFromString(str)
		if err != nil {
			diags.AddError("failed to convert string to action", err.Error())
			continue
		}

		result = append(result, action)
	}
	if diags.HasError() {
		return nil, diags
	}
	return result, nil
}

func axiomActionFromString(value string) (axiom.Action, error) {
	var action axiom.Action
	switch value {
	case Create:
		action = axiom.ActionCreate
	case Read:
		action = axiom.ActionRead
	case Update:
		action = axiom.ActionUpdate
	case Delete:
		action = axiom.ActionDelete
	default:
		return action, fmt.Errorf("invalid action: %s", value)
	}
	return action, nil
}
