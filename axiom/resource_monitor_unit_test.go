package axiom

import (
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"

	"github.com/axiomhq/axiom-go/axiom"
)

func TestValidateMonitor_QueryFields(t *testing.T) {
	t.Parallel()

	base := MonitorResourceModel{
		Type: types.StringValue(axiom.MonitorTypeMatchEvent.String()),
	}

	t.Run("requires one query field", func(t *testing.T) {
		t.Parallel()

		diags := validateMonitor(base)
		assert.True(t, diags.HasError())
		assert.Contains(t, diags[0].Summary(), "Exactly one query field is required")
	})

	t.Run("rejects both query fields", func(t *testing.T) {
		t.Parallel()

		plan := base
		plan.APLQuery = types.StringValue("['events']")
		plan.MPLQuery = types.StringValue("`test-metrics`:`http_request_duration_seconds` | align to 1m using avg")

		diags := validateMonitor(plan)
		assert.True(t, diags.HasError())
		assert.Contains(t, diags[0].Summary(), "Exactly one query field is required")
	})

	t.Run("accepts apl query only", func(t *testing.T) {
		t.Parallel()

		plan := base
		plan.APLQuery = types.StringValue("['events']")

		diags := validateMonitor(plan)
		assert.False(t, diags.HasError())
	})

	t.Run("accepts mpl query only", func(t *testing.T) {
		t.Parallel()

		plan := base
		plan.MPLQuery = types.StringValue("`test-metrics`:`http_request_duration_seconds` | align to 1m using avg")

		diags := validateMonitor(plan)
		assert.False(t, diags.HasError())
	})
}

func TestFlattenMonitor_QueryPreference(t *testing.T) {
	t.Parallel()

	monitor := &axiom.Monitor{
		ID:        "monitor-id",
		Name:      "monitor-name",
		APLQuery:  "`test-metrics`:`http_request_duration_seconds` | align to 1m using avg",
		Type:      axiom.MonitorTypeThreshold,
		Operator:  axiom.Above,
		CreatedBy: "user-id",
	}

	t.Run("preserves mpl_query in state when originally configured", func(t *testing.T) {
		t.Parallel()

		currentState := MonitorResourceModel{
			MPLQuery: types.StringValue("`test-metrics`:`http_request_duration_seconds` | align to 1m using avg"),
		}

		state := flattenMonitor(monitor, &currentState)
		assert.True(t, state.APLQuery.IsNull())
		assert.Equal(t, "`test-metrics`:`http_request_duration_seconds` | align to 1m using avg", state.MPLQuery.ValueString())
	})

	t.Run("stores apl_query when no mpl_query preference exists", func(t *testing.T) {
		t.Parallel()

		state := flattenMonitor(monitor, nil)
		assert.Equal(t, "`test-metrics`:`http_request_duration_seconds` | align to 1m using avg", state.APLQuery.ValueString())
		assert.True(t, state.MPLQuery.IsNull())
	})

	t.Run("stores mpl_query when api only returns mpl_query", func(t *testing.T) {
		t.Parallel()

		mplOnlyMonitor := &axiom.Monitor{
			ID:        "monitor-id",
			Name:      "monitor-name",
			MPLQuery:  "`test-metrics`:`http_request_duration_seconds` | align to 1m using avg",
			Type:      axiom.MonitorTypeThreshold,
			Operator:  axiom.Above,
			CreatedBy: "user-id",
		}

		state := flattenMonitor(mplOnlyMonitor, nil)
		assert.True(t, state.APLQuery.IsNull())
		assert.Equal(t, "`test-metrics`:`http_request_duration_seconds` | align to 1m using avg", state.MPLQuery.ValueString())
	})
}

func TestValidateMonitor_NewValues(t *testing.T) {
	t.Parallel()

	valid := MonitorResourceModel{
		Type:         types.StringValue(axiom.MonitorTypeNewValues.String()),
		APLQuery:     types.StringValue("['events'] | summarize count() by host"),
		TrackedValue: types.StringValue("host"),
		LookbackDays: types.Int64Value(14),
		BackfillDays: types.Int64Value(7),
	}
	assert.False(t, validateMonitor(valid).HasError())

	for name, tc := range map[string]struct {
		mutate  func(*MonitorResourceModel)
		summary string
	}{
		"requires tracked_value": {func(m *MonitorResourceModel) { m.TrackedValue = types.StringNull() }, "Tracked value is required"},
		"requires lookback_days": {func(m *MonitorResourceModel) {
			m.LookbackDays, m.BackfillDays = types.Int64Value(0), types.Int64Value(0)
		}, "Lookback days is required"},
		"limits backfill_days": {func(m *MonitorResourceModel) { m.BackfillDays = types.Int64Value(15) }, "Invalid backfill days"},
		"rejects mpl_query": {func(m *MonitorResourceModel) {
			m.APLQuery = types.StringNull()
			m.MPLQuery = types.StringValue("`test-metrics`:`http_request_duration_seconds` | align to 1m using avg")
		}, "APL query is required"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			plan := valid
			tc.mutate(&plan)
			diags := validateMonitor(plan)
			assert.True(t, diags.HasError())
			assert.Equal(t, tc.summary, diags[0].Summary())
		})
	}
}

func TestFlattenMonitor_NewValues(t *testing.T) {
	t.Parallel()

	monitor := &axiom.Monitor{
		ID:           "monitor-id",
		Name:         "monitor-name",
		APLQuery:     "['events'] | summarize count() by host",
		Type:         axiom.MonitorTypeNewValues,
		Interval:     5 * time.Minute,
		Range:        5 * time.Minute,
		TrackedValue: "host",
		LookbackDays: 14,
		BackfillDays: 7,
	}

	t.Run("stores the new values fields", func(t *testing.T) {
		t.Parallel()

		state := flattenMonitor(monitor, nil)
		assert.Equal(t, "NewValues", state.Type.ValueString())
		assert.Equal(t, "host", state.TrackedValue.ValueString())
		assert.Equal(t, int64(14), state.LookbackDays.ValueInt64())
		assert.Equal(t, int64(7), state.BackfillDays.ValueInt64())
		assert.Equal(t, int64(5), state.RangeMinutes.ValueInt64())
	})

	t.Run("keeps the configured range, which the server ignores", func(t *testing.T) {
		t.Parallel()

		state := flattenMonitor(monitor, &MonitorResourceModel{RangeMinutes: types.Int64Value(1)})
		assert.Equal(t, int64(1), state.RangeMinutes.ValueInt64())
	})

	t.Run("leaves tracked_value null on other monitors", func(t *testing.T) {
		t.Parallel()

		state := flattenMonitor(&axiom.Monitor{Type: axiom.MonitorTypeThreshold, Range: 5 * time.Minute}, &MonitorResourceModel{RangeMinutes: types.Int64Value(1)})
		assert.True(t, state.TrackedValue.IsNull())
		assert.Equal(t, int64(5), state.RangeMinutes.ValueInt64(), "other monitors keep the server's range")
	})
}
