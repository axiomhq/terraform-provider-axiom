# A token that only exists for the duration of the Terraform run.
ephemeral "axiom_token" "run" {
  name = "terraform-run"

  dataset_capabilities = {
    "application-logs" = {
      query = ["read"]
    }
  }
}

# A token that outlives the run so it can be handed to a secret store through
# a write-only argument. Its value is never written to the Terraform state.
# A new token is created by every plan and apply, so set expires_at to let
# the ones that are not stored expire.
ephemeral "axiom_token" "ingest" {
  name            = "application-ingest"
  expires_at      = "2027-01-01T00:00:00Z"
  delete_on_close = false

  dataset_capabilities = {
    "application-logs" = {
      ingest = ["create"]
    }
  }
}

# The value of a long-lived token managed by the axiom_token_stateless
# resource, which does not keep it in the Terraform state. No token is created
# here: the value is carried during the apply that creates or regenerates the
# managed token, and is an empty string in every other run.
resource "axiom_token_stateless" "collector" {
  name = "otel-collector"

  dataset_capabilities = {
    "otel-logs" = {
      ingest = ["create"]
    }
  }
}

ephemeral "axiom_token" "collector" {
  id = axiom_token_stateless.collector.id
}
