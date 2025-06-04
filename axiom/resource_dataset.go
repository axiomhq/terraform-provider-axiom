package axiom

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/axiomhq/axiom-go/axiom"
)

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ resource.Resource                = &DatasetResource{}
	_ resource.ResourceWithImportState = &DatasetResource{}
)

func NewDatasetResource() resource.Resource {
	return &DatasetResource{}
}

// DatasetResource defines the resource implementation.
type DatasetResource struct {
	client *axiom.Client
}

// DatasetResourceModel describes the resource data model.
type DatasetResourceModel struct {
	Name               types.String `tfsdk:"name"`
	Description        types.String `tfsdk:"description"`
	ID                 types.String `tfsdk:"id"`
	UseRetentionPeriod types.Bool   `tfsdk:"use_retention_period"`
	RetentionDays      types.Int64  `tfsdk:"retention_days"`
	ObjectFields       types.List   `tfsdk:"object_fields"`
}

func (r *DatasetResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_dataset"
}

func (r *DatasetResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Version: 1,
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Dataset name",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"description": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Dataset description",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Dataset identifier",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"use_retention_period": schema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Use retention for the dataset",
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.UseStateForUnknown(),
				},
			},
			"retention_days": schema.Int64Attribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Retention days for the dataset",
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"object_fields": schema.ListAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Object fields for the dataset",
				ElementType:         types.StringType,
			},
		},
	}
}

func (r *DatasetResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *DatasetResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan DatasetResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.client == nil {
		resp.Diagnostics.AddError("Client Error", "Client is not set")
		return
	}

	if plan.UseRetentionPeriod.ValueBool() && plan.RetentionDays.ValueInt64() == 0 {
		resp.Diagnostics.AddError("Client Error", "Retention days must be greater than 0 when use_retention_period is true")
		return
	}

	ds, err := r.client.Datasets.Create(ctx, axiom.DatasetCreateRequest{
		Name:               plan.Name.ValueString(),
		Description:        plan.Description.ValueString(),
		UseRetentionPeriod: plan.UseRetentionPeriod.ValueBool(),
		RetentionDays:      int(plan.RetentionDays.ValueInt64()),
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create dataset, got error: %s", err))
		return
	}

	if !plan.ObjectFields.IsUnknown() {
		objectFields, diags := typeStringSliceToStringSlice(ctx, plan.ObjectFields.Elements())
		if diags.HasError() {
			resp.Diagnostics.Append(diags...)
			return
		}

		err := r.client.Datasets.UpdateObjectFields(ctx, ds.ID, objectFields)
		if err != nil {
			resp.Diagnostics.AddError("failed to update dataset object fields", err.Error())
			return
		}

		ds.ObjectFields = objectFields
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, flattenDataset(ds))...)
}

func (r *DatasetResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var plan DatasetResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ds, err := r.client.Datasets.Get(ctx, plan.ID.ValueString())
	if err != nil {
		if isNotFoundError(err) {
			resp.Diagnostics.AddWarning(
				"Dataset Not Found",
				fmt.Sprintf("Dataset with ID %s does not exist and will be recreated if still defined in the configuration.", plan.ID.ValueString()),
			)
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read dataset", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, flattenDataset(ds))...)
}

func (r *DatasetResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan DatasetResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.client == nil {
		resp.Diagnostics.AddError("Client Error", "Client is not set")
		return
	}

	if plan.UseRetentionPeriod.ValueBool() && plan.RetentionDays.ValueInt64() == 0 {
		resp.Diagnostics.AddError("Client Error", "Retention days must be greater than 0 when use_retention_period is true")
		return
	}

	ds, err := r.client.Datasets.Update(ctx, plan.ID.ValueString(), axiom.DatasetUpdateRequest{
		Description:        plan.Description.ValueString(),
		UseRetentionPeriod: plan.UseRetentionPeriod.ValueBool(),
		RetentionDays:      int(plan.RetentionDays.ValueInt64()),
	})
	if err != nil {
		resp.Diagnostics.AddError("failed to update dataset", err.Error())
		return
	}

	if !plan.ObjectFields.IsUnknown() {
		objectFields, diags := typeStringSliceToStringSlice(ctx, plan.ObjectFields.Elements())
		if diags.HasError() {
			resp.Diagnostics.Append(diags...)
			return
		}

		err := r.client.Datasets.UpdateObjectFields(ctx, plan.ID.ValueString(), objectFields)
		if err != nil {
			resp.Diagnostics.AddError("failed to update dataset object fields", err.Error())
			return
		}

		ds.ObjectFields = objectFields
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, flattenDataset(ds))...)
}

func (r *DatasetResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var plan DatasetResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.Datasets.Delete(ctx, plan.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Failed to delete dataset", err.Error())
		return
	}
}

func (r *DatasetResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func flattenDataset(dataset *axiom.Dataset) DatasetResourceModel {
	var description types.String

	if dataset.Description != "" {
		description = types.StringValue(dataset.Description)
	}

	objectFields := make([]attr.Value, 0, len(dataset.ObjectFields))
	for _, fieldName := range dataset.ObjectFields {
		objectFields = append(objectFields, types.StringValue(fieldName))
	}

	return DatasetResourceModel{
		ID:                 types.StringValue(dataset.ID),
		Name:               types.StringValue(dataset.Name),
		Description:        description,
		UseRetentionPeriod: types.BoolValue(dataset.UseRetentionPeriod),
		RetentionDays:      types.Int64Value(int64(dataset.RetentionDays)),
		ObjectFields:       types.ListValueMust(types.StringType, objectFields),
	}
}
