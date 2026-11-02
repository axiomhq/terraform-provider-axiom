package axiom

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/axiomhq/axiom-go/axiom"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResourceLabelsLifecycle(t *testing.T) {
	for _, env := range []string{"AXIOM_URL", "AXIOM_TOKEN", "AXIOM_ORG_ID", "AXIOM_EDGE_URL", "AXIOM_EDGE", "AXIOM_API_TOKEN", "AXIOM_BASE_URL"} {
		t.Setenv(env, "")
	}
	ctx := context.Background()
	dataset := &axiom.Dataset{ID: "internal-dataset-id", Name: "logs", Kind: "axiom:events:v1", EdgeDeployment: "cloud"}
	monitor := &axiom.Monitor{ID: "monitor-id", Name: "errors", Type: axiom.MonitorTypeMatchEvent, APLQuery: "['logs']"}
	dashboard := dashboardResourcePayload{ID: "internal-dashboard-id", UID: "dashboard-uid", Dashboard: json.RawMessage(`{"name":"Overview"}`)}
	dashboardModel, err := flattenDashboardResource(dashboard, types.BoolValue(true), types.StringNull())
	require.NoError(t, err)
	for _, tc := range []struct {
		kind, id, route string
		newResource     func() resource.Resource
		model, response any
	}{
		{"dataset", "logs", "/v2/datasets", NewDatasetResource, flattenDataset(dataset, ""), dataset},
		{"monitor", "monitor-id", "/v2/monitors", NewMonitorResource, flattenMonitor(monitor, nil), monitor},
		{"dashboard", "dashboard-uid", "/v2/dashboards", NewDashboardResource, dashboardModel, dashboard},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			assigned := []string{}
			failAssignment := false
			mux := http.NewServeMux()
			mux.HandleFunc("/v2/labels/assignments", func(w http.ResponseWriter, r *http.Request) {
				if failAssignment {
					http.Error(w, "forbidden", http.StatusForbidden)
					return
				}
				target := labelTarget{Type: tc.kind, ID: tc.id}
				if r.Method == http.MethodPut {
					var body labelAssignment
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Error(err)
						w.WriteHeader(400)
						return
					}
					assert.Equal(t, target, body.Target)
					assert.NotNil(t, body.LabelIDs, "clearing requires [], not null")
					assigned = body.LabelIDs
					writeJSON(w, 200, body)
				} else {
					var body struct {
						Targets []labelTarget `json:"targets"`
					}
					assert.NoError(t, json.NewDecoder(r.Body).Decode(&body))
					assert.Equal(t, []labelTarget{target}, body.Targets)
					writeJSON(w, 200, []labelAssignment{{Target: target, LabelIDs: assigned}})
				}
			})
			serveResource := func(w http.ResponseWriter, r *http.Request) {
				if tc.kind == "dashboard" && r.Method != http.MethodGet {
					writeJSON(w, 200, map[string]any{"dashboard": tc.response})
				} else {
					writeJSON(w, 200, tc.response)
				}
			}
			mux.HandleFunc(tc.route, serveResource)
			mux.HandleFunc(tc.route+"/", serveResource)
			srv := httptest.NewServer(mux)
			defer srv.Close()
			client, err := axiom.NewClient(axiom.SetURL(srv.URL), axiom.SetToken("xapt-test"))
			require.NoError(t, err)
			r := tc.newResource()
			var configured resource.ConfigureResponse
			r.(resource.ResourceWithConfigure).Configure(ctx, resource.ConfigureRequest{ProviderData: client}, &configured)
			var schemaResp resource.SchemaResponse
			r.Schema(ctx, resource.SchemaRequest{}, &schemaResp)
			state := tfsdk.State{Schema: schemaResp.Schema}
			require.False(t, state.Set(ctx, tc.model).HasError())
			setLabels := func(ids ...string) types.Set {
				if ids == nil {
					ids = []string{}
				}
				value, diags := types.SetValueFrom(ctx, types.StringType, ids)
				require.False(t, diags.HasError())
				return value
			}
			checkState := func(expected types.Set) {
				var actual types.Set
				require.False(t, state.GetAttribute(ctx, path.Root("labels"), &actual).HasError())
				require.True(t, expected.Equal(actual), "expected %s, got %s", expected, actual)
			}
			initial := setLabels("lbl_b", "lbl_a")
			require.False(t, state.SetAttribute(ctx, path.Root("labels"), initial).HasError())
			configState := state
			require.False(t, configState.SetAttribute(ctx, path.Root("labels"), types.SetUnknown(types.StringType)).HasError())
			create := resource.CreateResponse{State: state}
			r.Create(ctx, resource.CreateRequest{Plan: tfsdk.Plan(state), Config: tfsdk.Config{Schema: state.Schema, Raw: configState.Raw}}, &create)
			require.False(t, create.Diagnostics.HasError(), "%v", create.Diagnostics)
			state = create.State
			checkState(initial)
			require.ElementsMatch(t, []string{"lbl_a", "lbl_b"}, assigned)

			assigned = []string{"lbl_external"}
			read := resource.ReadResponse{State: state}
			r.Read(ctx, resource.ReadRequest{State: state}, &read)
			require.False(t, read.Diagnostics.HasError(), "%v", read.Diagnostics)
			state = read.State
			checkState(setLabels("lbl_external"))

			dataSources := map[string]datasource.DataSource{
				"dataset": NewDatasetDataSource(), "monitor": NewMonitorDataSource(), "dashboard": NewDashboardDataSource(),
			}
			d := dataSources[tc.kind]
			var dsConfigured datasource.ConfigureResponse
			d.(datasource.DataSourceWithConfigure).Configure(ctx, datasource.ConfigureRequest{ProviderData: client}, &dsConfigured)
			var dsSchema datasource.SchemaResponse
			d.Schema(ctx, datasource.SchemaRequest{}, &dsSchema)
			dsState := tfsdk.State{Schema: dsSchema.Schema}
			if tc.kind == "dashboard" {
				require.False(t, dsState.Set(ctx, DashboardDataSourceModel{UID: types.StringValue(tc.id), Labels: types.SetNull(types.StringType)}).HasError())
			} else {
				dsState.Raw = state.Raw
			}
			dsRead := datasource.ReadResponse{State: dsState}
			d.Read(ctx, datasource.ReadRequest{Config: tfsdk.Config{Schema: dsSchema.Schema, Raw: dsState.Raw}}, &dsRead)
			require.False(t, dsRead.Diagnostics.HasError(), "%v", dsRead.Diagnostics)
			var dsLabels types.Set
			require.False(t, dsRead.State.GetAttribute(ctx, path.Root("labels"), &dsLabels).HasError())
			require.True(t, setLabels("lbl_external").Equal(dsLabels))

			for _, desired := range []types.Set{setLabels("lbl_c"), types.SetNull(types.StringType), setLabels()} {
				plan := state
				require.False(t, plan.SetAttribute(ctx, path.Root("labels"), desired).HasError())
				config := tfsdk.Config(plan)
				if desired.IsNull() {
					// Terraform can carry prior computed state into the plan even
					// when labels are omitted from configuration.
					require.False(t, plan.SetAttribute(ctx, path.Root("labels"), setLabels("lbl_stale")).HasError())
				}
				update := resource.UpdateResponse{State: state}
				r.Update(ctx, resource.UpdateRequest{State: state, Plan: tfsdk.Plan(plan), Config: config}, &update)
				require.False(t, update.Diagnostics.HasError(), "%v", update.Diagnostics)
				state = update.State
				if desired.IsNull() {
					checkState(setLabels("lbl_c"))
				} else {
					checkState(desired)
				}
			}
			require.Empty(t, assigned)

			failAssignment = true
			create = resource.CreateResponse{State: state}
			r.Create(ctx, resource.CreateRequest{Plan: tfsdk.Plan(state), Config: tfsdk.Config(state)}, &create)
			require.True(t, create.Diagnostics.HasError())
			var id types.String
			require.False(t, create.State.GetAttribute(ctx, path.Root("id"), &id).HasError())
			require.NotEmpty(t, id.ValueString(), "assignment failures must retain the created resource")
		})
	}
}
