package axiom

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/stretchr/testify/assert"
	"github.com/tidwall/gjson"

	ax "github.com/axiomhq/axiom-go/axiom"
)

func TestAccAxiomResources_basic(t *testing.T) {
	client, err := ax.NewClient()
	assert.NoError(t, err)

	resource.Test(t, resource.TestCase{
		PreCheck: func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"axiom": providerserver.NewProtocol6WithError(NewAxiomProvider()),
		},
		CheckDestroy: testAccCheckAxiomResourcesDestroyed(client),
		Steps: []resource.TestStep{
			{
				Config: testAccAxiomDatasetConfig_basic(),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAxiomResourcesExist(client, "axiom_dataset.test"),
					testAccCheckAxiomResourcesExist(client, "axiom_dataset.test_without_description"),
					resource.TestCheckResourceAttr("axiom_dataset.test", "name", "terraform-provider-dataset"),
					resource.TestCheckResourceAttr("axiom_dataset.test", "description", "A test dataset"),
					resource.TestCheckResourceAttr("axiom_dataset.test", "use_retention_period", "false"),
					resource.TestCheckResourceAttr("axiom_dataset.test", "retention_days", "0"),
					resource.TestCheckResourceAttr("axiom_virtual_field.test", "name", "VF"),
					resource.TestCheckResourceAttr("axiom_virtual_field.test", "description", "my virtual field"),
					resource.TestCheckResourceAttr("axiom_virtual_field.test", "expression", "a * b"),
					testAccCheckAxiomResourcesExist(client, "axiom_monitor.test_monitor"),
					testAccCheckAxiomResourcesExist(client, "axiom_monitor.test_monitor_without_description"),
					resource.TestCheckResourceAttr("axiom_monitor.test_monitor", "name", "test monitor"),
					testAccCheckAxiomResourcesExist(client, "axiom_notifier.slack_test"),
					resource.TestCheckResourceAttr("axiom_notifier.slack_test", "name", "slack_test"),
					testAccCheckAxiomResourcesExist(client, "axiom_token.test_token"),
					testAccCheckAxiomResourcesExist(client, "axiom_token.test_token_without_description"),
					resource.TestCheckResourceAttr("axiom_token.test_token", "name", "test_token"),
					resource.TestCheckResourceAttr("axiom_token.test_token", "description", "test_token"),
					resource.TestCheckResourceAttr("axiom_token.test_token", "expires_at", "2027-06-29T13:02:54Z"),
					resource.TestCheckResourceAttr("axiom_token.test_token", "dataset_capabilities.new-dataset.ingest.0", "create"),
					resource.TestCheckResourceAttr("axiom_token.test_token", "org_capabilities.api_tokens.0", "read"),
					testAccCheckAxiomResourcesExist(client, "axiom_token.dataset_token"),
					resource.TestCheckResourceAttr("axiom_token.dataset_token", "name", "dataset only token"),
					resource.TestCheckResourceAttr("axiom_token.dataset_token", "description", "Can only access a single dataset"),
					resource.TestCheckResourceAttr("axiom_token.dataset_token", "expires_at", "2027-06-29T13:02:54Z"),
					resource.TestCheckResourceAttr("axiom_token.dataset_token", "dataset_capabilities.new-dataset.ingest.0", "create"),
					resource.TestCheckResourceAttr("axiom_token.dataset_token", "dataset_capabilities.new-dataset.query.0", "read"),
				),
			},
		},
	})
}

func TestAccAxiomResources_data(t *testing.T) {
	client, err := ax.NewClient()
	assert.NoError(t, err)

	emailToAssert := "test@axiom.co"
	n, err := client.Notifiers.Create(context.Background(), ax.Notifier{Name: "my notifier", Properties: ax.NotifierProperties{Email: &ax.EmailConfig{
		Emails: []string{emailToAssert},
	}}})
	assert.NoError(t, err)

	defer func() {
		assert.NoError(t, client.Notifiers.Delete(context.Background(), n.ID))
	}()

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"axiom": providerserver.NewProtocol6WithError(NewAxiomProvider()),
		},
		CheckDestroy: testAccCheckAxiomResourcesDestroyed(client),
		Steps: []resource.TestStep{
			{
				Config: `
					provider "axiom" {
						api_token = "` + os.Getenv("AXIOM_TOKEN") + `"
						base_url  = "` + os.Getenv("AXIOM_URL") + `"
					}

					data "axiom_notifier" "my-notifier" {
						id = "` + n.ID + `"
					}
				`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.axiom_notifier.my-notifier", "properties.email.emails.#", "1"),
					resource.TestCheckResourceAttr("data.axiom_notifier.my-notifier", "properties.email.emails.0", emailToAssert),
				),
			},
		},
	})
}

