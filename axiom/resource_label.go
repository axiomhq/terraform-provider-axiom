package axiom

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"

	"github.com/axiomhq/axiom-go/axiom"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
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
	_ resource.Resource                = &LabelResource{}
	_ resource.ResourceWithConfigure   = &LabelResource{}
	_ resource.ResourceWithImportState = &LabelResource{}
)

func NewLabelResource() resource.Resource { return &LabelResource{} }

type LabelResource struct{ client *axiom.Client }

type LabelResourceModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	Color       types.String `tfsdk:"color"`
}

type labelColor struct {
	Type string `json:"type"`
	Name string `json:"name"`
}

type labelRequest struct {
	Name        string     `json:"name"`
	Description string     `json:"description"`
	Color       labelColor `json:"color"`
}

type labelResponse struct {
	ID                string          `json:"id"`
	Name              string          `json:"name"`
	Description       string          `json:"description"`
	Color             labelColor      `json:"color"`
	ResourcesCount    json.RawMessage `json:"resources_count"`
	AssignedResources json.RawMessage `json:"assigned_resources"`
}

func (r *LabelResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_label"
}

func (r *LabelResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an organization-level Axiom label. Reference its ID in dashboard, monitor, or dataset `labels` sets. Deleting a label also removes its assignments.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Label identifier.",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Label name, unique within the organization (case-insensitive). Use single spaces between words and no leading or trailing whitespace.",
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 128),
					stringvalidator.RegexMatches(regexp.MustCompile(`^[^\s\p{Z}\p{Cc}]+(?: [^\s\p{Z}\p{Cc}]+)*$`), "Use single spaces between words, no leading or trailing whitespace, and no control characters."),
				},
			},
			"description": schema.StringAttribute{
				Optional: true, Computed: true,
				Default:             stringdefault.StaticString(""),
				MarkdownDescription: "Label description, up to 512 characters. Removing it clears the description.",
				Validators:          []validator.String{stringvalidator.LengthAtMost(512)},
			},
			"color": schema.StringAttribute{
				Optional: true, Computed: true,
				Default:             stringdefault.StaticString("blue"),
				MarkdownDescription: "Palette color: red, coral, orange, amber, yellow, lime, green, teal, blue, iris, purple, or pink. Defaults to blue; removing it resets the color to blue.",
				Validators:          []validator.String{stringvalidator.OneOf("red", "coral", "orange", "amber", "yellow", "lime", "green", "teal", "blue", "iris", "purple", "pink")},
			},
		},
	}
}

func (r *LabelResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *LabelResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan LabelResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var result labelResponse
	if err := r.client.Call(ctx, http.MethodPost, "/v2/labels", labelRequestFromModel(plan), &result); err != nil {
		resp.Diagnostics.AddError("Unable to create label", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, flattenLabel(result))...)
}

func (r *LabelResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state LabelResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var result labelResponse
	if err := r.client.Call(ctx, http.MethodGet, "/v2/labels/"+url.PathEscape(state.ID.ValueString()), nil, &result); err != nil {
		if isNotFoundError(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read label", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, flattenLabel(result))...)
}

func (r *LabelResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan LabelResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var result labelResponse
	if err := r.client.Call(ctx, http.MethodPut, "/v2/labels/"+url.PathEscape(plan.ID.ValueString()), labelRequestFromModel(plan), &result); err != nil {
		resp.Diagnostics.AddError("Unable to update label", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, flattenLabel(result))...)
}

func (r *LabelResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state LabelResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.client.Call(ctx, http.MethodDelete, "/v2/labels/"+url.PathEscape(state.ID.ValueString()), nil, nil); err != nil && !isNotFoundError(err) {
		resp.Diagnostics.AddError("Unable to delete label", err.Error())
	}
}

func (r *LabelResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func labelRequestFromModel(plan LabelResourceModel) labelRequest {
	return labelRequest{Name: plan.Name.ValueString(), Description: plan.Description.ValueString(), Color: labelColor{Type: "palette", Name: plan.Color.ValueString()}}
}

func flattenLabel(label labelResponse) LabelResourceModel {
	return LabelResourceModel{ID: types.StringValue(label.ID), Name: types.StringValue(label.Name), Description: types.StringValue(label.Description), Color: types.StringValue(label.Color.Name)}
}
