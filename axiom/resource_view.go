package axiom

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/axiomhq/axiom-go/axiom"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = &ViewResource{}
	_ resource.ResourceWithConfigure   = &ViewResource{}
	_ resource.ResourceWithImportState = &ViewResource{}
)

func NewViewResource() resource.Resource { return &ViewResource{} }

type ViewResource struct{ client *axiom.Client }

type ViewResourceModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	APLQuery    types.String `tfsdk:"apl_query"`
	Datasets    types.Set    `tfsdk:"datasets"`
}

type viewRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	APLQuery    string `json:"aplQuery"`
}

type viewResponse struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	APLQuery    string   `json:"aplQuery"`
	Datasets    []string `json:"datasets"`
}

func (r *ViewResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_view"
}

func (r *ViewResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an Axiom view, a named APL query that can be queried like a dataset. " +
			"The API token needs permission to manage views and query the underlying datasets. " +
			"Views cannot reference other views, use dataset wildcards, or query datasets in different regions.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "View identifier, equal to its name. Use the name when importing an existing view.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "View name. Must not match an existing dataset or view. Changing the name replaces the view.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Validators:          []validator.String{stringvalidator.LengthAtLeast(1)},
			},
			"description": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString(""),
				MarkdownDescription: "View description. Removing it clears the description.",
			},
			"apl_query": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "APL query defining the view. Must reference at least one dataset.",
				Validators:          []validator.String{stringvalidator.LengthAtLeast(1)},
			},
			"datasets": schema.SetAttribute{
				Computed:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Datasets referenced by the view, derived by Axiom from apl_query.",
			},
		},
	}
}

func (r *ViewResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*axiom.Client)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Resource Configure Type", fmt.Sprintf("Expected *axiom.Client, got: %T.", req.ProviderData))
		return
	}
	r.client = client
}

func (r *ViewResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan ViewResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var result viewResponse
	if err := r.client.Call(ctx, http.MethodPost, "/v2/views", viewRequestFromModel(plan), &result); err != nil {
		resp.Diagnostics.AddError("Unable to create view", err.Error())
		return
	}
	state, diags := flattenView(result)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *ViewResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state ViewResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var result viewResponse
	if err := r.client.Call(ctx, http.MethodGet, "/v2/views/"+url.PathEscape(state.ID.ValueString()), nil, &result); err != nil {
		if isNotFoundError(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read view", err.Error())
		return
	}
	newState, diags := flattenView(result)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, newState)...)
}

func (r *ViewResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan ViewResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var result viewResponse
	if err := r.client.Call(ctx, http.MethodPut, "/v2/views/"+url.PathEscape(plan.ID.ValueString()), viewRequestFromModel(plan), &result); err != nil {
		resp.Diagnostics.AddError("Unable to update view", err.Error())
		return
	}
	state, diags := flattenView(result)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *ViewResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state ViewResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.Call(ctx, http.MethodDelete, "/v2/views/"+url.PathEscape(state.ID.ValueString()), nil, nil); err != nil && !isNotFoundError(err) {
		resp.Diagnostics.AddError("Unable to delete view", err.Error())
	}
}

func (r *ViewResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func viewRequestFromModel(plan ViewResourceModel) viewRequest {
	return viewRequest{
		Name:        plan.Name.ValueString(),
		Description: plan.Description.ValueString(),
		APLQuery:    plan.APLQuery.ValueString(),
	}
}

func flattenView(view viewResponse) (ViewResourceModel, diag.Diagnostics) {
	datasets, diags := flattenStringSet(view.Datasets)
	return ViewResourceModel{
		ID:          types.StringValue(view.Name),
		Name:        types.StringValue(view.Name),
		Description: types.StringValue(view.Description),
		APLQuery:    types.StringValue(view.APLQuery),
		Datasets:    datasets,
	}, diags
}
