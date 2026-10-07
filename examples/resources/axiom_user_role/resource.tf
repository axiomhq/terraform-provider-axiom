data "axiom_users" "all" {}

resource "axiom_user_role" "alice" {
  user_id = one([for u in data.axiom_users.all.users : u.id if u.email == "alice@example.com"])
  role    = "none"
}
