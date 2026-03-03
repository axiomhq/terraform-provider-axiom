package axiom

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/stretchr/testify/assert"

	ax "github.com/axiomhq/axiom-go/axiom"
)

func TestAccAxiomDashboardResource_WithProvidedUID(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("acceptance tests skipped unless TF_ACC is set")
	}
	testAccPreCheck(t)

	client, err := ax.NewClient()
	assert.NoError(t, err)

	uid := "tf-dashboard-" + strings.ReplaceAll(uuid.NewString(), "_", "-")
	name := "dash-provided-" + uuid.NewString()
	updatedName := name + "-updated"

	resourceName := "axiom_dashboard.test"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"axiom": providerserver.NewProtocol6WithError(NewAxiomProvider()),
		},
		CheckDestroy: testAccCheckAxiomResourcesDestroyed(client),
		Steps: []resource.TestStep{
			{
				Config: testAccAxiomDashboardConfig(uid, name, false),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAxiomResourcesExist(client, resourceName),
					resource.TestCheckResourceAttr(resourceName, "uid", uid),
					resource.TestCheckResourceAttrSet(resourceName, "id"),
				),
			},
			{
				Config: testAccAxiomDashboardConfig(uid, updatedName, false),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAxiomResourcesExist(client, resourceName),
					resource.TestCheckResourceAttr(resourceName, "uid", uid),
					resource.TestCheckResourceAttrWith(resourceName, "dashboard", func(v string) error {
						if !strings.Contains(v, updatedName) {
							return fmt.Errorf("expected updated dashboard name in dashboard JSON, got %s", v)
						}
						return nil
					}),
				),
			},
			{
				ResourceName:      resourceName,
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateId:     uid,
				ImportStateVerifyIgnore: []string{
					"dashboard",
				},
			},
		},
	})
}

func TestAccAxiomDashboardResource_ServerGeneratedUID(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("acceptance tests skipped unless TF_ACC is set")
	}
	testAccPreCheck(t)

	client, err := ax.NewClient()
	assert.NoError(t, err)

	name := "dash-generated-" + uuid.NewString()
	updatedName := name + "-updated"
	resourceName := "axiom_dashboard.test"

	var generatedUID string

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"axiom": providerserver.NewProtocol6WithError(NewAxiomProvider()),
		},
		CheckDestroy: testAccCheckAxiomResourcesDestroyed(client),
		Steps: []resource.TestStep{
			{
				Config: testAccAxiomDashboardConfigWithoutUID(name, false),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAxiomResourcesExist(client, resourceName),
					resource.TestCheckResourceAttrSet(resourceName, "uid"),
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					testAccCaptureDashboardUID(resourceName, &generatedUID),
				),
			},
			{
				Config: testAccAxiomDashboardConfigWithoutUID(updatedName, true),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAxiomResourcesExist(client, resourceName),
					resource.TestCheckResourceAttrWith(resourceName, "uid", func(v string) error {
						if generatedUID == "" {
							return fmt.Errorf("expected captured uid to be non-empty")
						}
						if v != generatedUID {
							return fmt.Errorf("expected server-generated uid to stay stable, want %s got %s", generatedUID, v)
						}
						return nil
					}),
					resource.TestCheckResourceAttrWith(resourceName, "dashboard", func(v string) error {
						if !strings.Contains(v, updatedName) {
							return fmt.Errorf("expected updated dashboard name in dashboard JSON, got %s", v)
						}
						return nil
					}),
				),
			},
			{
				ResourceName: resourceName,
				ImportState:  true,
				ImportStateIdFunc: func(*terraform.State) (string, error) {
					if generatedUID == "" {
						return "", fmt.Errorf("generated uid was not captured")
					}
					return generatedUID, nil
				},
				ImportStateVerify: true,
				ImportStateVerifyIgnore: []string{
					"dashboard",
					"overwrite",
				},
			},
		},
	})
}

