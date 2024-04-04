terraform {
  required_providers {
    axiom = {
      source  = "axiomhq/axiom"
      version = "1.0.1"
    }
  }
}

provider "axiom" {
  api_token = "API_TOKEN"
}

resource "axiom_dataset" "test_dataset" {
  name = "test_dataset"
  description = "This is a test dataset created by Terraform."
}

resource "axiom_notifier" "test_slack_notifier" {
  name = "test_slack_notifier"
  properties = {
    slack = {
      slack_url = "SLACK_URL"
    }
  }
}

resource "axiom_notifier" "test_discord_notifier" {
  name = "test_discord_notifier"
  properties = {
    discord = {
      discord_channel = "DISCORD_CHANNEL"
      discord_token = "DISCORD_TOKEN"
    }
  }
}

resource "axiom_notifier" "test_email_notifier" {
  name = "test_email_notifier"
  properties = {
    email= {
      emails = ["EMAIL1","EMAIL2"]
    }
  }
}

resource "axiom_monitor" "test_monitor" {
  depends_on       = [axiom_dataset.test_dataset, axiom_notifier.test_slack_notifier]
  name             = "test_monitor"
  description      = "This is a test monitor created by Terraform."
  apl_query        = "['test_dataset'] | summarize count() by bin_auto(_time)"
  interval_minutes = 5
  operator         = "Above"
  range_minutes    = 5
  threshold        = 1
  notifier_ids = [
    axiom_notifier.test_slack_notifier.id
  ]
  alert_on_no_data = false
  notify_by_group  = false
}

resource "axiom_user" "test_user" {
  name  = "test_user"
  email = "test@abc.com"
  role  = "user"
}