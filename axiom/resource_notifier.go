package axiom

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/objectvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/axiomhq/axiom-go/axiom"
)

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ resource.Resource                = &NotifierResource{}
	_ resource.ResourceWithImportState = &NotifierResource{}
)

func NewNotifierResource() resource.Resource {
	return &NotifierResource{}
}

// NotifierResource defines the resource implementation.
type NotifierResource struct {
	client *axiom.Client
}

// NotifierResourceModel describes the resource data model.
type NotifierResourceModel struct {
	ID         types.String        `tfsdk:"id"`
	Name       types.String        `tfsdk:"name"`
	Properties *NotifierProperties `tfsdk:"properties"`
}

type NotifierProperties struct {
	Discord        *DiscordConfig        `tfsdk:"discord"`
	DiscordWebhook *DiscordWebhookConfig `tfsdk:"discord_webhook"`
	Email          *EmailConfig          `tfsdk:"email"`
	Opsgenie       *OpsGenieConfig       `tfsdk:"opsgenie"`
	Pagerduty      *PagerDutyConfig      `tfsdk:"pagerduty"`
	Slack          *SlackConfig          `tfsdk:"slack"`
	Webhook        *WebhookConfig        `tfsdk:"webhook"`
	CustomWebhook  *CustomWebhookConfig  `tfsdk:"custom_webhook"`
}

type SlackConfig struct {
	SlackURL types.String `tfsdk:"slack_url"`
}

type DiscordConfig struct {
	DiscordChannel types.String `tfsdk:"discord_channel"`
	DiscordToken   types.String `tfsdk:"discord_token"`
}

type DiscordWebhookConfig struct {
	DiscordWebhookURL types.String `tfsdk:"discord_webhook_url"`
}

type EmailConfig struct {
	Emails types.List `tfsdk:"emails"`
}

type OpsGenieConfig struct {
	APIKey types.String `tfsdk:"api_key"`
	IsEU   types.Bool   `tfsdk:"is_eu"`
}

type PagerDutyConfig struct {
	RoutingKey types.String `tfsdk:"routing_key"`
	Token      types.String `tfsdk:"token"`
}

type WebhookConfig struct {
	URL types.String `tfsdk:"url"`
}

type CustomWebhookConfig struct {
	URL     types.String `tfsdk:"url"`
	Headers types.Map    `tfsdk:"headers"`
	Body    types.String `tfsdk:"body"`
}

func (r *NotifierResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_notifier"
}

