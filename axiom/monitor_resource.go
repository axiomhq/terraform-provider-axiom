package axiom

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/axiomhq/axiom-go/axiom"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &MonitorResource{}
var _ resource.ResourceWithImportState = &MonitorResource{}

func NewMonitorResource() resource.Resource {
	return &MonitorResource{}
}

// MonitorResource defines the resource implementation.
type MonitorResource struct {
	client *axiom.Client
}

// MonitorResourceModel describes the resource data model.
type MonitorResourceModel struct {
	Name            types.String   `tfsdk:"name"`
	Description     types.String   `tfsdk:"description"`
	ID              types.String   `tfsdk:"id"`
	AlertOnNoData   types.Bool     `tfsdk:"alert_on_no_data"`
	AplQuery        types.String   `tfsdk:"apl_query"`
	Disabled        types.Bool     `tfsdk:"disabled"`
	IntervalMinutes types.Int64    `tfsdk:"interval_minutes"`
	MatchEveryN     types.Int64    `tfsdk:"match_every_n"`
	MatchValue      types.String   `tfsdk:"match_value"`
	NotifierIds     []types.String `tfsdk:"notifier_ids"`
	Operator        types.String   `tfsdk:"operator"`
	RangeMinutes    types.Int64    `tfsdk:"range_minutes"`
	Threshold       types.Float64  `tfsdk:"threshold"`
}

func (r *MonitorResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_monitor"
}

func (r *MonitorResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Example Monitor",

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
			"match_every_n": schema.Int64Attribute{
				MarkdownDescription: "Unknown",
				Optional:            true,
			},
			"match_value": schema.StringAttribute{
				MarkdownDescription: "Unknown",
				Optional:            true,
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

func (r *MonitorResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *MonitorResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data *MonitorResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	fmt.Println("Creating Monitor")
	fmt.Printf("name: %s\n", data.Name.ValueString())
	fmt.Printf("description: %s\n", data.Description.ValueString())

	if r.client == nil {
		resp.Diagnostics.AddError("Client Error", "Client is not set")
		return
	}
	if r.client.Monitors == nil {
		resp.Diagnostics.AddError("Client Error", "Client Monitors is not set")
		return
	}

	notifierIds := make([]string, len(data.NotifierIds))
	for i, v := range data.NotifierIds {
		notifierIds[i] = v.ValueString()
	}

	ds, err := r.client.Monitors.Create(ctx, axiom.Monitor{
		Name:            data.Name.ValueString(),
		AlertOnNoData:   data.AlertOnNoData.ValueBool(),
		AplQuery:        data.AplQuery.ValueString(),
		Description:     data.Description.ValueString(),
		Disabled:        data.Disabled.ValueBool(),
		IntervalMinutes: data.IntervalMinutes.ValueInt64(),
		MatchEveryN:     data.MatchEveryN.ValueInt64(),
		MatchValue:      data.MatchValue.ValueString(),
		NotifierIds:     notifierIds,
		Operator:        data.Operator.ValueString(),
		RangeMinutes:    data.RangeMinutes.ValueInt64(),
		Threshold:       data.Threshold.ValueFloat64(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create Monitor, got error: %s", err))
		return
	}

	data.ID = types.StringValue(ds.ID)
	data.Name = types.StringValue(ds.Name)
	data.Description = types.StringValue(ds.Description)

	// Write logs using the tflog package
	// Documentation: https://terraform.io/plugin/log
	tflog.Trace(ctx, "created a resource")

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *MonitorResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data *MonitorResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	ds, err := r.client.Monitors.Get(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to read Monitor", err.Error())
		return
	}

	data.Name = types.StringValue(ds.Name)
	data.ID = types.StringValue(ds.ID)
	data.Description = types.StringValue(ds.Description)

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *MonitorResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data *MonitorResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	ds, err := r.client.Monitors.Update(ctx, data.ID.ValueString(), axiom.Monitor{
		AlertOnNoData:   false,
		AplQuery:        "",
		Description:     data.Description.ValueString(),
		Disabled:        false,
		IntervalMinutes: 0,
		MatchEveryN:     0,
		MatchValue:      "",
		Name:            "",
		NotifierIds:     nil,
		Operator:        "",
		RangeMinutes:    0,
		Threshold:       0,
	})
	if err != nil {
		resp.Diagnostics.AddError("failed to update Monitor", err.Error())
		return
	}

	data.ID = types.StringValue(ds.ID)
	data.Name = types.StringValue(ds.Name)
	data.Description = types.StringValue(ds.Description)

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *MonitorResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data *MonitorResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.Monitors.Delete(ctx, data.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Failed to delete Monitor", err.Error())
		return
	}

	tflog.Info(ctx, "deleted Monitor resource", map[string]interface{}{"id": data.ID.ValueString()})
}

func (r *MonitorResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
