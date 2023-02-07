terraform {
  required_providers {
    axiom = {
      source = "axiom-provider"
    }
  }
}

provider "axiom" {
  token = "..."
  org_id = "..."
}

// create a dataset resource with name and description
resource "axiom_dataset" "testing" {
  name = "testing"
  description = "testing datasets using tf"
}

// create a dataset datasource identifiying it by id
data "axiom_dataset" "testing_ds" {
  id = "testing"
}


// using the previous result of the data source, we can print the name and description of the dataset
output "dataset_name" {
  value = format("%s: %s", data.axiom_dataset.testing_ds.name, data.axiom_dataset.testing_ds.description)
}
