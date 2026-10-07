# A role that can only query two datasets.
resource "axiom_role" "team_a_readers" {
  name        = "team-a-readers"
  description = "Query team A datasets"

  dataset_capabilities = {
    "team-a-logs"   = { query = ["read"] }
    "team-a-traces" = { query = ["read"] }
  }

  org_capabilities = {
    dashboards = ["read"]
  }
}
