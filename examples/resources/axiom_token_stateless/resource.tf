# The token value is not kept in the Terraform state.
resource "axiom_token_stateless" "collector" {
  name        = "otel-collector"
  description = "OpenTelemetry collector"

  dataset_capabilities = {
    "otel-logs" = {
      ingest = ["create"]
    }
  }

  # The previous token keeps working this long after the token is regenerated.
  rotation_grace_period = "48h"
}

# Carries the value during the apply that creates or regenerates the token.
# Pass ephemeral.axiom_token.collector.token to a write-only argument of the
# resource that stores the token, and change the version of that argument in
# the same apply that regenerates the token.
ephemeral "axiom_token" "collector" {
  id = axiom_token_stateless.collector.id
}