func TestAccAxiomResources_resolvable(t *testing.T) {
	client, err := ax.NewClient()
	assert.NoError(t, err)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"axiom": providerserver.NewProtocol6WithError(NewAxiomProvider()),
		},
		CheckDestroy: testAccCheckAxiomResourcesDestroyed(client),
		Steps: []resource.TestStep{
			{
				Config: `
					provider "axiom" {
						api_token = "` + os.Getenv("AXIOM_TOKEN") + `"
						base_url  = "` + os.Getenv("AXIOM_URL") + `"
					}

					resource "axiom_dataset" "test" {
						name        = "new-dataset"
						description = "A test dataset"
					}

					resource "axiom_monitor" "new_monitor" {
						depends_on       = [axiom_dataset.test]

						name             = "test monitor"
						description      = "new_monitor updated"
						apl_query        = <<EOT
							['new-dataset']
							| summarize count()
							EOT
						interval_minutes = 5
						operator         = "Above"
						range_minutes    = 5
						threshold        = 1
						alert_on_no_data = false
						notify_by_group  = true
						resolvable 		 = true
						type = "Threshold"
					}
				`,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckResourcesCreatesCorrectValues(client, "axiom_monitor.new_monitor", "resolvable", "resolvable"),
					testAccCheckResourcesCreatesCorrectValues(client, "axiom_monitor.new_monitor", "notify_by_group", "notifyByGroup"),
				),
			},
		},
	})
}

// TestAccAxiom_RecreateAfterAPIDeletion tests the behavior of the Axiom provider
// when a monitor is deleted via the Axiom API (outside Terraform) and then Terraform
// attempts to reapply the configuration.
// Note: The Axiom provider currently fails with a 404 error when the monitor is deleted
// outside Terraform, which is not ideal. It should handle 404 errors by marking the resource
// as absent and allowing Terraform to recreate it.
func TestAccAxiomResources_recreate_after_api_deletion(t *testing.T) {
	client, err := ax.NewClient()
	assert.NoError(t, err)

	// Define the dataset and monitor names for consistent reference
	datasetName := "new-dataset-recreate-" + uuid.NewString() // Add a random suffix to avoid conflicts
	monitorName := "test-monitor-recreate-" + uuid.NewString()

	// Define the Terraform configuration for the test
	config := `
		provider "axiom" {
			api_token = "` + os.Getenv("AXIOM_TOKEN") + `"
			base_url  = "` + os.Getenv("AXIOM_URL") + `"
		}

		resource "axiom_dataset" "test" {
			name        = "` + datasetName + `"
			description = "A test dataset for recreate test"
		}

		resource "axiom_monitor" "test_monitor" {
			depends_on       = [axiom_dataset.test]

			name             = "` + monitorName + `"
			description      = "Monitor for recreate test"
			apl_query        = <<EOT
				['` + datasetName + `']
				| summarize count()
				EOT
			interval_minutes = 5
			operator         = "Above"
			range_minutes    = 5
			threshold        = 1
			alert_on_no_data = false
			notify_by_group  = true
			resolvable       = true
			type             = "Threshold"
		}
	`

	deleteMonitorViaAPI := func(client *ax.Client, monitorName string) func() {
		return func() {
			// Find the monitor by name
			monitors, err := client.Monitors.List(context.Background())
			if err != nil {
				t.Fatalf("failed to list monitors: %v", err)
			}

			var monitorID string
			for _, m := range monitors {
				if m.Name == monitorName {
					monitorID = m.ID
					break
				}
			}

			if monitorID == "" {
				t.Fatalf("monitor with name %s not found", monitorName)
			}

			// Delete the monitor via the Axiom API
			err = client.Monitors.Delete(context.Background(), monitorID)
			if err != nil {
				t.Fatalf("failed to delete monitor via API: %v", err)
			}
		}
	}

	deleteDatasetViaAPI := func(client *ax.Client, datasetName string) func() {
		return func() {
			// Delete the dataset via the Axiom API using the dataset name as the ID
			err := client.Datasets.Delete(context.Background(), datasetName)
			if err != nil {
				t.Fatalf("failed to delete dataset via API: %v", err)
			}
		}
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"axiom": providerserver.NewProtocol6WithError(NewAxiomProvider()),
		},
		CheckDestroy: testAccCheckAxiomResourcesDestroyed(client),
		Steps: []resource.TestStep{
			// Step 1: Create the dataset and monitor
			{
				Config: config,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("axiom_monitor.test_monitor", "name", monitorName),
					resource.TestCheckResourceAttr("axiom_monitor.test_monitor", "description", "Monitor for recreate test"),
				),
			},
			// Step 2: Delete the monitor via the Axiom API and reapply
			{
				PreConfig: func() {
					// Delete the monitor via the Axiom API
					deleteMonitorViaAPI(client, monitorName)()
				},
				Config: config,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("axiom_monitor.test_monitor", "name", monitorName),
					resource.TestCheckResourceAttr("axiom_monitor.test_monitor", "description", "Monitor for recreate test"),
				),
			},
			// Step 3: Delete the dataset (which will delete the monitor too)
			{
				PreConfig: func() {
					// Delete the monitor via the Axiom API
					deleteDatasetViaAPI(client, datasetName)()
				},
				Config: config,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("axiom_dataset.test", "id", datasetName),
					resource.TestCheckResourceAttr("axiom_monitor.test_monitor", "name", monitorName),
					resource.TestCheckResourceAttr("axiom_monitor.test_monitor", "description", "Monitor for recreate test"),
				),
			},
		},
	})
}

