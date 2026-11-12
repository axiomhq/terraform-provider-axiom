package axiom

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/axiomhq/axiom-go/axiom"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMonitorUpdateMetadata(t *testing.T) {
	for _, env := range []string{"AXIOM_URL", "AXIOM_TOKEN", "AXIOM_ORG_ID", "AXIOM_EDGE_URL", "AXIOM_EDGE", "AXIOM_API_TOKEN", "AXIOM_BASE_URL"} {
		t.Setenv(env, "")
	}
	ctx := context.Background()
	stored := &axiom.Monitor{ID: "monitor-id", Name: "errors", Type: axiom.MonitorTypeMatchEvent, APLQuery: "['logs']", Metadata: map[string]string{"team": "atlas"}}
	empty := types.MapValueMust(types.StringType, map[string]attr.Value{})
	for _, tc := range []struct {
		name   string
		config types.Map
		want   string
	}{
		// With -refresh=false the plan keeps the stale {} from state.
		{"left out of the config keeps the stored metadata", types.MapNull(types.StringType), ""},
		{"an empty map clears it", empty, "{}"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var sent map[string]json.RawMessage
			mux := http.NewServeMux()
			mux.HandleFunc("/v2/monitors/", func(w http.ResponseWriter, r *http.Request) {
				assert.NoError(t, json.NewDecoder(r.Body).Decode(&sent))
				writeJSON(w, 200, stored)
			})
			mux.HandleFunc("/v2/labels/assignments", func(w http.ResponseWriter, _ *http.Request) {
				writeJSON(w, 200, []labelAssignment{{Target: labelTarget{Type: "monitor", ID: "monitor-id"}, LabelIDs: []string{}}})
			})
			srv := httptest.NewServer(mux)
			defer srv.Close()
			client, err := axiom.NewClient(axiom.SetURL(srv.URL), axiom.SetToken("xapt-test"))
			require.NoError(t, err)
			r := NewMonitorResource()
			r.(resource.ResourceWithConfigure).Configure(ctx, resource.ConfigureRequest{ProviderData: client}, &resource.ConfigureResponse{})
			var schemaResp resource.SchemaResponse
			r.Schema(ctx, resource.SchemaRequest{}, &schemaResp)

			plan := tfsdk.Plan{Schema: schemaResp.Schema}
			model := flattenMonitor(&axiom.Monitor{ID: "monitor-id", Name: "errors", Type: axiom.MonitorTypeMatchEvent, APLQuery: "['logs']"}, nil)
			require.False(t, plan.Set(ctx, model).HasError())
			require.False(t, plan.SetAttribute(ctx, path.Root("metadata"), empty).HasError())
			config := tfsdk.Config{Schema: schemaResp.Schema, Raw: plan.Raw}
			configState := tfsdk.State(config)
			require.False(t, configState.SetAttribute(ctx, path.Root("metadata"), tc.config).HasError())
			config.Raw = configState.Raw

			resp := resource.UpdateResponse{State: tfsdk.State{Schema: schemaResp.Schema}}
			r.Update(ctx, resource.UpdateRequest{Plan: plan, Config: config}, &resp)
			require.False(t, resp.Diagnostics.HasError(), "%v", resp.Diagnostics)
			assert.Equal(t, tc.want, string(sent["metadata"]))
		})
	}
}
