package axiom

import (
	"os"
	"testing"

	ax "github.com/axiomhq/axiom-go/axiom"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/stretchr/testify/assert"
)

func TestNotifiers(t *testing.T) {
	client, err := ax.NewClient()
	assert.NoError(t, err)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"axiom": providerserver.NewProtocol6WithError(NewAxiomProvider()),
		},
		CheckDestroy: testAccCheckAxiomResourcesDestroyed(client),
		Steps: []resource.TestStep{
			{
				Config: testConfigNotifier("slack", ` {
					slack = {
						slack_url = "https://hooks.slack.com/services/T00000000/B00000000/XXXXXXXXXXXXXXXXXXXXXXXX"
					} }`),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckResourcesCreatesCorrectValues(client, "axiom_notifier.slack", "properties.slack.slack_url", "properties.slack.slackUrl"),
				),
			},
			{
				Config: testConfigNotifier("discord", ` {
					discord = {
						discord_channel = "general"
						discord_token = "fake_discord_token"
					} }`),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckResourcesCreatesCorrectValues(client, "axiom_notifier.discord", "properties.discord.discord_channel", "properties.discord.discordChannel"),
					testAccCheckResourcesCreatesCorrectValues(client, "axiom_notifier.discord", "properties.discord.discord_token", "properties.discord.discordToken"),
				),
			},
			{
				Config: testConfigNotifier("discord_webhook", ` {
					discord_webhook = {
						discord_webhook_url = "https://discord.com/api/webhooks/1234567890/abcdefghijklmnopqrstuvwxyz"
					} }`),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckResourcesCreatesCorrectValues(client, "axiom_notifier.discord_webhook", "properties.discord_webhook.discord_webhook_url", "properties.discordWebhook.discordWebhookUrl"),
				),
			},
			{
				Config: testConfigNotifier("email", ` {
					email = {
						emails = ["test@example.com"]
					} }`),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckResourcesCreatesCorrectValues(client, "axiom_notifier.email", "properties.email.emails.0", "properties.email.emails.0"),
				),
			},
			{
				Config: testConfigNotifier("opsgenie", ` {
					opsgenie = {
						api_key = "fake_opsgenie_api_key"
						is_eu = true
					} }`),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckResourcesCreatesCorrectValues(client, "axiom_notifier.opsgenie", "properties.opsgenie.api_key", "properties.opsgenie.apiKey"),
					testAccCheckResourcesCreatesCorrectValues(client, "axiom_notifier.opsgenie", "properties.opsgenie.is_eu", "properties.opsgenie.isEU"),
				),
			},
			{
				Config: testConfigNotifier("pagerduty", ` {
					pagerduty = {
						routing_key = "fake_routing_key"
						token = "fake_pagerduty_token"
					} }`),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckResourcesCreatesCorrectValues(client, "axiom_notifier.pagerduty", "properties.pagerduty.routing_key", "properties.pagerduty.routingKey"),
					testAccCheckResourcesCreatesCorrectValues(client, "axiom_notifier.pagerduty", "properties.pagerduty.token", "properties.pagerduty.token"),
				),
			},
			{
				Config: testConfigNotifier("webhook", ` {
					webhook = {
						url = "https://example.com/webhook"
					} }`),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckResourcesCreatesCorrectValues(client, "axiom_notifier.webhook", "properties.webhook.url", "properties.webhook.url"),
				),
			},
			{
				Config: testConfigNotifier("custom_webhook", ` {
					custom_webhook = {
						url = "https://example.com/custom_webhook"
						headers = {
							"Authorization" = "Bearer token"
							"Content-Type" = "application/json"
						}
						body = `+`<<EOF
							{
								"action": "{{.Action}}",
								"event": {
								"monitorID": "{{.MonitorID}}",
								"body": "{{.Body}}",
								"description": "{{.Description}}",
								"queryEndTime": "{{.QueryEndTime}}",
								"queryStartTime": "{{.QueryStartTime}}",
								"timestamp": "{{.Timestamp}}",
								"title": "{{.Title}}",
								"value": {{.Value}},
								"matchedEvent": {{jsonObject .MatchedEvent}}
								}
							}
						EOF`+`
					} }`),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckResourcesCreatesCorrectValues(client, "axiom_notifier.custom_webhook", "properties.custom_webhook.url", "properties.customWebhook.url"),
					testAccCheckResourcesCreatesCorrectValues(client, "axiom_notifier.custom_webhook", "properties.custom_webhook.headers.Authorization", "properties.customWebhook.headers.Authorization"),
					testAccCheckResourcesCreatesCorrectValues(client, "axiom_notifier.custom_webhook", "properties.custom_webhook.headers.Content-Type", "properties.customWebhook.headers.Content-Type"),
					testAccCheckResourcesCreatesCorrectValues(client, "axiom_notifier.custom_webhook", "properties.custom_webhook.body", "properties.customWebhook.body"),
				),
			},
		},
	})
}

func testConfigNotifier(notifierType, notifierConfig string) string {
	return `
		provider "axiom" {
			api_token = "` + os.Getenv("AXIOM_TOKEN") + `"
			base_url  = "` + os.Getenv("AXIOM_URL") + `"
		}

		resource "axiom_notifier" "` + notifierType + `" {
			name = "` + notifierType + `"
			properties = ` + notifierConfig + `
		}
	`
}