func TestAccAxiomResources_capabilities(t *testing.T) {
	client, err := ax.NewClient()
	assert.NoError(t, err)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"axiom": providerserver.NewProtocol6WithError(NewAxiomProvider()),
		},
		CheckDestroy: testAccCheckAxiomResourcesDestroyed(client),
		Steps: []resource.TestStep{
			{
				Config: `
					provider "axiom" {
						api_token = "` + os.Getenv("AXIOM_TOKEN") + `"
						base_url  = "` + os.Getenv("AXIOM_URL") + `"
					}

					resource "axiom_token" "test_token" {
						name        = "test-token-org-capabilities"
						description = "Test token with org capabilities"
						org_capabilities = {
							datasets = []
							api_tokens = []
						}
					}
				`,
				ExpectError: regexp.MustCompile(`at\sleast\sone\scapability\smust\sbe\sset`),
			},
			{
				Config: `
					provider "axiom" {
						api_token = "` + os.Getenv("AXIOM_TOKEN") + `"
						base_url  = "` + os.Getenv("AXIOM_URL") + `"
					}

					resource "axiom_token" "test_token" {
						name        = "test-token-dataset-capabilities"
						description = "Test token with dataset capabilities"
						dataset_capabilities = {
							"new-dataset" = {
								query  = []
							}
						}
					}
				`,
				ExpectError: regexp.MustCompile(`At\sleast\sone\sdataset\scapability\smust\sbe\sset`),
			},
		},
	})
}

