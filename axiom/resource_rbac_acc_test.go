package axiom

import (
	"context"
	"errors"
	"net/http"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/stretchr/testify/require"

	ax "github.com/axiomhq/axiom-go/axiom"
)

// testAccPreCheckRBAC skips the test when the organization lacks the RBAC
// add-on, which the API reports as 403 on role creation.
func testAccPreCheckRBAC(t *testing.T, client *ax.Client) {
	testAccPreCheck(t)

	role, err := client.Roles.Create(context.Background(), ax.RoleRequest{
		Name: "terraform-provider-rbac-probe-" + uuid.NewString()[:8],
	})
	if httpErr := new(ax.HTTPError); errors.As(err, httpErr) && httpErr.Status == http.StatusForbidden {
		t.Skipf("RBAC is not available for this organization: %s", err)
	}
	require.NoError(t, err)
	require.NoError(t, client.Roles.Delete(context.Background(), role.ID))
}

func TestAccAxiomResources_rbac(t *testing.T) {
	if os.Getenv(resource.EnvTfAcc) == "" {
		t.Skipf("Acceptance tests skipped unless env '%s' set", resource.EnvTfAcc)
	}

	client, err := ax.NewClient()
	require.NoError(t, err)

	suffix := uuid.NewString()[:8]
	provider := `
provider "axiom" {
  api_token = "` + os.Getenv("AXIOM_TOKEN") + `"
  base_url  = "` + os.Getenv("AXIOM_URL") + `"
}
`

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheckRBAC(t, client) },
		ProtoV6ProviderFactories: testRBACProviderFactories,
		CheckDestroy:             testAccCheckAxiomResourcesDestroyed(client),
		Steps: []resource.TestStep{
			{
				Config: provider + `
resource "axiom_role" "test" {
  name = "terraform-provider-role-` + suffix + `"
  dataset_capabilities = {
    "terraform-provider-dataset-` + suffix + `" = { query = ["read"] }
  }
}

resource "axiom_group" "test" {
  name  = "terraform-provider-group-` + suffix + `"
  roles = [axiom_role.test.id]
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					testAccCheckAxiomResourcesExist(client, "axiom_role.test"),
					testAccCheckAxiomResourcesExist(client, "axiom_group.test"),
					resource.TestCheckTypeSetElemAttr("axiom_role.test", "dataset_capabilities.terraform-provider-dataset-"+suffix+".query.*", "read"),
					resource.TestCheckResourceAttrPair("axiom_group.test", "roles.0", "axiom_role.test", "id"),
					resource.TestCheckResourceAttr("axiom_group.test", "members.#", "0"),
					resource.TestCheckResourceAttr("axiom_group.test", "is_managed", "false"),
				),
			},
			{
				ResourceName:      "axiom_role.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				ResourceName:      "axiom_group.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: provider + `
resource "axiom_role" "test" {
  name        = "terraform-provider-role-` + suffix + `"
  description = "Updated by the acceptance tests"
  dataset_capabilities = {
    "terraform-provider-dataset-` + suffix + `" = { query = ["read"], ingest = ["create"] }
  }
  org_capabilities = {
    dashboards = ["read"]
  }
}

resource "axiom_group" "test" {
  name        = "terraform-provider-group-` + suffix + `"
  description = "Updated by the acceptance tests"
  roles       = [axiom_role.test.id]
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("axiom_role.test", "description", "Updated by the acceptance tests"),
					resource.TestCheckTypeSetElemAttr("axiom_role.test", "dataset_capabilities.terraform-provider-dataset-"+suffix+".ingest.*", "create"),
					resource.TestCheckTypeSetElemAttr("axiom_role.test", "org_capabilities.dashboards.*", "read"),
					resource.TestCheckResourceAttr("axiom_group.test", "description", "Updated by the acceptance tests"),
				),
			},
		},
	})
}
