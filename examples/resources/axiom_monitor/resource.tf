resource "axiom_monitor" "threshold" {
  name             = "Error rate"
  type             = "Threshold"
  apl_query        = "['http-logs'] | where status >= 500 | summarize count() by bin_auto(_time)"
  interval_minutes = 5
  range_minutes    = 5
  operator         = "Above"
  threshold        = 100
}

# Named levels set threshold to the most severe level's value. A level without
# notifier_ids uses the monitor's. Each level's notifiers hear about the alert
# while it is at that level or a more severe one: here Slack hears about both
# levels, and PagerDuty only about the critical one.
resource "axiom_monitor" "threshold_levels" {
  name             = "Error rate levels"
  type             = "Threshold"
  apl_query        = "['http-logs'] | where status >= 500 | summarize count() by bin_auto(_time)"
  interval_minutes = 5
  range_minutes    = 5
  operator         = "Above"
  notifier_ids     = [axiom_notifier.slack.id]
  thresholds = [
    { name = "Page on-call", severity = "critical", value = 500, recovery = 400, notifier_ids = [axiom_notifier.pagerduty.id] },
    { name = "Heads up", severity = "warning", value = 100 },
  ]
}

# On an anomaly monitor, level values are tolerances in standard deviations,
# and tolerance is set to the most severe level's value.
resource "axiom_monitor" "anomaly_levels" {
  name             = "Request volume anomaly"
  type             = "AnomalyDetection"
  apl_query        = "['http-logs'] | summarize count() by bin_auto(_time)"
  interval_minutes = 5
  range_minutes    = 60
  compare_days     = 7
  operator         = "Above"
  thresholds = [
    { name = "Page on-call", severity = "critical", value = 4 },
    { name = "Heads up", severity = "warning", value = 2, recovery = 1.5 },
  ]
}