func TestAccAxiomResources_dataset_retention(t *testing.T) {
	client, err := ax.NewClient()
	assert.NoError(t, err)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"axiom": providerserver.NewProtocol6WithError(NewAxiomProvider()),
		},
		CheckDestroy: testAccCheckAxiomResourcesDestroyed(client),
		Steps: []resource.TestStep{
			// Step 1: Create the dataset with default retention settings
			{
				Config: `
					provider "axiom" {
						api_token = "` + os.Getenv("AXIOM_TOKEN") + `"
						base_url  = "` + os.Getenv("AXIOM_URL") + `"
					}

					resource "axiom_dataset" "test" {
						name        = "new-dataset"
						description = "A test dataset"
					}
				`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("axiom_dataset.test", "id", "new-dataset"),
					resource.TestCheckResourceAttr("axiom_dataset.test", "description", "A test dataset"),
					resource.TestCheckResourceAttr("axiom_dataset.test", "use_retention_period", "false"),
					resource.TestCheckResourceAttr("axiom_dataset.test", "retention_days", "0"),
				),
			},
			// Step 2: Update the dataset to enable retention period
			{
				Config: `
					provider "axiom" {
						api_token = "` + os.Getenv("AXIOM_TOKEN") + `"
						base_url  = "` + os.Getenv("AXIOM_URL") + `"
					}

					resource "axiom_dataset" "test" {
						name        = "new-dataset"
						use_retention_period = true
						retention_days = 30
					}
				`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("axiom_dataset.test", "id", "new-dataset"),
					resource.TestCheckResourceAttr("axiom_dataset.test", "description", "A test dataset"),
					resource.TestCheckResourceAttr("axiom_dataset.test", "use_retention_period", "true"),
					resource.TestCheckResourceAttr("axiom_dataset.test", "retention_days", "30"),
				),
			},
			// Step 3: Just update the description (should not affect retention period settings)
			{
				Config: `
					provider "axiom" {
						api_token = "` + os.Getenv("AXIOM_TOKEN") + `"
						base_url  = "` + os.Getenv("AXIOM_URL") + `"
					}

					resource "axiom_dataset" "test" {
						name        = "new-dataset"
						description = "Updated description for a test dataset to check retention period settings"
					}
				`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("axiom_dataset.test", "id", "new-dataset"),
					resource.TestCheckResourceAttr("axiom_dataset.test", "description", "Updated description for a test dataset to check retention period settings"),
					resource.TestCheckResourceAttr("axiom_dataset.test", "use_retention_period", "true"),
					resource.TestCheckResourceAttr("axiom_dataset.test", "retention_days", "30"),
				),
			},
			// Step 4: Update the retention period on the dataset to be 0 days (should result in an error as use_retention_period is true)
			{
				Config: `
					provider "axiom" {
						api_token = "` + os.Getenv("AXIOM_TOKEN") + `"
						base_url  = "` + os.Getenv("AXIOM_URL") + `"
					}

					resource "axiom_dataset" "test" {
						name        = "new-dataset"
						retention_days = 0
					}
				`,
				ExpectError: regexp.MustCompile(`Retention\sdays\smust\sbe\sgreater\sthan\s0\swhen\suse_retention_period\sis\strue`),
			},
		},
	})
}

func TestAccAxiomResources_dataset_object_fields(t *testing.T) {
	client, err := ax.NewClient()
	assert.NoError(t, err)

	// Define the dataset name for consistent reference
	datasetName := "new-dataset-objectfields-" + uuid.NewString() // Add a random suffix to avoid conflicts

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"axiom": providerserver.NewProtocol6WithError(NewAxiomProvider()),
		},
		CheckDestroy: testAccCheckAxiomResourcesDestroyed(client),
		Steps: []resource.TestStep{
			// Step 1: Create the dataset with some object fields
			{
				Config: `
					provider "axiom" {
						api_token = "` + os.Getenv("AXIOM_TOKEN") + `"
						base_url  = "` + os.Getenv("AXIOM_URL") + `"
					}

					resource "axiom_dataset" "test" {
						name        = "` + datasetName + `"
						description = "A test dataset"
						object_fields = ["field1", "field2"]
					}
				`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("axiom_dataset.test", "id", datasetName),
					resource.TestCheckResourceAttr("axiom_dataset.test", "description", "A test dataset"),
					resource.TestCheckResourceAttr("axiom_dataset.test", "object_fields.#", "2"),
					resource.TestCheckResourceAttr("axiom_dataset.test", "object_fields.0", "field1"),
					resource.TestCheckResourceAttr("axiom_dataset.test", "object_fields.1", "field2"),
				),
			},
			// Step 2: Update the dataset to add more object fields
			{
				Config: `
					provider "axiom" {
						api_token = "` + os.Getenv("AXIOM_TOKEN") + `"
						base_url  = "` + os.Getenv("AXIOM_URL") + `"
					}

					resource "axiom_dataset" "test" {
						name        = "` + datasetName + `"
						object_fields = ["field1", "field2", "field3", "field4"]
					}
				`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("axiom_dataset.test", "id", datasetName),
					resource.TestCheckResourceAttr("axiom_dataset.test", "object_fields.#", "4"),
					resource.TestCheckResourceAttr("axiom_dataset.test", "object_fields.0", "field1"),
					resource.TestCheckResourceAttr("axiom_dataset.test", "object_fields.1", "field2"),
					resource.TestCheckResourceAttr("axiom_dataset.test", "object_fields.2", "field3"),
					resource.TestCheckResourceAttr("axiom_dataset.test", "object_fields.3", "field4"),
				),
			},
			// Step 3: Just update the description (should not affect object fields)
			{
				Config: `
					provider "axiom" {
						api_token = "` + os.Getenv("AXIOM_TOKEN") + `"
						base_url  = "` + os.Getenv("AXIOM_URL") + `"
					}

					resource "axiom_dataset" "test" {
						name        = "` + datasetName + `"
						description = "Updated description for a test dataset to check object fields"
					}
				`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("axiom_dataset.test", "id", datasetName),
					resource.TestCheckResourceAttr("axiom_dataset.test", "description", "Updated description for a test dataset to check object fields"),
					resource.TestCheckResourceAttr("axiom_dataset.test", "object_fields.#", "4"),
					resource.TestCheckResourceAttr("axiom_dataset.test", "object_fields.0", "field1"),
					resource.TestCheckResourceAttr("axiom_dataset.test", "object_fields.1", "field2"),
					resource.TestCheckResourceAttr("axiom_dataset.test", "object_fields.2", "field3"),
					resource.TestCheckResourceAttr("axiom_dataset.test", "object_fields.3", "field4"),
				),
			},
			// Step 4: Update again to remove all object fields
			{
				Config: `
					provider "axiom" {
						api_token = "` + os.Getenv("AXIOM_TOKEN") + `"
						base_url  = "` + os.Getenv("AXIOM_URL") + `"
					}

					resource "axiom_dataset" "test" {
						name        = "` + datasetName + `"
						object_fields = []
					}
				`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("axiom_dataset.test", "id", datasetName),
					resource.TestCheckResourceAttr("axiom_dataset.test", "object_fields.#", "0"),
				),
			},
		},
	})
}

