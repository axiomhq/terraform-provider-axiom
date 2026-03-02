package axiom

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/axiomhq/axiom-go/axiom"
)

var (
	_ resource.Resource                = &DashboardResource{}
	_ resource.ResourceWithImportState = &DashboardResource{}
)

func NewDashboardResource() resource.Resource {
	return &DashboardResource{}
}

type DashboardResource struct {
	client *axiom.Client
}

type DashboardResourceModel struct {
	ID         types.String `tfsdk:"id"`
	UID        types.String `tfsdk:"uid"`
	Dashboard  types.String `tfsdk:"dashboard"`
	Version    types.Int64  `tfsdk:"version"`
	Overwrite  types.Bool   `tfsdk:"overwrite"`
	Message    types.String `tfsdk:"message"`
	InternalID types.String `tfsdk:"internal_id"`
	CreatedAt  types.String `tfsdk:"created_at"`
	UpdatedAt  types.String `tfsdk:"updated_at"`
	CreatedBy  types.String `tfsdk:"created_by"`
	UpdatedBy  types.String `tfsdk:"updated_by"`
}

type dashboardUpsertRequest struct {
	Dashboard json.RawMessage `json:"dashboard"`
	UID       string          `json:"uid,omitempty"`
	Version   int64           `json:"version,omitempty"`
	Overwrite bool            `json:"overwrite,omitempty"`
	Message   string          `json:"message,omitempty"`
}

type dashboardResourcePayload struct {
	UID       string          `json:"uid"`
	ID        string          `json:"id"`
	Version   int64           `json:"version"`
	Dashboard json.RawMessage `json:"dashboard"`
	CreatedAt string          `json:"createdAt"`
	UpdatedAt string          `json:"updatedAt"`
	CreatedBy string          `json:"createdBy"`
	UpdatedBy string          `json:"updatedBy"`
}

type dashboardWriteResponse struct {
	Dashboard dashboardResourcePayload `json:"dashboard"`
}

func (r *DashboardResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dashboard"
}

func (r *DashboardResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Dashboard identifier (same value as `uid`).",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"uid": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Stable dashboard identifier. If omitted, Axiom generates one.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"dashboard": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The dashboard document as a JSON string (for example from `jsonencode(...)`).",
			},
			"version": schema.Int64Attribute{
				Computed:            true,
				MarkdownDescription: "Monotonic dashboard version used for optimistic updates.",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"overwrite": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
				MarkdownDescription: "When `true`, force update and ignore `version` conflicts.",
			},
			"message": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Optional write note included in dashboard upsert requests.",
			},
			"internal_id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Internal dashboard identifier returned by the API.",
			},
			"created_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Creation timestamp returned by the API.",
			},
			"updated_at": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Last update timestamp returned by the API.",
			},
			"created_by": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Creator returned by the API.",
			},
			"updated_by": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Last updater returned by the API.",
			},
		},
	}
}

func (r *DashboardResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *DashboardResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan DashboardResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.client == nil {
		resp.Diagnostics.AddError("Client Error", "Client is not set")
		return
	}

	payload, uid, diags := dashboardUpsertPayloadFromModel(plan, "", 0, true)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	rawReq, err := json.Marshal(payload)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create dashboard", fmt.Sprintf("Unable to encode request payload: %s", err))
		return
	}

	rawResp, err := r.client.Dashboards.CreateRaw(ctx, rawReq)
	if err != nil {
		addDashboardCreateError(resp, err, uid, 0)
		return
	}

	created, err := decodeDashboardWriteResponse(rawResp)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create dashboard", fmt.Sprintf("Unable to decode API response: %s", err))
		return
	}

	state, err := flattenDashboardResource(created.Dashboard, plan.Overwrite, plan.Message)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create dashboard", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, state)...)
}

