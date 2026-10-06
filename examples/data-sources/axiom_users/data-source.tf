data "axiom_users" "all" {}

output "user_ids_by_email" {
  value = { for u in data.axiom_users.all.users : u.email => u.id }
}
