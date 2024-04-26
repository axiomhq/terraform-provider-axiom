# Axiom Provider

A Terraform provider that allows you to manage resources in [Axiom](https://axiom.co/).

Axiom lets you make the most of your event data without compromises: all your data, all the time, for all possible needs. Say goodbye to data sampling, waiting times, and hefty fees.

📖 For more information, see the [documentation](https://axiom.co/docs/apps/terraform).

🔧 To see the provider in action, check out the [example](example/main.tf).

❓ Issues or feedback? [Contact us](https://axiom.co/contact) or [join the Axiom Discord community](https://axiom.co/discord).

For more information about the available resources and data sources, use the left navigation.

## Prerequisites

- [Sign up for a free Axiom account](https://app.axiom.co/register). All you need is an email address.
- [Create an advanced API token in Axiom](https://axiom.co/docs/reference/tokens#create-advanced-api-token) with the permissions to perform the actions you want to use. For example, to use Terraform to create and update datasets, create the advanced API token with these permissions.
- [Create a Terraform account](https://app.terraform.io/signup/account).
- [Install the Terraform CLI](https://developer.hashicorp.com/terraform/cli).

## Install the provider

To install the Axiom Terraform Provider from the [Terraform Registry](https://registry.terraform.io/providers/axiomhq/axiom/latest), follow these steps:

1. Add the following code to your Terraform configuration file. Replace `API_TOKEN` with the Axiom API token you have generated. For added security, store the API token in an environment variable.

    ```hcl
    terraform {
      required_providers {
        axiom = {
          source  = "axiomhq/axiom"
        }
      }
    }

    provider "axiom" {
      api_token = "API_TOKEN"
    }
    ```

2. In your terminal, go to the folder of your main Terraform configuration file, and then run the command `terraform init`.

## Create a dataset

To create a dataset in Axiom using the provider, add the following code to your Terraform configuration file:

```hcl
resource "axiom_dataset" "test_dataset" {
  name = "test_dataset"
  description = "This is a test dataset created by Terraform."
}
```

## Access existing dataset

To access an existing dataset in Axiom using the provider, follow these steps:

1. Determine the ID of the Axiom dataset using the [`getDatasets` query of the Axiom API](https://axiom.co/docs/restapi/endpoints/getDatasets).
2. Add the following code to your Terraform configuration file. Replace `DATASET_ID` with the ID of the Axiom dataset.

```hcl
data "axiom_dataset" "test_dataset" {
  id = "DATASET_ID"
}
```

## Schema

### Required

- `api_token` (String) The Axiom API token.

### Optional

- `base_url` (String) The base url of the axiom api.

## Example

```terraform
terraform {
  required_providers {
    axiom = {
      source  = "axiomhq/axiom"
      version = "1.1.0"
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
```