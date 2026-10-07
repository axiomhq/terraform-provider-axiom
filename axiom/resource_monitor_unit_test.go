package axiom

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

func TestMonitorSchema_TypeChangeRequiresReplace(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	var schemaResp resource.SchemaResponse
	(&MonitorResource{}).Schema(ctx, resource.SchemaRequest{}, &schemaResp)
	require.False(t, schemaResp.Diagnostics.HasError())

	typeAttr, ok := schemaResp.Schema.Attributes["type"].(schema.StringAttribute)
	require.True(t, ok, "type attribute must be a string attribute")

	// An existing resource: the plan modifier only considers replacement when
	// both the prior state and the plan are known objects.
	existingResource := func() tftypes.Value {
		values := make(map[string]tftypes.Value, len(schemaResp.Schema.Attributes))
		for name, attr := range schemaResp.Schema.Attributes {
			values[name] = tftypes.NewValue(attr.GetType().TerraformType(ctx), nil)
		}
		return tftypes.NewValue(schemaResp.Schema.Type().TerraformType(ctx), values)
	}

	planType := func(stateType, plannedType string) planmodifier.StringResponse {
		req := planmodifier.StringRequest{
			Path:        path.Root("type"),
			StateValue:  types.StringValue(stateType),
			PlanValue:   types.StringValue(plannedType),
			ConfigValue: types.StringValue(plannedType),
			State:       tfsdk.State{Schema: schemaResp.Schema, Raw: existingResource()},
			Plan:        tfsdk.Plan{Schema: schemaResp.Schema, Raw: existingResource()},
		}
		resp := planmodifier.StringResponse{PlanValue: req.PlanValue}
		for _, modifier := range typeAttr.PlanModifiers {
			modifier.PlanModifyString(ctx, req, &resp)
		}
		return resp
	}

	t.Run("changing the type replaces the monitor", func(t *testing.T) {
		t.Parallel()

		resp := planType(axiom.MonitorTypeThreshold.String(), axiom.MonitorTypeMatchEvent.String())
		require.False(t, resp.Diagnostics.HasError())
		assert.True(t, resp.RequiresReplace)
	})

	t.Run("keeping the type updates in place", func(t *testing.T) {
		t.Parallel()

		resp := planType(axiom.MonitorTypeThreshold.String(), axiom.MonitorTypeThreshold.String())
		require.False(t, resp.Diagnostics.HasError())
		assert.False(t, resp.RequiresReplace)
	})
}
