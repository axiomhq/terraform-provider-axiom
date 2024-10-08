---
# generated by https://github.com/hashicorp/terraform-plugin-docs
page_title: "axiom_monitor Data Source - terraform-provider-axiom"
subcategory: ""
description: |-
  
---

# axiom_monitor (Data Source)





<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `id` (String) Monitor identifier

### Read-Only

- `alert_on_no_data` (Boolean) If the monitor should trigger an alert if there is no data
- `apl_query` (String) The query used inside the monitor
- `description` (String) Monitor description
- `disabled_until` (String) The time the monitor will be disabled until
- `interval_minutes` (Number) How often the monitor should run
- `name` (String) Monitor name
- `notifier_ids` (List of String) A list of notifier id's to be used when this monitor triggers
- `notify_by_group` (Boolean) If the monitor should track non-time groups separately
- `operator` (String) Operator used in monitor trigger evaluation
- `range_minutes` (Number) Query time range from now
- `resolvable` (Boolean) Determines whether the events triggered by the monitor are individually resolvable. This has no effect on threshold monitors
- `threshold` (Number) The threshold where the monitor should trigger