func testAccPreCheck(t *testing.T) {
	if os.Getenv("AXIOM_TOKEN") == "" {
		t.Fatalf("AXIOM_TOKEN must be set for acceptance tests")
	}
}

func testAccCheckAxiomResourcesDestroyed(client *ax.Client) func(s *terraform.State) error {
	return func(s *terraform.State) error {
		for id, resource := range s.RootModule().Resources {
			if strings.HasPrefix(id, "data.") {
				continue
			}
			var err error
			switch resource.Type {
			case "axiom_notifier":
				_, err = client.Notifiers.Get(context.Background(), resource.Primary.ID)
			case "axiom_dataset":
				_, err = client.Datasets.Get(context.Background(), resource.Primary.ID)
			case "axiom_monitor":
				_, err = client.Monitors.Get(context.Background(), resource.Primary.ID)
			case "axiom_token":
				_, err = client.Tokens.Get(context.Background(), resource.Primary.ID)
			case "axiom_virtual_field":
				_, err = client.VirtualFields.Get(context.Background(), resource.Primary.ID)
			}
			datasetErr, ok := err.(ax.HTTPError)
			if !ok {
				return fmt.Errorf("could not assert error for %s as ax.HTTPError: %T", id, err)
			}
			if datasetErr.Status != http.StatusNotFound {
				return fmt.Errorf("http error incorrect for %s GET: %v", id, datasetErr.Status)
			}
		}

		return nil
	}
}

func testAccCheckAxiomResourcesExist(client *ax.Client, resourceName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource not found: %s", resourceName)
		}

		var err error
		switch rs.Type {
		case "axiom_notifier":
			_, err = client.Notifiers.Get(context.Background(), rs.Primary.ID)
		case "axiom_dataset":
			_, err = client.Datasets.Get(context.Background(), rs.Primary.ID)
		case "axiom_monitor":
			_, err = client.Monitors.Get(context.Background(), rs.Primary.ID)
		case "axiom_token":
			_, err = client.Tokens.Get(context.Background(), rs.Primary.ID)
		case "axiom_virtual_field":
			_, err = client.VirtualFields.Get(context.Background(), rs.Primary.ID)
		}

		if err != nil {
			fmt.Println("testAccCheckAxiomResourcesExist", resourceName, err)
		}

		return nil
	}
}

