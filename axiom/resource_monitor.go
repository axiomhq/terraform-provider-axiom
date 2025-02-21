package axiom

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/float64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
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
	Name                         types.String  `tfsdk:"name"`
	Description                  types.String  `tfsdk:"description"`
	ID                           types.String  `tfsdk:"id"`
	AlertOnNoData                types.Bool    `tfsdk:"alert_on_no_data"`
	NotifyByGroup                types.Bool    `tfsdk:"notify_by_group"`
	APLQuery                     types.String  `tfsdk:"apl_query"`
	DisabledUntil                types.String  `tfsdk:"disabled_until"`
	IntervalMinutes              types.Int64   `tfsdk:"interval_minutes"`
	NotifierIds                  types.List    `tfsdk:"notifier_ids"`
	Operator                     types.String  `tfsdk:"operator"`
	RangeMinutes                 types.Int64   `tfsdk:"range_minutes"`
	Threshold                    types.Float64 `tfsdk:"threshold"`
	Resolvable                   types.Bool    `tfsdk:"resolvable"`
	SecondDelay                  types.Int64   `tfsdk:"second_delay"`
	NotifyEveryRun               types.Bool    `tfsdk:"notify_every_run"`
	SkipResolved                 types.Bool    `tfsdk:"skip_resolved"`
	Tolerance                    types.Float64 `tfsdk:"tolerance"`
	TriggerFromNRuns             types.Int64   `tfsdk:"trigger_from_n_runs"`
	TriggerAfterNPositiveResults types.Int64   `tfsdk:"trigger_after_n_positive_results"`
	CompareDays                  types.Int64   `tfsdk:"compare_days"`
	Type                         types.String  `tfsdk:"type"`
	CreatedBy                    types.String  `tfsdk:"created_by"`
	CreatedAt                    types.String  `tfsdk:"created_at"`
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
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
			"notify_by_group": schema.BoolAttribute{
				MarkdownDescription: "If the monitor should track non-time groups separately",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
			"apl_query": schema.StringAttribute{
				MarkdownDescription: "The query used inside the monitor",
				Required:            true,
			},
			"disabled_until": schema.StringAttribute{
				MarkdownDescription: "The time the monitor will be disabled until",
				Optional:            true,
				Validators: []validator.String{
					stringvalidator.RegexMatches(regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})$`), "Disabled until is not a valid time format"),
				},
			},
			"interval_minutes": schema.Int64Attribute{
				MarkdownDescription: "How often the monitor should run",
				Optional:            true,
				Computed:            true,
				Default:             int64default.StaticInt64(1),
			},
			"notifier_ids": schema.ListAttribute{
				Optional:    true,
				ElementType: types.StringType,
				Description: "A list of notifier id's to be used when this monitor triggers",
			},
			"operator": schema.StringAttribute{
				MarkdownDescription: "Operator used in monitor trigger evaluation",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString(""),
				Validators: []validator.String{
					stringvalidator.OneOf([]string{
						axiom.Below.String(),
						axiom.BelowOrEqual.String(),
						axiom.Above.String(),
						axiom.AboveOrEqual.String(),
					}...),
				},
			},
			"range_minutes": schema.Int64Attribute{
				MarkdownDescription: "Query time range from now",
				Optional:            true,
				Computed:            true,
				Default:             int64default.StaticInt64(1),
			},
			"threshold": schema.Float64Attribute{
				MarkdownDescription: "The threshold where the monitor should trigger",
				Optional:            true,
				Computed:            true,
				Default:             float64default.StaticFloat64(0),
			},
			"resolvable": schema.BoolAttribute{
				MarkdownDescription: "Determines whether the events triggered by the monitor are individually resolvable. " +
					"This has no effect on threshold monitors",
				Optional: true,
				Default:  booldefault.StaticBool(false),
				Computed: true,
			},
			"second_delay": schema.Int64Attribute{
				MarkdownDescription: "The delay in seconds before the monitor runs (useful for situations where data is batched/delayed)",
				Optional:            true,
				Computed:            true,
				Default:             int64default.StaticInt64(0),
			},
			"notify_every_run": schema.BoolAttribute{
				MarkdownDescription: "Indicates whether to send notifications on every trigger",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
			"skip_resolved": schema.BoolAttribute{
				MarkdownDescription: "Specifies whether to skip resolved alerts",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
			"created_by": schema.StringAttribute{
				MarkdownDescription: "The ID of the user who created the monitor",
				Computed:            true,
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "The timestamp when the monitor was created",
				Computed:            true,
			},
			"tolerance": schema.Float64Attribute{
				MarkdownDescription: "The tolerance percentage for anomaly detection",
				Optional:            true,
				Computed:            true,
				Default:             float64default.StaticFloat64(0),
			},
			"trigger_from_n_runs": schema.Int64Attribute{
				MarkdownDescription: "The number of consecutive check runs that must trigger before triggering an alert",
				Optional:            true,
				Computed:            true,
				Default:             int64default.StaticInt64(1),
			},
			"trigger_after_n_positive_results": schema.Int64Attribute{
				MarkdownDescription: "The number of positive results needed before triggering",
				Optional:            true,
				Computed:            true,
				Default:             int64default.StaticInt64(0),
			},
			"compare_days": schema.Int64Attribute{
				MarkdownDescription: "The number of days to compare for anomaly detection",
				Optional:            true,
				Computed:            true,
				Default:             int64default.StaticInt64(0),
			},
			"type": schema.StringAttribute{
				MarkdownDescription: "The type of the monitor. Possible values include: 'Threshold', 'AnomalyDetection', 'MatchEvent'",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.OneOf([]string{
						"Threshold",
						"AnomalyDetection",
						"MatchEvent",
					}...),
				},
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

	monitor, err := r.client.Monitors.Create(ctx, axiom.MonitorCreateRequest{Monitor: *monitor})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create Monitor, got error: %s", err))
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, flattenMonitor(monitor))...)
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

	resp.Diagnostics.Append(resp.State.Set(ctx, flattenMonitor(monitor))...)
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

	monitor, err := r.client.Monitors.Update(ctx, plan.ID.ValueString(), axiom.MonitorUpdateRequest{Monitor: *monitor})
	if err != nil {
		resp.Diagnostics.AddError("failed to update Monitor", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, flattenMonitor(monitor))...)
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

	var disabledUntil time.Time
	if !plan.DisabledUntil.IsNull() {
		var err error
		disabledUntil, err = time.Parse(time.RFC3339, plan.DisabledUntil.ValueString())
		if err != nil {
			diags.AddError("invalid disabled until", plan.DisabledUntil.String())
			diags.AddError("failed to parse disabled until as RFC3339", err.Error())
			return nil, diags
		}
	}

	var operator axiom.Operator
	switch plan.Operator.ValueString() {
	case axiom.Below.String():
		operator = axiom.Below
	case axiom.BelowOrEqual.String():
		operator = axiom.BelowOrEqual
	case axiom.Above.String():
		operator = axiom.Above
	case axiom.AboveOrEqual.String():
		operator = axiom.AboveOrEqual
	}

	var monitorType axiom.MonitorType
	switch plan.Type.ValueString() {
	case "Threshold":
		monitorType = axiom.MonitorTypeThreshold
	case "MatchEvent":
		monitorType = axiom.MonitorTypeMatchEvent
	case "AnomalyDetection":
		monitorType = axiom.MonitorTypeAnonalyDetection
	default:
		diags.AddError(
			"Invalid monitor type",
			fmt.Sprintf("Monitor type must be one of: Threshold, MatchEvent, AnomalyDetection. Got: %s", plan.Type.ValueString()),
		)
		return nil, diags
	}

	if diags = validateMonitor(plan); diags.HasError() {
		return nil, diags
	}

	return &axiom.Monitor{
		Name:                         plan.Name.ValueString(),
		AlertOnNoData:                plan.AlertOnNoData.ValueBool(),
		NotifyByGroup:                plan.NotifyByGroup.ValueBool(),
		APLQuery:                     plan.APLQuery.ValueString(),
		Description:                  plan.Description.ValueString(),
		DisabledUntil:                disabledUntil,
		Interval:                     time.Duration(plan.IntervalMinutes.ValueInt64() * int64(time.Minute)),
		NotifierIDs:                  notifierIds,
		Operator:                     operator,
		Range:                        time.Duration(plan.RangeMinutes.ValueInt64() * int64(time.Minute)),
		Threshold:                    plan.Threshold.ValueFloat64(),
		Resolvable:                   plan.Resolvable.ValueBool(),
		SecondDelay:                  time.Duration(plan.SecondDelay.ValueInt64()) * time.Second,
		NotifyEveryRun:               plan.NotifyEveryRun.ValueBool(),
		SkipResolved:                 plan.SkipResolved.ValueBool(),
		Tolerance:                    plan.Tolerance.ValueFloat64(),
		TriggerFromNRuns:             plan.TriggerFromNRuns.ValueInt64(),
		TriggerAfterNPositiveResults: plan.TriggerAfterNPositiveResults.ValueInt64(),
		CompareDays:                  plan.CompareDays.ValueInt64(),
		Type:                         monitorType,
	}, nil
}

func flattenMonitor(monitor *axiom.Monitor) MonitorResourceModel {
	var disabledUntil types.String
	var description types.String

	if !monitor.DisabledUntil.IsZero() {
		disabledUntil = types.StringValue(monitor.DisabledUntil.Format(time.RFC3339))
	}

	if monitor.Description != "" {
		description = types.StringValue(monitor.Description)
	}
	return MonitorResourceModel{
		ID:                           types.StringValue(monitor.ID),
		Name:                         types.StringValue(monitor.Name),
		Description:                  description,
		AlertOnNoData:                types.BoolValue(monitor.AlertOnNoData),
		NotifyByGroup:                types.BoolValue(monitor.NotifyByGroup),
		APLQuery:                     types.StringValue(monitor.APLQuery),
		DisabledUntil:                disabledUntil,
		IntervalMinutes:              types.Int64Value(int64(monitor.Interval.Minutes())),
		NotifierIds:                  flattenStringSlice(monitor.NotifierIDs),
		Operator:                     types.StringValue(monitor.Operator.String()),
		RangeMinutes:                 types.Int64Value(int64(monitor.Range.Minutes())),
		Threshold:                    types.Float64Value(monitor.Threshold),
		Resolvable:                   types.BoolValue(monitor.Resolvable),
		SecondDelay:                  types.Int64Value(int64(monitor.SecondDelay.Seconds())),
		NotifyEveryRun:               types.BoolValue(monitor.NotifyEveryRun),
		SkipResolved:                 types.BoolValue(monitor.SkipResolved),
		Tolerance:                    types.Float64Value(monitor.Tolerance),
		TriggerFromNRuns:             types.Int64Value(monitor.TriggerFromNRuns),
		TriggerAfterNPositiveResults: types.Int64Value(monitor.TriggerAfterNPositiveResults),
		CompareDays:                  types.Int64Value(monitor.CompareDays),
		Type:                         types.StringValue(monitor.Type.String()),
		CreatedBy:                    types.StringValue(monitor.CreatedBy),
		CreatedAt:                    types.StringValue(monitor.CreatedAt.Format(time.RFC3339)),
	}
}

func validateMonitor(plan MonitorResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics
	switch plan.Type.ValueString() {
	case axiom.MonitorTypeThreshold.String():
		if plan.IntervalMinutes.IsNull() {
			diags = append(diags, diag.NewErrorDiagnostic(
				"Interval is required",
				"Interval is required for monitor type threshold",
			))
		}
		if plan.RangeMinutes.IsNull() {
			diags = append(diags, diag.NewErrorDiagnostic(
				"Range is required",
				"Range is required for monitor type threshold",
			))
		}
		if plan.Threshold.IsNull() {
			diags = append(diags, diag.NewErrorDiagnostic(
				"Threshold is required",
				"Threshold is required for monitor type threshold",
			))
		}
		if plan.Operator.IsNull() {
			diags = append(diags, diag.NewErrorDiagnostic(
				"Operator is required",
				"Operator is required for monitor type threshold",
			))
		}
	case axiom.MonitorTypeMatchEvent.String():

	case axiom.MonitorTypeAnonalyDetection.String():
		if plan.IntervalMinutes.IsNull() {
			diags = append(diags, diag.NewErrorDiagnostic(
				"Interval is required",
				"Interval is required for monitor type anomaly detection",
			))
		}
		if plan.RangeMinutes.IsNull() {
			diags = append(diags, diag.NewErrorDiagnostic(
				"Range is required",
				"Range is required for monitor type anomaly detection",
			))
		}
		if plan.CompareDays.IsNull() {
			diags = append(diags, diag.NewErrorDiagnostic(
				"CompareDays is required",
				"CompareDays is required for monitor type anomaly detection",
			))
		}
		if plan.Tolerance.IsNull() {
			diags = append(diags, diag.NewErrorDiagnostic(
				"Tolerance is required",
				"Tolerance is required for monitor type anomaly detection",
			))
		}
		if plan.Operator.IsNull() {
			diags = append(diags, diag.NewErrorDiagnostic(
				"Operator is required",
				"Operator is required for monitor type anomaly detection",
			))
		}
	}
	return diags
}
