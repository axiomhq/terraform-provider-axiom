package axiom

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/axiomhq/axiom-go/axiom"
)

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ resource.Resource                = &MonitorResource{}
	_ resource.ResourceWithImportState = &MonitorResource{}
)

func NewMonitorResource() resource.Resource {
	return &MonitorResource{}
}

// MonitorResource defines the resource implementation.
type MonitorResource struct {
	client *axiom.Client
}

// MonitorResourceModel describes the resource data model.
type MonitorResourceModel struct {
	Name            types.String  `tfsdk:"name"`
	Description     types.String  `tfsdk:"description"`
	ID              types.String  `tfsdk:"id"`
	AlertOnNoData   types.Bool    `tfsdk:"alert_on_no_data"`
	APLQuery        types.String  `tfsdk:"apl_query"`
	Disabled        types.Bool    `tfsdk:"disabled"`
	IntervalMinutes types.Int64   `tfsdk:"interval_minutes"`
	NotifierIds     types.List    `tfsdk:"notifier_ids"`
	Operator        types.String  `tfsdk:"operator"`
	RangeMinutes    types.Int64   `tfsdk:"range_minutes"`
	Threshold       types.Float64 `tfsdk:"threshold"`
}

func (r *MonitorResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_monitor"
}

func (r *MonitorResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Version: 1,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Monitor identifier",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Monitor name",
				Required:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Monitor description",
				Optional:            true,
			},
			"alert_on_no_data": schema.BoolAttribute{
				MarkdownDescription: "If the monitor should trigger an alert if there is no data",
				Required:            true,
			},
			"apl_query": schema.StringAttribute{
				MarkdownDescription: "The query used inside the monitor",
				Required:            true,
			},
			"disabled": schema.BoolAttribute{
				MarkdownDescription: "Is the monitor disabled",
				Required:            true,
			},
			"interval_minutes": schema.Int64Attribute{
				MarkdownDescription: "How often the monitor should run",
				Required:            true,
			},
			"notifier_ids": schema.ListAttribute{
				Optional:    true,
				ElementType: types.StringType,
				Description: "A list of notifier id's to be used when this monitor triggers",
			},
			"operator": schema.StringAttribute{
				MarkdownDescription: "Operator used in monitor trigger evaluation",
				Required:            true,
			},
			"range_minutes": schema.Int64Attribute{
				MarkdownDescription: "Query time range from now",
				Required:            true,
			},
			"threshold": schema.Float64Attribute{
				MarkdownDescription: "The threshold where the monitor should trigger",
				Required:            true,
			},
		},
	}
}

func (r *MonitorResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *MonitorResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan MonitorResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.client == nil {
		resp.Diagnostics.AddError("Client Error", "Client is not set")
		return
	}

	monitor, diags := extractMonitorResourceModel(ctx, plan)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	monitor, err := r.client.Monitors.Create(ctx, axiom.MonitorCreateRequest{*monitor})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create Monitor, got error: %s", err))
		return
	}

	plan = *flattenMonitor(monitor)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *MonitorResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var plan MonitorResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	monitor, err := r.client.Monitors.Get(ctx, plan.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to read Monitor", err.Error())
		return
	}

	plan = *flattenMonitor(monitor)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *MonitorResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan MonitorResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	monitor, diags := extractMonitorResourceModel(ctx, plan)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	monitor, err := r.client.Monitors.Update(ctx, plan.ID.ValueString(), axiom.MonitorUpdateRequest{*monitor})
	if err != nil {
		resp.Diagnostics.AddError("failed to update Monitor", err.Error())
		return
	}

	plan = *flattenMonitor(monitor)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *MonitorResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var plan MonitorResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.Monitors.Delete(ctx, plan.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Failed to delete Monitor", err.Error())
		return
	}
}

func (r *MonitorResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func extractMonitorResourceModel(ctx context.Context, plan MonitorResourceModel) (*axiom.Monitor, diag.Diagnostics) {
	notifierIds, diags := typeStringSliceToStringSlice(ctx, plan.NotifierIds.Elements())
	if diags.HasError() {
		return nil, diags
	}

	return &axiom.Monitor{
		Name:          plan.Name.ValueString(),
		AlertOnNoData: plan.AlertOnNoData.ValueBool(),
		APLQuery:      plan.APLQuery.ValueString(),
		Description:   plan.Description.ValueString(),
		Disabled:      plan.Disabled.ValueBool(),
		Interval:      time.Duration(plan.IntervalMinutes.ValueInt64() * int64(time.Minute)),
		NotifierIDs:   notifierIds,
		Operator:      plan.Operator.ValueString(),
		Range:         time.Duration(plan.RangeMinutes.ValueInt64() * int64(time.Minute)),
		Threshold:     plan.Threshold.ValueFloat64(),
	}, nil
}

func flattenMonitor(monitor *axiom.Monitor) *MonitorResourceModel {

	return &MonitorResourceModel{
		ID:              types.StringValue(monitor.ID),
		Name:            types.StringValue(monitor.Name),
		Description:     types.StringValue(monitor.Description),
		AlertOnNoData:   types.BoolValue(monitor.AlertOnNoData),
		APLQuery:        types.StringValue(monitor.APLQuery),
		Disabled:        types.BoolValue(monitor.Disabled),
		IntervalMinutes: types.Int64Value(int64(monitor.Interval.Minutes())),
		NotifierIds:     flattenStringSlice(monitor.NotifierIDs),
		Operator:        types.StringValue(monitor.Operator),
		RangeMinutes:    types.Int64Value(int64(monitor.Range.Minutes())),
		Threshold:       types.Float64Value(monitor.Threshold),
	}
}
