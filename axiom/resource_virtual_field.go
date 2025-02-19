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

	"github.com/axiomhq/axiom-go/axiom"
)

// Ensure provider-defined types fully satisfy framework interfaces.
var (
	_ resource.Resource                = &VirtualFieldResource{}
	_ resource.ResourceWithImportState = &VirtualFieldResource{}
)

// NewVirtualFieldResource creates a new VirtualFieldResource instance.
func NewVirtualFieldResource() resource.Resource {
	return &VirtualFieldResource{}
}

// VirtualFieldResource defines the resource implementation.
type VirtualFieldResource struct {
	client *axiom.Client
}

// VirtualFieldResourceModel describes the resource data model.
type VirtualFieldResourceModel struct {
	ID          types.String `tfsdk:"id"`
	Dataset     types.String `tfsdk:"dataset"`
	Name        types.String `tfsdk:"name"`
	Expression  types.String `tfsdk:"expression"`
	Description types.String `tfsdk:"description"`
	Type        types.String `tfsdk:"type"`
	Unit        types.String `tfsdk:"unit"`
}

func (r *VirtualFieldResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_virtual_field"
}

func (r *VirtualFieldResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Version: 1,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Virtual Field identifier",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"dataset": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Dataset the virtual field belongs to",
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Name of the virtual field",
			},
			"expression": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Expression defining the virtual field logic",
			},
			"description": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Optional description of the virtual field",
			},
			"type": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Type of the virtual field",
			},
			"unit": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Unit of the virtual field",
			},
		},
	}
}

func (r *VirtualFieldResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *VirtualFieldResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan VirtualFieldResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.client == nil {
		resp.Diagnostics.AddError("Client Error", "Client is not set")
		return
	}

	vfield, err := r.client.VirtualFields.Create(ctx, axiom.VirtualField{
		Dataset:     plan.Dataset.ValueString(),
		Name:        plan.Name.ValueString(),
		Expression:  plan.Expression.ValueString(),
		Description: plan.Description.ValueString(),
		Type:        plan.Type.ValueString(),
		Unit:        plan.Unit.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create Virtual Field, got error: %s", err))
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, flattenVirtualField(vfield))...)
}

func (r *VirtualFieldResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var plan VirtualFieldResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	vfield, err := r.client.VirtualFields.Get(ctx, plan.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to read Virtual Field", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, flattenVirtualField(vfield))...)
}

func (r *VirtualFieldResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan VirtualFieldResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	vfield, err := r.client.VirtualFields.Update(ctx, plan.ID.ValueString(), axiom.VirtualField{
		Dataset:     plan.Dataset.ValueString(),
		Name:        plan.Name.ValueString(),
		Expression:  plan.Expression.ValueString(),
		Description: plan.Description.ValueString(),
		Type:        plan.Type.ValueString(),
		Unit:        plan.Unit.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update Virtual Field, got error: %s", err))
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, flattenVirtualField(vfield))...)
}

func (r *VirtualFieldResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var plan VirtualFieldResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.VirtualFields.Delete(ctx, plan.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Failed to delete Virtual Field", err.Error())
		return
	}
}

func (r *VirtualFieldResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func flattenVirtualField(vfield *axiom.VirtualFieldWithID) VirtualFieldResourceModel {
	return VirtualFieldResourceModel{
		ID:          types.StringValue(vfield.ID),
		Dataset:     types.StringValue(vfield.Dataset),
		Name:        types.StringValue(vfield.Name),
		Expression:  types.StringValue(vfield.Expression),
		Description: types.StringValue(vfield.Description),
		Type:        types.StringValue(vfield.Type),
		Unit:        types.StringValue(vfield.Unit),
	}
}
