package axiom

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
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
var _ resource.Resource = &NotifierResource{}
var _ resource.ResourceWithImportState = &NotifierResource{}

func NewNotifierResource() resource.Resource {
	return &NotifierResource{}
}

// NotifierResource defines the resource implementation.
type NotifierResource struct {
	client *axiom.Client
}

// NotifierResourceModel describes the resource data model.
type NotifierResourceModel struct {
	ID         types.String       `tfsdk:"id"`
	Name       types.String       `tfsdk:"name"`
	Properties NotifierProperties `tfsdk:"properties"`
	Type       types.String       `tfsdk:"type"`
}

type NotifierProperties struct {
	//Discord        *DiscordConfig        `json:"discord,omitempty"`
	//DiscordWebhook *DiscordWebhookConfig `json:"discordWebhook,omitempty"`
	//Email          *EmailConfig          `json:"email,omitempty"`
	//Opsgenie       *OpsGenieConfig       `json:"opsgenie,omitempty"`
	//Pagerduty      *PagerDutyConfig      `json:"pagerduty,omitempty"`
	Slack *SlackConfig `tfsdk:"slack"`
	//Webhook        *WebhookConfig        `json:"webhook,omitempty"`
}

type SlackConfig struct {
	SlackURL types.String `tfsdk:"slack_url"`
}

func (r *NotifierResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_notifier"
}

func (r *NotifierResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Example Notifier",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Notifier identifier",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Notifier name",
				Required:            true,
			},
			"type": schema.StringAttribute{
				MarkdownDescription: "The type of notifier",
				Required:            true,
			},
			"properties": schema.ObjectAttribute{
				MarkdownDescription: "The properties of the notifier",
				Required:            true,
				AttributeTypes: map[string]attr.Type{
					"slack": types.ObjectType{
						AttrTypes: map[string]attr.Type{
							"slack_url": types.StringType,
						},
					},
				},
			},
		},
	}
}

func (r *NotifierResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *NotifierResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data *NotifierResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if r.client == nil {
		resp.Diagnostics.AddError("Client Error", "Client is not set")
		return
	}

	properties := axiom.NotifierProperties{}
	if data.Properties.Slack != nil {
		properties.Slack = &axiom.SlackConfig{
			SlackURL: data.Properties.Slack.SlackURL.ValueString(),
		}
	}

	ds, err := r.client.Notifiers.Create(ctx, axiom.Notifier{
		Name:       data.Name.ValueString(),
		Properties: properties,
		Type:       data.Type.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create Notifier, got error: %s", err))
		return
	}

	data.ID = types.StringValue(ds.ID)
	data.Name = types.StringValue(ds.Name)
	data.Type = types.StringValue(ds.Type)

	// Write logs using the tflog package
	// Documentation: https://terraform.io/plugin/log
	tflog.Trace(ctx, "created a resource")

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *NotifierResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data *NotifierResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	ds, err := r.client.Notifiers.Get(ctx, data.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to read Notifier", err.Error())
		return
	}

	data.ID = types.StringValue(ds.ID)
	data.Name = types.StringValue(ds.Name)
	data.Type = types.StringValue(ds.Type)

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *NotifierResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data *NotifierResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	properties := axiom.NotifierProperties{}
	if data.Properties.Slack != nil {
		properties.Slack = &axiom.SlackConfig{
			SlackURL: data.Properties.Slack.SlackURL.ValueString(),
		}
	}

	ds, err := r.client.Notifiers.Update(ctx, data.ID.ValueString(), axiom.Notifier{
		ID:         data.ID.ValueString(),
		Name:       data.Name.ValueString(),
		Properties: properties,
		Type:       data.Type.ValueString(),
	})
	if err != nil {
		resp.Diagnostics.AddError("failed to update Notifier", err.Error())
		return
	}

	data.ID = types.StringValue(ds.ID)
	data.Name = types.StringValue(ds.Name)
	data.Type = types.StringValue(ds.Type)

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *NotifierResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data *NotifierResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.Notifiers.Delete(ctx, data.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Failed to delete Notifier", err.Error())
		return
	}

	tflog.Info(ctx, "deleted Notifier resource", map[string]interface{}{"id": data.ID.ValueString()})
}

func (r *NotifierResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
