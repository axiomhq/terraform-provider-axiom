package axiom

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/stretchr/testify/assert"

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
					resource.TestCheckResourceAttr("axiom_dataset.test", "name", "test-dataset"),
					resource.TestCheckResourceAttr("axiom_dataset.test", "description", "A test dataset"),
					testAccCheckAxiomResourcesExist(client, "axiom_monitor.test_monitor"),
					resource.TestCheckResourceAttr("axiom_monitor.test_monitor", "name", "test monitor"),
					testAccCheckAxiomResourcesExist(client, "axiom_notifier.slack_test"),
					resource.TestCheckResourceAttr("axiom_notifier.slack_test", "name", "slack_test"),
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
		for id, resource := range s.Modules[0].Resources {
			var err error
			switch resource.Type {
			case "axiom_notifier":
				_, err = client.Notifiers.Get(context.Background(), resource.Primary.ID)
			case "axiom_dataset":
				_, err = client.Datasets.Get(context.Background(), resource.Primary.ID)
			case "axiom_monitor":
				_, err = client.Monitors.Get(context.Background(), resource.Primary.ID)
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

func testAccCheckAxiomResourcesExist(client *ax.Client, n string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		for _, resource := range s.Modules[0].Resources {
			var err error
			switch resource.Type {
			case "axiom_notifier":
				_, err = client.Notifiers.Get(context.Background(), resource.Primary.ID)
			case "axiom_dataset":
				_, err = client.Datasets.Get(context.Background(), resource.Primary.ID)
			case "axiom_monitor":
				_, err = client.Monitors.Get(context.Background(), resource.Primary.ID)
			}
			return err
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
		name        = "test-dataset"
		description = "A test dataset"
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
		depends_on       = [axiom_dataset.test, axiom_notifier.slack_test]

		name             = "test monitor"
		description      = "test_monitor updated"
		apl_query        = <<EOT
			['test-dataset']
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
		resolvable       = false
	}
`
}

func TestAccAxiomResources_data(t *testing.T) {
	client, err := ax.NewClient()
	assert.NoError(t, err)

	emailToAssert := "test@axiom.co"
	n, err := client.Notifiers.Create(context.Background(), ax.Notifier{Name: "my notifier", Properties: ax.NotifierProperties{Email: &ax.EmailConfig{
		Emails: []string{emailToAssert},
	}}})

	defer func() {
		assert.NoError(t, client.Notifiers.Delete(context.Background(), n.ID))
	}()

	assert.NoError(t, err)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"axiom": providerserver.NewProtocol6WithError(NewAxiomProvider()),
		},
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
