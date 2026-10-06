package axiom

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/axiomhq/axiom-go/axiom"
)

func testRBACUser(id, email, role string) *axiom.User {
	u := &axiom.User{ID: id, Name: id, Email: email}
	u.Role.ID, u.Role.Name = role, role
	return u
}

func testRBACProviderConfig(url string) string {
	return `
provider "axiom" {
  api_token = "xaat-test"
  base_url  = "` + url + `"
}
`
}

var testRBACProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"axiom": providerserver.NewProtocol6WithError(NewAxiomProvider()),
}

// TestRBACResources_lifecycle sets up dataset-scoped access the way the
// support docs describe it: a custom role, a group carrying the role, and
// group members whose base role is "none".
func TestRBACResources_lifecycle(t *testing.T) {
	srv := newFakeRBACAPI(t,
		testRBACUser("user-alice", "alice@example.com", "user"),
		testRBACUser("user-bob", "bob@example.com", "user"),
		testRBACUser("user-carol", "carol@example.com", "admin"),
	)
	provider := testRBACProviderConfig(srv.URL)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testRBACProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: provider + `
data "axiom_users" "all" {}

locals {
  team_a_emails = ["alice@example.com", "bob@example.com"]
  team_a_ids    = [for u in data.axiom_users.all.users : u.id if contains(local.team_a_emails, u.email)]
}

resource "axiom_role" "team_a" {
  name = "team-a-readers"
  dataset_capabilities = {
    "team-a-logs" = { query = ["read"] }
  }
}

resource "axiom_group" "team_a" {
  name    = "team-a"
  roles   = [axiom_role.team_a.id]
  members = local.team_a_ids
}

resource "axiom_user_role" "alice" {
  user_id = one([for u in data.axiom_users.all.users : u.id if u.email == "alice@example.com"])
  role    = "none"
}

resource "axiom_user_role" "bob" {
  user_id = one([for u in data.axiom_users.all.users : u.id if u.email == "bob@example.com"])
  role    = "none"
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.axiom_users.all", "users.#", "3"),
					resource.TestCheckResourceAttr("axiom_role.team_a", "name", "team-a-readers"),
					resource.TestCheckNoResourceAttr("axiom_role.team_a", "description"),
					resource.TestCheckResourceAttr("axiom_role.team_a", "dataset_capabilities.team-a-logs.query.#", "1"),
					resource.TestCheckTypeSetElemAttr("axiom_role.team_a", "dataset_capabilities.team-a-logs.query.*", "read"),
					resource.TestCheckResourceAttr("axiom_role.team_a", "dataset_capabilities.team-a-logs.ingest.#", "0"),
					resource.TestCheckResourceAttr("axiom_role.team_a", "view_capabilities.%", "0"),
					resource.TestCheckResourceAttr("axiom_role.team_a", "org_capabilities.dashboards.#", "0"),
					resource.TestCheckResourceAttr("axiom_role.team_a", "members.#", "0"),
					resource.TestCheckResourceAttrPair("axiom_group.team_a", "roles.0", "axiom_role.team_a", "id"),
					resource.TestCheckResourceAttr("axiom_group.team_a", "members.#", "2"),
					resource.TestCheckTypeSetElemAttr("axiom_group.team_a", "members.*", "user-alice"),
					resource.TestCheckTypeSetElemAttr("axiom_group.team_a", "members.*", "user-bob"),
					resource.TestCheckResourceAttr("axiom_group.team_a", "is_managed", "false"),
					resource.TestCheckResourceAttr("axiom_user_role.alice", "user_id", "user-alice"),
					resource.TestCheckResourceAttr("axiom_user_role.alice", "role", "none"),
					resource.TestCheckResourceAttr("axiom_user_role.bob", "user_id", "user-bob"),
					resource.TestCheckResourceAttr("axiom_user_role.bob", "role", "none"),
				),
			},
			{
				ResourceName:      "axiom_role.team_a",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				ResourceName:      "axiom_group.team_a",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				ResourceName:      "axiom_user_role.alice",
				ImportState:       true,
				ImportStateId:     "user-alice",
				ImportStateVerify: true,
			},
			{
				Config: provider + `
resource "axiom_role" "team_a" {
  name        = "team-a-readers"
  description = "Query and ingest team A data"
  dataset_capabilities = {
    "team-a-logs"   = { query = ["read"], ingest = ["create"] }
    "team-a-traces" = { query = ["read"] }
  }
  view_capabilities = {
    "team-a-errors" = { query = ["read"] }
  }
  org_capabilities = {
    dashboards = ["read", "create"]
  }
}

resource "axiom_group" "team_a" {
  name        = "team-a"
  description = "Team A"
  roles       = [axiom_role.team_a.id]
  members     = ["user-alice"]
}

resource "axiom_user_role" "alice" {
  user_id = "user-alice"
  role    = "none"
}

resource "axiom_user_role" "bob" {
  user_id = "user-bob"
  role    = "none"
}

resource "axiom_user_role" "carol" {
  user_id = "user-carol"
  role    = axiom_role.team_a.id
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("axiom_role.team_a", "description", "Query and ingest team A data"),
					resource.TestCheckResourceAttr("axiom_role.team_a", "dataset_capabilities.%", "2"),
					resource.TestCheckTypeSetElemAttr("axiom_role.team_a", "dataset_capabilities.team-a-logs.ingest.*", "create"),
					resource.TestCheckTypeSetElemAttr("axiom_role.team_a", "view_capabilities.team-a-errors.query.*", "read"),
					resource.TestCheckResourceAttr("axiom_role.team_a", "org_capabilities.dashboards.#", "2"),
					resource.TestCheckResourceAttr("axiom_group.team_a", "description", "Team A"),
					resource.TestCheckResourceAttr("axiom_group.team_a", "members.#", "1"),
					resource.TestCheckResourceAttrPair("axiom_user_role.carol", "role", "axiom_role.team_a", "id"),
				),
			},
			{
				// Refresh picks up the members derived from base roles.
				RefreshState: true,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("axiom_role.team_a", "members.#", "1"),
					resource.TestCheckTypeSetElemAttr("axiom_role.team_a", "members.*", "user-carol"),
				),
			},
			{
				// Omitting members keeps the current members instead of
				// removing them.
				Config: provider + `
resource "axiom_role" "team_a" {
  name = "team-a-readers"
  dataset_capabilities = {
    "team-a-logs" = { query = ["read"] }
  }
}

resource "axiom_group" "team_a" {
  name  = "team-a"
  roles = [axiom_role.team_a.id]
}
`,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr("axiom_role.team_a", "description"),
					resource.TestCheckResourceAttr("axiom_role.team_a", "dataset_capabilities.%", "1"),
					resource.TestCheckResourceAttr("axiom_role.team_a", "view_capabilities.%", "0"),
					resource.TestCheckResourceAttr("axiom_role.team_a", "org_capabilities.dashboards.#", "0"),
					resource.TestCheckResourceAttr("axiom_group.team_a", "members.#", "1"),
					resource.TestCheckTypeSetElemAttr("axiom_group.team_a", "members.*", "user-alice"),
				),
			},
		},
	})
}