func testAccCheckResourcesCreatesCorrectValues(client *ax.Client, resourceName, tfKey, apiKey string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource not found: %s", resourceName)
		}

		var err error
		var actual any
		switch rs.Type {
		case "axiom_notifier":
			actual, err = client.Notifiers.Get(context.Background(), rs.Primary.ID)
		case "axiom_dataset":
			actual, err = client.Datasets.Get(context.Background(), rs.Primary.ID)
		case "axiom_monitor":
			actual, err = client.Monitors.Get(context.Background(), rs.Primary.ID)
		case "axiom_token":
			actual, err = client.Tokens.Get(context.Background(), rs.Primary.ID)
		case "axiom_virtual_field":
			actual, err = client.VirtualFields.Get(context.Background(), rs.Primary.ID)
		}
		if err != nil {
			return fmt.Errorf("error fetching %s from Axiom: %s", rs.Type, err)
		}

		actualJSON, err := json.Marshal(actual)
		if err != nil {
			return fmt.Errorf("error marshaling actual object to JSON: %s", err)
		}

		// Unmarshal JSON into a map for easy comparison
		var actualMap map[string]interface{}
		err = json.Unmarshal(actualJSON, &actualMap)
		if err != nil {
			return fmt.Errorf("error unmarshaling JSON to map: %s", err)
		}

		// Loop through properties to compare them
		stateValue, found := rs.Primary.Attributes[tfKey]
		if !found {
			return fmt.Errorf("property %s not found in Terraform state", tfKey)
		}

		// Use gjson to get the value using the dot notation path
		actualValue := gjson.GetBytes(actualJSON, apiKey)

		if !actualValue.Exists() {
			return fmt.Errorf("property %s not found in API response: %s", apiKey, string(actualJSON))
		}

		if fmt.Sprintf("%v", actualValue) != stateValue {
			return fmt.Errorf("mismatch for %s|%s: expected %s, got %v", tfKey, apiKey, stateValue, actualValue)
		}
		return nil
	}
}

func testAccAxiomDatasetConfig_basic() string {
	return `
provider "axiom" {
  api_token = "` + os.Getenv("AXIOM_TOKEN") + `"
  base_url  = "` + os.Getenv("AXIOM_URL") + `"
}

resource "axiom_dataset" "test" {
  name        = "terraform-provider-dataset"
  description = "A test dataset"
}

resource "axiom_virtual_field" "test" {
  depends_on = [axiom_dataset.test]  
  name        = "VF"
  description = "my virtual field"
  expression = "a * b"
  dataset = axiom_dataset.test.id
}

resource "axiom_dataset" "test_without_description" {
  name = "terraform-provider-dataset-without-description"
}

resource "axiom_notifier" "slack_test" {
  name = "slack_test"
  properties = {
    slack = {
      slack_url = "https://hooks.slack.com/services/EXAMPLE/EXAMPLE/EXAMPLE"
    }
  }
}

resource "axiom_monitor" "test_monitor" {
  depends_on = [axiom_dataset.test, axiom_notifier.slack_test]

  name             = "test monitor"
  description      = "test_monitor updated"
  apl_query        = <<EOT
			['terraform-provider-dataset']
			| summarize count()
			EOT
  interval_minutes = 5
  operator         = "Above"
  range_minutes    = 5
  threshold        = 1
  notifier_ids = [
    axiom_notifier.slack_test.id
  ]
  type			   = "Threshold"
  alert_on_no_data = false
  notify_by_group  = false
}

resource "axiom_monitor" "test_monitor_without_description" {
  depends_on = [axiom_dataset.test, axiom_notifier.slack_test]

  name             = "test monitor without description"
  apl_query        = <<EOT
			['terraform-provider-dataset']
			| summarize count()
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
  type = "Threshold" 
}

resource "axiom_monitor" "test_monitor_match_event" {
  depends_on = [axiom_dataset.test, axiom_notifier.slack_test]

  name             = "test event matching monitor"
  description      = "this is a match event monitor so can't contain summarize"
  apl_query        = <<EOT
			['terraform-provider-dataset']
			EOT
  notifier_ids = [
    axiom_notifier.slack_test.id
  ]
  type = "MatchEvent"
}

resource "axiom_token" "test_token" {
  name        = "test_token"
  description = "test_token"
  expires_at  = "2027-06-29T13:02:54Z"
  dataset_capabilities = {
    "new-dataset" = {
      ingest = ["create"],
      query  = ["read"]
    }
  }
  org_capabilities = {
    api_tokens = ["read"]
  }
}

resource "axiom_token" "test_token_without_description" {
  name        = "test_token_without_description"
  expires_at  = "2027-06-29T13:02:54Z"
  dataset_capabilities = {
    "new-dataset" = {
      ingest = ["create"],
      query  = ["read"]
    }
  }
  org_capabilities = {
    api_tokens = ["read"]
  }
}

resource "axiom_token" "dataset_token" {
  name        = "dataset only token"
  description = "Can only access a single dataset"
  expires_at  = "2027-06-29T13:02:54Z"
  dataset_capabilities = {
    "new-dataset" = {
      ingest = ["create"],
      query  = ["read"]
    }
  }
}
`
}