func (r *DashboardResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state DashboardResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.client == nil {
		resp.Diagnostics.AddError("Client Error", "Client is not set")
		return
	}

	uid := dashboardUIDFromState(state)
	rawResp, err := r.client.Dashboards.GetRaw(ctx, uid)
	if err != nil {
		if isNotFoundError(err) {
			resp.Diagnostics.AddWarning(
				"Dashboard Not Found",
				fmt.Sprintf("Dashboard with UID %s does not exist and will be recreated if still defined in the configuration.", uid),
			)
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError("Failed to read dashboard", err.Error())
		return
	}

	dashboard, err := decodeDashboardResource(rawResp)
	if err != nil {
		resp.Diagnostics.AddError("Failed to read dashboard", fmt.Sprintf("Unable to decode API response: %s", err))
		return
	}

	flattened, err := flattenDashboardResource(*dashboard, state.Overwrite, state.Message)
	if err != nil {
		resp.Diagnostics.AddError("Failed to read dashboard", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, flattened)...)
}

func (r *DashboardResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan DashboardResourceModel
	var state DashboardResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.client == nil {
		resp.Diagnostics.AddError("Client Error", "Client is not set")
		return
	}

	stateVersion := state.Version.ValueInt64()
	payload, uid, diags := dashboardUpsertPayloadFromModel(plan, dashboardUIDFromState(state), stateVersion, false)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	rawReq, err := json.Marshal(payload)
	if err != nil {
		resp.Diagnostics.AddError("Failed to update dashboard", fmt.Sprintf("Unable to encode request payload: %s", err))
		return
	}

	rawResp, err := r.client.Dashboards.UpdateRaw(ctx, uid, rawReq)
	if err != nil {
		addDashboardUpdateError(resp, err, uid, stateVersion)
		return
	}

	updated, err := decodeDashboardWriteResponse(rawResp)
	if err != nil {
		resp.Diagnostics.AddError("Failed to update dashboard", fmt.Sprintf("Unable to decode API response: %s", err))
		return
	}

	flattened, err := flattenDashboardResource(updated.Dashboard, plan.Overwrite, plan.Message)
	if err != nil {
		resp.Diagnostics.AddError("Failed to update dashboard", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, flattened)...)
}

func (r *DashboardResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state DashboardResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.client == nil {
		resp.Diagnostics.AddError("Client Error", "Client is not set")
		return
	}

	uid := dashboardUIDFromState(state)
	if err := r.client.Dashboards.Delete(ctx, uid); err != nil {
		if isNotFoundError(err) {
			return
		}
		resp.Diagnostics.AddError("Failed to delete dashboard", err.Error())
		return
	}
}

func (r *DashboardResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("uid"), req.ID)...)
}

func dashboardUpsertPayloadFromModel(plan DashboardResourceModel, fallbackUID string, currentVersion int64, isCreate bool) (dashboardUpsertRequest, string, diag.Diagnostics) {
	var diags diag.Diagnostics

	normalizedDashboard, err := normalizeDashboardString(plan.Dashboard.ValueString())
	if err != nil {
		diags.AddError("Invalid dashboard JSON", fmt.Sprintf("`dashboard` must be valid JSON: %s", err))
		return dashboardUpsertRequest{}, "", diags
	}

	uid := fallbackUID
	if !plan.UID.IsNull() && !plan.UID.IsUnknown() {
		uid = plan.UID.ValueString()
	}

	payload := dashboardUpsertRequest{
		Dashboard: normalizedDashboard,
		UID:       uid,
	}

	overwrite := !plan.Overwrite.IsNull() && !plan.Overwrite.IsUnknown() && plan.Overwrite.ValueBool()
	if overwrite {
		payload.Overwrite = true
	} else if !isCreate && currentVersion == 0 {
		if uid != "" {
			diags.AddError(
				"Missing dashboard version",
				"The provider could not determine a dashboard version for a safe update. Re-import or refresh state before applying.",
			)
		}
		return payload, uid, diags
	} else {
		payload.Version = currentVersion
	}

	if !plan.Message.IsNull() && !plan.Message.IsUnknown() {
		payload.Message = plan.Message.ValueString()
	}

	return payload, uid, diags
}

