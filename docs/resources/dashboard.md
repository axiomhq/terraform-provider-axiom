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

- `id` (String) Dashboard identifier (same value as `uid`).

## Import

Import by dashboard UID:

```terraform
terraform import axiom_dashboard.example <uid>
```