func TestRBACResources_validation(t *testing.T) {
	srv := newFakeRBACAPI(t)
	provider := testRBACProviderConfig(srv.URL)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testRBACProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: provider + `
resource "axiom_role" "test" {
  name                 = "test"
  dataset_capabilities = { "logs" = {} }
}
`,
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`The dataset "logs" must have at least one capability`),
			},
			{
				Config: provider + `
resource "axiom_role" "test" {
  name                 = "test"
  dataset_capabilities = { "logs" = { query = [] } }
}
`,
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`The dataset "logs" must have at least one capability`),
			},
			{
				Config: provider + `
resource "axiom_role" "test" {
  name              = "test"
  view_capabilities = { "*" = { query = ["read"] } }
}
`,
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`Invalid Attribute Value Match`),
			},
			{
				Config: provider + `
resource "axiom_role" "test" {
  name                 = "test"
  dataset_capabilities = { "logs" = { query = ["create"] } }
}
`,
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`value must be\s+one of`),
			},
		},
	})
}

func TestUserResource_update(t *testing.T) {
	srv := newFakeRBACAPI(t)
	provider := testRBACProviderConfig(srv.URL)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testRBACProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: provider + `
resource "axiom_user" "test" {
  name  = "Dana"
  email = "dana@example.com"
  role  = "user"
}
`,
				Check: resource.TestCheckResourceAttr("axiom_user.test", "role", "user"),
			},
			{
				Config: provider + `
resource "axiom_role" "test" {
  name                 = "readers"
  dataset_capabilities = { "logs" = { query = ["read"] } }
}

resource "axiom_user" "test" {
  name  = "Dana"
  email = "dana@example.com"
  role  = axiom_role.test.id
}
`,
				Check: resource.TestCheckResourceAttrPair("axiom_user.test", "role", "axiom_role.test", "id"),
			},
			{
				Config: provider + `
resource "axiom_user" "test" {
  name  = "Dana Scully"
  email = "dana@example.com"
  role  = "none"
}
`,
				ExpectError: regexp.MustCompile(`User name can't be changed`),
			},
		},
	})
}
