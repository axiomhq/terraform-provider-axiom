terraform {
  required_providers {
    axiom = {
      source  = "axiomhq/axiom"
      version = "1.0.4"
    }
  }
}

provider "axiom" {
  api_token = "API_TOKEN"
  base_url  = "https://api.axiom.co"
}

// create a dataset resource with name and description
resource "axiom_dataset" "testing_dataset" {
  name        = "created_via_terraform"
  description = "testing datasets using tf"
}

resource "axiom_notifier" "slack_test" {
  name = "slack_test"
  properties = {
    slack = {
      slack_url = "https://hooks.slack.com/services/EXAMPLE/EXAMPLE/EXAMPLE"
    }
    #        discord = {
    #          discord_channel = "https://discord.com/api/webhooks/EXAMPLE/EXAMPLE/EXAMPLE"
    #          discord_token = "EXAMPLE"
    #        }
    #        email= {
    #          emails = ["test","test2"]
    #        }
  }
}

resource "axiom_monitor" "test_monitor" {
  depends_on       = [axiom_dataset.testing_dataset, axiom_notifier.slack_test]
  name             = "test monitor"
  description      = "test_monitor updated"
  apl_query        = <<EOT
['created_via_terraform']
| summarize count() by bin_auto(_time)
EOT
  interval_minutes = 5
  operator         = "Above"
  range_minutes    = 5
  threshold        = 1
  notifier_ids = [
    axiom_notifier.slack_test.id
  ]
  alert_on_no_data = false
  notify_by_group  = false
}

resource "axiom_user" "test_user" {
  name  = "axiom user"
  email = "axiomusers@example.com"
  role  = "user"
}