func (r *NotifierResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Version: 1,
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
			"properties": schema.SingleNestedAttribute{
				MarkdownDescription: "The properties of the notifier",
				Required:            true,
				Attributes: map[string]schema.Attribute{
					"slack": schema.SingleNestedAttribute{
						Attributes: map[string]schema.Attribute{
							"slack_url": schema.StringAttribute{
								MarkdownDescription: "The slack URL",
								Required:            true,
							},
						},
						Optional: true,
						Validators: []validator.Object{
							objectvalidator.ExactlyOneOf(
								path.MatchRelative().AtParent().AtName("discord"),
								path.MatchRelative().AtParent().AtName("discord_webhook"),
								path.MatchRelative().AtParent().AtName("email"),
								path.MatchRelative().AtParent().AtName("opsgenie"),
								path.MatchRelative().AtParent().AtName("pagerduty"),
								path.MatchRelative().AtParent().AtName("webhook"),
								path.MatchRelative().AtParent().AtName("custom_webhook"),
							),
						},
					},
					"discord": schema.SingleNestedAttribute{
						Attributes: map[string]schema.Attribute{
							"discord_channel": schema.StringAttribute{
								MarkdownDescription: "The discord channel",
								Required:            true,
							},
							"discord_token": schema.StringAttribute{
								MarkdownDescription: "The discord token",
								Required:            true,
							},
						},
						Optional: true,
						Validators: []validator.Object{
							objectvalidator.ExactlyOneOf(
								path.MatchRelative().AtParent().AtName("slack"),
								path.MatchRelative().AtParent().AtName("discord_webhook"),
								path.MatchRelative().AtParent().AtName("email"),
								path.MatchRelative().AtParent().AtName("opsgenie"),
								path.MatchRelative().AtParent().AtName("pagerduty"),
								path.MatchRelative().AtParent().AtName("webhook"),
								path.MatchRelative().AtParent().AtName("custom_webhook"),
							),
						},
					},
					"discord_webhook": schema.SingleNestedAttribute{
						Attributes: map[string]schema.Attribute{
							"discord_webhook_url": schema.StringAttribute{
								MarkdownDescription: "The discord webhook URL",
								Required:            true,
							},
						},
						Optional: true,
						Validators: []validator.Object{
							objectvalidator.ExactlyOneOf(
								path.MatchRelative().AtParent().AtName("slack"),
								path.MatchRelative().AtParent().AtName("discord"),
								path.MatchRelative().AtParent().AtName("email"),
								path.MatchRelative().AtParent().AtName("opsgenie"),
								path.MatchRelative().AtParent().AtName("pagerduty"),
								path.MatchRelative().AtParent().AtName("webhook"),
								path.MatchRelative().AtParent().AtName("custom_webhook"),
							),
						},
					},
					"email": schema.SingleNestedAttribute{
						Attributes: map[string]schema.Attribute{
							"emails": schema.ListAttribute{
								MarkdownDescription: "The emails to be notified",
								Required:            true,
								ElementType:         types.StringType,
							},
						},
						Optional: true,
						Validators: []validator.Object{
							objectvalidator.ExactlyOneOf(
								path.MatchRelative().AtParent().AtName("slack"),
								path.MatchRelative().AtParent().AtName("discord"),
								path.MatchRelative().AtParent().AtName("discord_webhook"),
								path.MatchRelative().AtParent().AtName("opsgenie"),
								path.MatchRelative().AtParent().AtName("pagerduty"),
								path.MatchRelative().AtParent().AtName("webhook"),
								path.MatchRelative().AtParent().AtName("custom_webhook"),
							),
						},
					},
					"opsgenie": schema.SingleNestedAttribute{
						Attributes: map[string]schema.Attribute{
							"api_key": schema.StringAttribute{
								MarkdownDescription: "The opsgenie API key",
								Required:            true,
							},
							"is_eu": schema.BoolAttribute{
								MarkdownDescription: "The opsgenie is EU",
								Required:            true,
							},
						},
						Optional: true,
						Validators: []validator.Object{
							objectvalidator.ExactlyOneOf(
								path.MatchRelative().AtParent().AtName("slack"),
								path.MatchRelative().AtParent().AtName("discord"),
								path.MatchRelative().AtParent().AtName("discord_webhook"),
								path.MatchRelative().AtParent().AtName("email"),
								path.MatchRelative().AtParent().AtName("pagerduty"),
								path.MatchRelative().AtParent().AtName("webhook"),
								path.MatchRelative().AtParent().AtName("custom_webhook"),
							),
						},
					},
					"pagerduty": schema.SingleNestedAttribute{
						Attributes: map[string]schema.Attribute{
							"routing_key": schema.StringAttribute{
								MarkdownDescription: "The pagerduty routing key",
								Required:            true,
							},
							"token": schema.StringAttribute{
								MarkdownDescription: "The pager duty token",
								Optional:            true,
								DeprecationMessage:  "token is deprecated, and is not used",
							},
						},
						Optional: true,
						Validators: []validator.Object{
							objectvalidator.ExactlyOneOf(
								path.MatchRelative().AtParent().AtName("slack"),
								path.MatchRelative().AtParent().AtName("discord"),
								path.MatchRelative().AtParent().AtName("discord_webhook"),
								path.MatchRelative().AtParent().AtName("email"),
								path.MatchRelative().AtParent().AtName("opsgenie"),
								path.MatchRelative().AtParent().AtName("webhook"),
								path.MatchRelative().AtParent().AtName("custom_webhook"),
							),
						},
					},
					"webhook": schema.SingleNestedAttribute{
						Attributes: map[string]schema.Attribute{
							"url": schema.StringAttribute{
								MarkdownDescription: "The webhook URL",
								Required:            true,
							},
						},
						Optional: true,
						Validators: []validator.Object{
							objectvalidator.ExactlyOneOf(
								path.MatchRelative().AtParent().AtName("slack"),
								path.MatchRelative().AtParent().AtName("discord"),
								path.MatchRelative().AtParent().AtName("discord_webhook"),
								path.MatchRelative().AtParent().AtName("email"),
								path.MatchRelative().AtParent().AtName("opsgenie"),
								path.MatchRelative().AtParent().AtName("pagerduty"),
								path.MatchRelative().AtParent().AtName("custom_webhook"),
							),
						},
					},
					"custom_webhook": schema.SingleNestedAttribute{
						Attributes: map[string]schema.Attribute{
							"url": schema.StringAttribute{
								MarkdownDescription: "The webhook URL",
								Required:            true,
							},
							"body": schema.StringAttribute{
								MarkdownDescription: "The JSON body",
								Required:            true,
							},
							"headers": schema.MapAttribute{
								ElementType:         types.StringType,
								MarkdownDescription: "Any headers associated with the request",
								Optional:            true,
								Sensitive:           true,
							},
						},
						Optional: true,
						Validators: []validator.Object{
							objectvalidator.ExactlyOneOf(
								path.MatchRelative().AtParent().AtName("slack"),
								path.MatchRelative().AtParent().AtName("discord"),
								path.MatchRelative().AtParent().AtName("discord_webhook"),
								path.MatchRelative().AtParent().AtName("email"),
								path.MatchRelative().AtParent().AtName("opsgenie"),
								path.MatchRelative().AtParent().AtName("pagerduty"),
								path.MatchRelative().AtParent().AtName("webhook"),
							),
						},
					},
				},
			},
		},
	}
}

