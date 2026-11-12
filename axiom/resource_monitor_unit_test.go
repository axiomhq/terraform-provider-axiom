package axiom

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
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

func TestMonitorMetadata(t *testing.T) {
	t.Parallel()

	plan := func(metadata types.Map) MonitorResourceModel {
		return MonitorResourceModel{
			Type:     types.StringValue(axiom.MonitorTypeMatchEvent.String()),
			APLQuery: types.StringValue("['events']"),
			Metadata: metadata,
		}
	}
	tests := []struct {
		name     string
		metadata types.Map
		want     map[string]string
	}{
		// A nil map is not sent, so the API keeps the stored metadata.
		{"omitted keeps the stored metadata", types.MapNull(types.StringType), nil},
		{"unknown keeps the stored metadata", types.MapUnknown(types.StringType), nil},
		{"empty removes the metadata", types.MapValueMust(types.StringType, map[string]attr.Value{}), map[string]string{}},
		{"values are sent", types.MapValueMust(types.StringType, map[string]attr.Value{"team": types.StringValue("atlas")}), map[string]string{"team": "atlas"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			monitor, diags := extractMonitorResourceModel(t.Context(), plan(tt.metadata))
			assert.False(t, diags.HasError(), diags)
			assert.Equal(t, tt.want, monitor.Metadata)
			if tt.want != nil {
				assert.NotNil(t, monitor.Metadata)
			}
		})
	}

	t.Run("a monitor without metadata reads as an empty map", func(t *testing.T) {
		t.Parallel()

		state := flattenMonitor(&axiom.Monitor{Type: axiom.MonitorTypeMatchEvent}, nil)
		assert.Equal(t, types.MapValueMust(types.StringType, map[string]attr.Value{}), state.Metadata)
		state = flattenMonitor(&axiom.Monitor{Type: axiom.MonitorTypeMatchEvent, Metadata: map[string]string{"team": "atlas"}}, nil)
		assert.Equal(t, types.MapValueMust(types.StringType, map[string]attr.Value{"team": types.StringValue("atlas")}), state.Metadata)
	})
}