func TestAccAxiomDashboardResource_UIDInDashboardDocument(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("acceptance tests skipped unless TF_ACC is set")
	}
	testAccPreCheck(t)

	client, err := ax.NewClient()
	assert.NoError(t, err)

	uid := "tf-dashboard-doc-uid-" + strings.ReplaceAll(uuid.NewString(), "_", "-")
	resourceName := "axiom_dashboard.test"

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"axiom": providerserver.NewProtocol6WithError(NewAxiomProvider()),
		},
		CheckDestroy: testAccCheckAxiomResourcesDestroyed(client),
		Steps: []resource.TestStep{
			{
				Config: testAccAxiomDashboardConfigDocumentUID(uid, "doc-uid-create", false, true),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAxiomResourcesExist(client, resourceName),
					resource.TestCheckResourceAttr(resourceName, "uid", uid),
				),
			},
			{
				Config: testAccAxiomDashboardConfigDocumentUID(uid, "doc-uid-update", false, false),
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAxiomResourcesExist(client, resourceName),
					resource.TestCheckResourceAttr(resourceName, "uid", uid),
					resource.TestCheckResourceAttrWith(resourceName, "dashboard", func(v string) error {
						if strings.Contains(v, `"uid"`) {
							return fmt.Errorf("expected dashboard JSON in state to omit uid when not configured, got %s", v)
						}
						return nil
					}),
				),
			},
		},
	})
}

func testAccAxiomDashboardConfig(uid, name string, overwrite bool) string {
	return fmt.Sprintf(`
provider "axiom" {
  api_token = %q
  base_url  = %q
}

resource "axiom_dashboard" "test" {
  uid       = %q
  overwrite = %t
  dashboard = jsonencode({
    name            = %q
    description     = "terraform acceptance dashboard"
    refreshTime     = 60
    schemaVersion   = 2
    timeWindowStart = "qr-now-1h"
    timeWindowEnd   = "qr-now"
    charts          = []
    layout          = []
  })
}
`, os.Getenv("AXIOM_TOKEN"), os.Getenv("AXIOM_URL"), uid, overwrite, name)
}

func testAccAxiomDashboardConfigWithoutUID(name string, overwrite bool) string {
	return fmt.Sprintf(`
provider "axiom" {
  api_token = %q
  base_url  = %q
}

resource "axiom_dashboard" "test" {
  overwrite = %t
  dashboard = jsonencode({
    name            = %q
    description     = "terraform acceptance dashboard"
    refreshTime     = 60
    schemaVersion   = 2
    timeWindowStart = "qr-now-1h"
    timeWindowEnd   = "qr-now"
    charts          = []
    layout          = []
  })
}
`, os.Getenv("AXIOM_TOKEN"), os.Getenv("AXIOM_URL"), overwrite, name)
}

func testAccAxiomDashboardConfigDocumentUID(uid, name string, overwrite, includeUIDInDocument bool) string {
	uidField := ""
	if includeUIDInDocument {
		uidField = fmt.Sprintf("uid             = %q\n", uid)
	}

	return fmt.Sprintf(`
provider "axiom" {
  api_token = %q
  base_url  = %q
}

resource "axiom_dashboard" "test" {
  overwrite = %t
  dashboard = jsonencode({
    %sname            = %q
    description     = "terraform acceptance dashboard"
    refreshTime     = 60
    schemaVersion   = 2
    timeWindowStart = "qr-now-1h"
    timeWindowEnd   = "qr-now"
    charts          = []
    layout          = []
  })
}
`, os.Getenv("AXIOM_TOKEN"), os.Getenv("AXIOM_URL"), overwrite, uidField, name)
}

func testAccCaptureDashboardUID(resourceName string, out *string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[resourceName]
		if !ok {
			return fmt.Errorf("resource not found: %s", resourceName)
		}

		uid := rs.Primary.Attributes["uid"]
		if uid == "" {
			return fmt.Errorf("resource %s has empty uid", resourceName)
		}

		*out = uid
		return nil
	}
}
