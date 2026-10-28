resource "axiom_dataset" "logs" {
  name = "application-logs"
}

resource "axiom_view" "errors" {
  name        = "application-errors"
  description = "Application error events"
  apl_query   = <<-APL
    ['${axiom_dataset.logs.name}']
    | where level == "error"
  APL
}
