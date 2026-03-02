---
page_title: "axiom_dashboard Data Source - terraform-provider-axiom"
subcategory: ""
description: |-
  Reads an Axiom v2 dashboard by UID.
---

# axiom_dashboard (Data Source)

## Schema

### Required

- `uid` (String) Dashboard UID.

### Read-Only

- `created_at` (String) Creation timestamp returned by the API.
- `created_by` (String) Creator returned by the API.
- `dashboard` (String) Dashboard document as normalized JSON.
- `id` (String) Dashboard identifier (same value as `uid`).
- `internal_id` (String) Internal dashboard identifier returned by the API.
- `updated_at` (String) Last update timestamp returned by the API.
- `updated_by` (String) Last updater returned by the API.
- `version` (Number) Monotonic dashboard version.