func (r *NotifierResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
	var plan NotifierResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.client == nil {
		resp.Diagnostics.AddError("Client Error", "Client is not set")
		return
	}

	notifier, diags := extractNotifier(ctx, plan)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	notifier, err := r.client.Notifiers.Create(ctx, *notifier)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create Notifier, got error: %s", err))
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, flattenNotifier(*notifier))...)
}

func (r *NotifierResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var plan NotifierResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &plan)...)

	if resp.Diagnostics.HasError() {
		return
	}

	notifier, err := r.client.Notifiers.Get(ctx, plan.ID.ValueString())
	if err != nil {
		if isNotFoundError(err) {
			resp.Diagnostics.AddWarning(
				"Notifier Not Found",
				fmt.Sprintf("Notifier with ID %s does not exist and will be recreated if still defined in the configuration.", plan.ID.ValueString()),
			)
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read Notifier", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, flattenNotifier(*notifier))...)
}

func (r *NotifierResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan NotifierResourceModel
	// Read Terraform plan plan into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)

	if resp.Diagnostics.HasError() {
		return
	}

	notifier, diags := extractNotifier(ctx, plan)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	notifier, err := r.client.Notifiers.Update(ctx, plan.ID.ValueString(), *notifier)
	if err != nil {
		resp.Diagnostics.AddError("failed to update Notifier", err.Error())
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, flattenNotifier(*notifier))...)
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
}

