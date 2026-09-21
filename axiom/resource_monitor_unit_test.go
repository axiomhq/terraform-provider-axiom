package axiom

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
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

func TestUpgradeMonitorResourceStateV0_PreservesImportedID(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	r := &MonitorResource{}

	upgraders := r.UpgradeState(ctx)
	upgrader, ok := upgraders[0]
	require.True(t, ok, "expected a version 0 state upgrader")
	require.NotNil(t, upgrader.PriorSchema)
	require.NotNil(t, upgrader.StateUpgrader)

	prior := tfsdk.State{
		Schema: *upgrader.PriorSchema,
		Raw:    tftypes.NewValue(upgrader.PriorSchema.Type().TerraformType(ctx), nil),
	}
	diags := prior.SetAttribute(ctx, path.Root("id"), "mon_imported")
	require.False(t, diags.HasError(), diags.Errors())

	var schemaResp resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &schemaResp)

	req := resource.UpgradeStateRequest{State: &prior}
	resp := resource.UpgradeStateResponse{
		State: tfsdk.State{Schema: schemaResp.Schema},
	}
	upgrader.StateUpgrader(ctx, req, &resp)
	require.False(t, resp.Diagnostics.HasError(), resp.Diagnostics.Errors())

	var upgraded MonitorResourceModel
	require.False(t, resp.State.Get(ctx, &upgraded).HasError())
	assert.Equal(t, "mon_imported", upgraded.ID.ValueString())
}

func TestMonitorResource_ImportDoesNotFailOnVersion0Upgrade(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	server := providerserver.NewProtocol6(NewAxiomProvider())()

	const importedID = "mon_imported"

	importResp, err := server.ImportResourceState(ctx, &tfprotov6.ImportResourceStateRequest{
		TypeName: "axiom_monitor",
		ID:       importedID,
	})
	require.NoError(t, err)
	require.False(t, protoDiagnosticsHasError(importResp.Diagnostics), protoDiagnostics(importResp.Diagnostics))
	require.Len(t, importResp.ImportedResources, 1)
	require.NotNil(t, importResp.ImportedResources[0].State)

	var schemaResp resource.SchemaResponse
	(&MonitorResource{}).Schema(ctx, resource.SchemaRequest{}, &schemaResp)
	schemaType := schemaResp.Schema.Type().TerraformType(ctx)

	importedVal, err := importResp.ImportedResources[0].State.Unmarshal(schemaType)
	require.NoError(t, err)

	// Imported state is tagged as schema version 0 by Terraform/Pulumi and
	// must upgrade to the current schema before Read can hydrate the monitor.
	for name, rawJSON := range map[string][]byte{
		"id-only":      []byte(`{"id":"` + importedID + `"}`),
		"imported-obj": mustTftypesObjectJSON(t, importedVal),
	} {
		t.Run(name, func(t *testing.T) {
			upgradeResp, err := server.UpgradeResourceState(ctx, &tfprotov6.UpgradeResourceStateRequest{
				TypeName: "axiom_monitor",
				Version:  0,
				RawState: &tfprotov6.RawState{JSON: rawJSON},
			})
			require.NoError(t, err)
			require.False(t, protoDiagnosticsHasError(upgradeResp.Diagnostics), protoDiagnostics(upgradeResp.Diagnostics))
			require.NotNil(t, upgradeResp.UpgradedState)

			upgradedVal, err := upgradeResp.UpgradedState.Unmarshal(schemaType)
			require.NoError(t, err)

			var upgraded MonitorResourceModel
			require.False(t, tfsdk.State{Schema: schemaResp.Schema, Raw: upgradedVal}.Get(ctx, &upgraded).HasError())
			assert.Equal(t, importedID, upgraded.ID.ValueString())
		})
	}
}

func mustTftypesObjectJSON(t *testing.T, val tftypes.Value) []byte {
	t.Helper()

	var obj map[string]tftypes.Value
	require.NoError(t, val.As(&obj))

	out := make(map[string]any, len(obj))
	for key, attr := range obj {
		if attr.IsNull() {
			out[key] = nil
			continue
		}

		var str string
		if err := attr.As(&str); err == nil {
			out[key] = str
			continue
		}

		out[key] = nil
	}

	raw, err := json.Marshal(out)
	require.NoError(t, err)
	return raw
}

func protoDiagnosticsHasError(diags []*tfprotov6.Diagnostic) bool {
	for _, d := range diags {
		if d != nil && d.Severity == tfprotov6.DiagnosticSeverityError {
			return true
		}
	}
	return false
}

func protoDiagnostics(diags []*tfprotov6.Diagnostic) string {
	var out string
	for _, d := range diags {
		if d == nil {
			continue
		}
		out += d.Summary + ": " + d.Detail + "\n"
	}
	return out
}
