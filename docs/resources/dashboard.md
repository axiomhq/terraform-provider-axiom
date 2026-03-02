---
page_title: "axiom_dashboard Resource - terraform-provider-axiom"
subcategory: ""
description: |-
  Manages an Axiom v2 dashboard using UID-based lifecycle and optimistic concurrency.
---

# axiom_dashboard (Resource)

## Schema

### Required

- `dashboard` (String) The dashboard document as a JSON string (for example from `jsonencode(...)`).

### Optional

- `overwrite` (Boolean) When `true`, force update and ignore `version` conflicts.
- `uid` (String) Stable dashboard identifier. If omitted, Axiom generates one.

### Read-Only

- `created_at` (String) Creation timestamp returned by the API.
- `created_by` (String) Creator returned by the API.
- `id` (String) Dashboard identifier (same value as `uid`).
- `updated_at` (String) Last update timestamp returned by the API.
- `updated_by` (String) Last updater returned by the API.
- `version` (Number) Monotonic dashboard version used for optimistic updates.

## Import

Import by dashboard UID:

```terraform
terraform import axiom_dashboard.example <uid>
```
