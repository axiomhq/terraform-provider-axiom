# Axiom Terraform Provider

The axiom providers allows you to create and manage datasets in Axiom.

## Usage

import the provider

```hcl
terraform {
  required_providers {
    axiom = {
      source = "axiom-provider"
    }
  }
}
```

configure the provider with your personal API token and organization ID

```hcl
provider "axiom" {
  token = "your_personal_api_token_here"
  org_id = "organization_id_here"
}
```

finally, create dataset resources

```hcl
resource "axiom_dataset" "example" {
  name = "example"
  description = "an example dataset created by terraform"
}
```

or, create a dataset datasource to reference an existing dataset

```hcl
data "axiom_dataset" "testing_ds" {
  id = "testing"
}
```

For more examples, checkout the [playground directory](playground/main.tf)


## Development

The axiom provider utilizes axiom-go sdk under the hood.
