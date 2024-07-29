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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
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
	Name            types.String  `tfsdk:"name"`
	Description     types.String  `tfsdk:"description"`
	ID              types.String  `tfsdk:"id"`
	AlertOnNoData   types.Bool    `tfsdk:"alert_on_no_data"`
	NotifyByGroup   types.Bool    `tfsdk:"notify_by_group"`
	APLQuery        types.String  `tfsdk:"apl_query"`
	DisabledUntil   types.String  `tfsdk:"disabled_until"`
	IntervalMinutes types.Int64   `tfsdk:"interval_minutes"`
	NotifierIds     types.List    `tfsdk:"notifier_ids"`
	Operator        types.String  `tfsdk:"operator"`
	RangeMinutes    types.Int64   `tfsdk:"range_minutes"`
	Threshold       types.Float64 `tfsdk:"threshold"`
	Resolvable      types.Bool    `tfsdk:"resolvable"`
	Disabled        types.Bool    `tfsdk:"disabled"`
	SecondDelay     types.Int64   `tfsdk:"second_delay"`
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
			"notify_by_group": schema.BoolAttribute{
				MarkdownDescription: "If the monitor should track non-time groups separately",
				Required:            true,
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
				Required:            true,
			},
			"threshold": schema.Float64Attribute{
				MarkdownDescription: "The threshold where the monitor should trigger",
				Required:            true,
			},
			"resolvable": schema.BoolAttribute{
				MarkdownDescription: "Determines whether the events triggered by the monitor are individually resolvable. " +
					"This has no effect on threshold monitors",
				Optional: true,
				Default:  booldefault.StaticBool(false),
				Computed: true,
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

	return &axiom.Monitor{
		Name:          plan.Name.ValueString(),
		AlertOnNoData: plan.AlertOnNoData.ValueBool(),
		NotifyByGroup: plan.NotifyByGroup.ValueBool(),
		APLQuery:      plan.APLQuery.ValueString(),
		Description:   plan.Description.ValueString(),
		DisabledUntil: disabledUntil,
		Interval:      time.Duration(plan.IntervalMinutes.ValueInt64() * int64(time.Minute)),
		NotifierIDs:   notifierIds,
		Operator:      operator,
		Range:         time.Duration(plan.RangeMinutes.ValueInt64() * int64(time.Minute)),
		Threshold:     plan.Threshold.ValueFloat64(),
		Resolvable:    plan.Resolvable.ValueBool(),
		Disabled:      plan.Disabled.ValueBool(),
		SecondDelay:   int(plan.SecondDelay.ValueInt64()),
	}, nil
}

func flattenMonitor(monitor *axiom.Monitor) MonitorResourceModel {
	var disabledUntil types.String
	if !monitor.DisabledUntil.IsZero() {
		disabledUntil = types.StringValue(monitor.DisabledUntil.Format(time.RFC3339))
	}
	return MonitorResourceModel{
		ID:              types.StringValue(monitor.ID),
		Name:            types.StringValue(monitor.Name),
		Description:     types.StringValue(monitor.Description),
		AlertOnNoData:   types.BoolValue(monitor.AlertOnNoData),
		NotifyByGroup:   types.BoolValue(monitor.NotifyByGroup),
		APLQuery:        types.StringValue(monitor.APLQuery),
		DisabledUntil:   disabledUntil,
		IntervalMinutes: types.Int64Value(int64(monitor.Interval.Minutes())),
		NotifierIds:     flattenStringSlice(monitor.NotifierIDs),
		Operator:        types.StringValue(monitor.Operator.String()),
		RangeMinutes:    types.Int64Value(int64(monitor.Range.Minutes())),
		Threshold:       types.Float64Value(monitor.Threshold),
		Resolvable:      types.BoolValue(monitor.Resolvable),
		Disabled:        types.BoolValue(monitor.Disabled),
		SecondDelay:     types.Int64Value(int64(monitor.SecondDelay)),
	}
}
