data "axiom_users" "all" {}

locals {
  team_a_emails = ["alice@example.com", "bob@example.com"]
  team_a_ids    = [for u in data.axiom_users.all.users : u.id if contains(local.team_a_emails, u.email)]
}

# Members of the group receive the role in addition to their base role.
resource "axiom_group" "team_a" {
  name    = "team-a"
  roles   = [axiom_role.team_a_readers.id]
  members = local.team_a_ids
}

# Give the members no base role, so the group is their only access.
resource "axiom_user_role" "team_a" {
  for_each = toset(local.team_a_ids)
  user_id  = each.value
  role     = "none"
}
