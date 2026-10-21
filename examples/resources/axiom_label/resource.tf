resource "axiom_label" "production" {
  name        = "production"
  description = "Production resources"
  color       = "green"
}

resource "axiom_dataset" "logs" {
  name   = "production-logs"
  labels = [axiom_label.production.id]
}