func (r *NotifierResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func extractNotifier(ctx context.Context, plan NotifierResourceModel) (*axiom.Notifier, diag.Diagnostics) {
	var diags diag.Diagnostics
	notifier := axiom.Notifier{
		ID:         plan.ID.ValueString(),
		Name:       plan.Name.ValueString(),
		Properties: axiom.NotifierProperties{},
	}

	switch {
	case plan.Properties.Slack != nil:
		notifier.Properties.Slack = &axiom.SlackConfig{
			SlackURL: plan.Properties.Slack.SlackURL.ValueString(),
		}
	case plan.Properties.Discord != nil:
		notifier.Properties.Discord = &axiom.DiscordConfig{
			DiscordChannel: plan.Properties.Discord.DiscordChannel.ValueString(),
			DiscordToken:   plan.Properties.Discord.DiscordToken.ValueString(),
		}
	case plan.Properties.DiscordWebhook != nil:
		notifier.Properties.DiscordWebhook = &axiom.DiscordWebhookConfig{
			DiscordWebhookURL: plan.Properties.DiscordWebhook.DiscordWebhookURL.ValueString(),
		}
	case plan.Properties.Email != nil:
		values, diags := typeStringSliceToStringSlice(ctx, plan.Properties.Email.Emails.Elements())
		if diags.HasError() {
			return nil, diags
		}
		notifier.Properties.Email = &axiom.EmailConfig{
			Emails: values,
		}
	case plan.Properties.Opsgenie != nil:
		notifier.Properties.Opsgenie = &axiom.OpsGenieConfig{
			APIKey: plan.Properties.Opsgenie.APIKey.ValueString(),
			IsEU:   plan.Properties.Opsgenie.IsEU.ValueBool(),
		}
	case plan.Properties.Pagerduty != nil:
		notifier.Properties.Pagerduty = &axiom.PagerDutyConfig{
			RoutingKey: plan.Properties.Pagerduty.RoutingKey.ValueString(),
			Token:      plan.Properties.Pagerduty.Token.ValueString(),
		}
	case plan.Properties.Webhook != nil:
		notifier.Properties.Webhook = &axiom.WebhookConfig{
			URL: plan.Properties.Webhook.URL.ValueString(),
		}
	case plan.Properties.CustomWebhook != nil:
		headers := map[string]string{}
		diags := plan.Properties.CustomWebhook.Headers.ElementsAs(ctx, &headers, false)
		if diags.HasError() {
			return nil, diags
		}
		notifier.Properties.CustomWebhook = &axiom.CustomWebhook{
			URL:     plan.Properties.CustomWebhook.URL.ValueString(),
			Headers: headers,
			Body:    plan.Properties.CustomWebhook.Body.ValueString(),
		}
	}

	return &notifier, diags
}

func flattenNotifier(notifier axiom.Notifier) NotifierResourceModel {
	return NotifierResourceModel{
		ID:         types.StringValue(notifier.ID),
		Name:       types.StringValue(notifier.Name),
		Properties: buildNotifierProperties(notifier.Properties),
	}
}

func buildNotifierProperties(properties axiom.NotifierProperties) *NotifierProperties {
	var notifierProperties NotifierProperties
	if properties.Discord != nil {
		notifierProperties.Discord = &DiscordConfig{
			DiscordChannel: types.StringValue(properties.Discord.DiscordChannel),
			DiscordToken:   types.StringValue(properties.Discord.DiscordToken),
		}
	}
	if properties.DiscordWebhook != nil {
		notifierProperties.DiscordWebhook = &DiscordWebhookConfig{
			DiscordWebhookURL: types.StringValue(properties.DiscordWebhook.DiscordWebhookURL),
		}
	}
	if properties.Email != nil {
		notifierProperties.Email = &EmailConfig{
			Emails: flattenStringSlice(properties.Email.Emails),
		}
	}
	if properties.Opsgenie != nil {
		notifierProperties.Opsgenie = &OpsGenieConfig{
			APIKey: types.StringValue(properties.Opsgenie.APIKey),
			IsEU:   types.BoolValue(properties.Opsgenie.IsEU),
		}
	}
	if properties.Pagerduty != nil {
		notifierProperties.Pagerduty = &PagerDutyConfig{
			RoutingKey: types.StringValue(properties.Pagerduty.RoutingKey),
			Token:      types.StringValue(properties.Pagerduty.Token),
		}
	}
	if properties.Slack != nil {
		notifierProperties.Slack = &SlackConfig{
			SlackURL: types.StringValue(properties.Slack.SlackURL),
		}
	}
	if properties.Webhook != nil {
		notifierProperties.Webhook = &WebhookConfig{
			URL: types.StringValue(properties.Webhook.URL),
		}
	}
	if properties.CustomWebhook != nil {
		headerValues := map[string]attr.Value{}
		for k, v := range properties.CustomWebhook.Headers {
			headerValues[k] = types.StringValue(v)
		}
		headers := types.MapValueMust(types.StringType, headerValues)

		notifierProperties.CustomWebhook = &CustomWebhookConfig{
			URL:     types.StringValue(properties.CustomWebhook.URL),
			Headers: headers,
			Body:    types.StringValue(properties.CustomWebhook.Body),
		}
	}
	return &notifierProperties
}

func flattenStringSlice(values []string) types.List {
	if len(values) == 0 {
		return types.ListNull(types.StringType)
	}

	listElements := make([]attr.Value, 0, len(values))
	for _, value := range values {
		columnElement := types.StringValue(value)
		listElements = append(listElements, columnElement)
	}

	return types.ListValueMust(types.StringType, listElements)
}