func dashboardUIDFromState(state DashboardResourceModel) string {
	if !state.UID.IsNull() && !state.UID.IsUnknown() && state.UID.ValueString() != "" {
		return state.UID.ValueString()
	}

	if !state.ID.IsNull() && !state.ID.IsUnknown() {
		return state.ID.ValueString()
	}

	return ""
}

func flattenDashboardResource(in dashboardResourcePayload, overwrite types.Bool, message types.String) (DashboardResourceModel, error) {
	normalizedDashboard, err := normalizeDashboardRaw(in.Dashboard)
	if err != nil {
		return DashboardResourceModel{}, fmt.Errorf("unable to normalize dashboard document: %w", err)
	}

	uid := in.UID
	if uid == "" {
		return DashboardResourceModel{}, errors.New("dashboard response is missing uid")
	}

	return DashboardResourceModel{
		ID:         types.StringValue(uid),
		UID:        types.StringValue(uid),
		Dashboard:  types.StringValue(normalizedDashboard),
		Version:    types.Int64Value(in.Version),
		Overwrite:  overwrite,
		Message:    message,
		InternalID: types.StringValue(in.ID),
		CreatedAt:  types.StringValue(in.CreatedAt),
		UpdatedAt:  types.StringValue(in.UpdatedAt),
		CreatedBy:  types.StringValue(in.CreatedBy),
		UpdatedBy:  types.StringValue(in.UpdatedBy),
	}, nil
}

func decodeDashboardWriteResponse(raw []byte) (*dashboardWriteResponse, error) {
	out := new(dashboardWriteResponse)
	if err := json.Unmarshal(raw, out); err != nil {
		return nil, err
	}
	return out, nil
}

func decodeDashboardResource(raw []byte) (*dashboardResourcePayload, error) {
	out := new(dashboardResourcePayload)
	if err := json.Unmarshal(raw, out); err != nil {
		return nil, err
	}
	return out, nil
}

func normalizeDashboardString(raw string) (json.RawMessage, error) {
	parsed := make(map[string]any)
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil, err
	}

	normalized, err := json.Marshal(parsed)
	if err != nil {
		return nil, err
	}

	return normalized, nil
}

func normalizeDashboardRaw(raw json.RawMessage) (string, error) {
	if len(raw) == 0 {
		return "", errors.New("dashboard payload is empty")
	}

	parsed := make(map[string]any)
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", err
	}

	normalized, err := json.Marshal(parsed)
	if err != nil {
		return "", err
	}

	return string(normalized), nil
}

func addDashboardUpdateError(resp *resource.UpdateResponse, err error, uid string, localVersion int64) {
	addDashboardWriteErrorDiagnostics(&resp.Diagnostics, err, uid, localVersion)
}

func addDashboardWriteErrorDiagnostics(diags *diag.Diagnostics, err error, uid string, localVersion int64) {
	var apiErr axiom.HTTPError
	if errors.As(err, &apiErr) {
		if apiErr.Status == 412 {
			message := fmt.Sprintf("Dashboard `%s` update rejected due to version mismatch (local version: %d). Refresh state, re-import, or set `overwrite = true` to force reconciliation.", uid, localVersion)
			diags.AddError("Dashboard version mismatch", message)
			return
		}

		diags.AddError("Dashboard API error", fmt.Sprintf("Axiom API returned status %d: %s", apiErr.Status, apiErr.Message))
		return
	}

	diags.AddError("Dashboard API error", err.Error())
}

func addDashboardCreateError(resp *resource.CreateResponse, err error, uid string, localVersion int64) {
	addDashboardWriteErrorDiagnostics(&resp.Diagnostics, err, uid, localVersion)
}
