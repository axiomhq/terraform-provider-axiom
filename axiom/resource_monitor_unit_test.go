package axiom

import (
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"regexp"
	"slices"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
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

// TestMonitorThresholdLevels runs axiom_monitor against a fake v2 monitors API
// that follows the threshold levels contract: levels come back sorted from
// least to most severe, threshold is the most severe level's value, a monitor
// without levels reads back as one unnamed level, an empty thresholds list
// removes the levels, and a request without thresholds on a monitor with more
// than one level is rejected.
func TestMonitorThresholdLevels(t *testing.T) {
	for _, env := range []string{"AXIOM_URL", "AXIOM_TOKEN", "AXIOM_ORG_ID", "AXIOM_EDGE_URL", "AXIOM_EDGE", "AXIOM_API_TOKEN", "AXIOM_BASE_URL"} {
		t.Setenv(env, "")
	}

	var mu sync.Mutex
	stored := map[string]map[string]any{}
	var requests []map[string]any
	severities := []string{"info", "warning", "error", "critical"}

	respond := func(w http.ResponseWriter, m map[string]any) {
		out := maps.Clone(m)
		if out["thresholds"] == nil {
			out["thresholds"] = []any{map[string]any{"value": out["threshold"]}}
		}
		writeJSON(w, http.StatusOK, out)
	}
	write := func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		requests = append(requests, maps.Clone(body))
		id := r.PathValue("id")
		if r.Method == http.MethodPost {
			id = fmt.Sprintf("monitor-%d", len(requests))
		} else if stored[id] == nil {
			writeError(w, http.StatusNotFound, "monitor not found")
			return
		}
		body["id"] = id
		if levels, ok := body["thresholds"].([]any); ok && len(levels) == 0 {
			// An empty list removes the levels, leaving threshold.
			delete(body, "thresholds")
		} else if ok {
			slices.SortFunc(levels, func(a, b any) int {
				return slices.Index(severities, a.(map[string]any)["severity"].(string)) -
					slices.Index(severities, b.(map[string]any)["severity"].(string))
			})
			body["threshold"] = levels[len(levels)-1].(map[string]any)["value"]
		} else if old, _ := stored[id]["thresholds"].([]any); len(old) > 1 {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("monitor has %d threshold levels, set thresholds to change them", len(old)))
			return
		}
		stored[id] = body
		respond(w, body)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v2/monitors", write)
	mux.HandleFunc("PUT /v2/monitors/{id}", write)
	mux.HandleFunc("GET /v2/monitors/{id}", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if m := stored[r.PathValue("id")]; m != nil {
			respond(w, m)
		} else {
			writeError(w, http.StatusNotFound, "monitor not found")
		}
	})
	mux.HandleFunc("DELETE /v2/monitors/{id}", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		delete(stored, r.PathValue("id"))
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /v2/labels/assignments", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Targets []labelTarget `json:"targets"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		writeJSON(w, http.StatusOK, []labelAssignment{{Target: body.Targets[0], LabelIDs: []string{}}})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	configWith := func(monitorType, operator, levels string) string {
		if operator != "" {
			operator = fmt.Sprintf("operator = %q", operator)
		}
		return fmt.Sprintf(`
provider "axiom" {
  api_token = "xaat-test"
  base_url  = %q
}
resource "axiom_monitor" "test" {
  name      = "cpu"
  type      = %q
  apl_query = "['metrics'] | summarize avg(cpu)"
  %s
  %s
}
data "axiom_monitor" "test" {
  id = axiom_monitor.test.id
}
`, srv.URL, monitorType, operator, levels)
	}
	config := func(levels string) string { return configWith("Threshold", "Above", levels) }
	// Written most severe first; the API returns them least severe first.
	twoLevels := func(critical string) string {
		return `thresholds = [
    { name = "Page on-call", severity = "critical", value = ` + critical + `, recovery = 90 },
    { name = "Heads up", severity = "warning", value = 80 },
  ]`
	}
	const res = "axiom_monitor.test"
	// lastRequest checks the threshold fields of the last monitor write.
	lastRequest := func(want string) resource.TestCheckFunc {
		return func(*terraform.State) error {
			mu.Lock()
			defer mu.Unlock()
			last := requests[len(requests)-1]
			fields := map[string]any{"threshold": last["threshold"]}
			if levels, sent := last["thresholds"]; sent {
				fields["thresholds"] = levels
			}
			if got, _ := json.Marshal(fields); string(got) != "{"+want+"}" {
				return fmt.Errorf("last request sent %s, want {%s}", got, want)
			}
			return nil
		}
	}

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"axiom": providerserver.NewProtocol6WithError(NewAxiomProvider()),
		},
		Steps: []resource.TestStep{
			{
				Config: config(``),
				Check:  resource.TestCheckResourceAttr(res, "threshold", "0"),
			},
			{
				Config: configWith("MatchEvent", "", ``),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(res, "type", "MatchEvent"),
					resource.TestCheckResourceAttr(res, "threshold", "0"),
					resource.TestCheckNoResourceAttr(res, "thresholds"),
				),
			},
			{
				Config:      configWith("MatchEvent", "", twoLevels("95")),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`thresholds are only supported on Threshold monitors, not MatchEvent`),
			},
			{
				Config:      configWith("Threshold", "", twoLevels("95")),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`thresholds need operator Above, AboveOrEqual, Below or BelowOrEqual`),
			},
			{
				Config: config(`thresholds = [
    { name = "Page on-call", severity = "critical", value = 95 },
    { name = "Wake someone", severity = "critical", value = 99 },
  ]`),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`Threshold level severity "critical" is used more than once`),
			},
			{
				Config: config(`thresholds = [
    { name = "Page on-call", severity = "critical", value = 95 },
    { name = "Page on-call", severity = "error", value = 90 },
  ]`),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`Threshold level name "Page on-call" is used more than once`),
			},
			{
				// An existing threshold-only config sends no thresholds and
				// plans no diff against the unnamed level the API returns.
				Config: config(`threshold = 95`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(res, "threshold", "95"),
					resource.TestCheckNoResourceAttr(res, "thresholds"),
					func(*terraform.State) error {
						mu.Lock()
						defer mu.Unlock()
						for _, r := range requests {
							if _, ok := r["thresholds"]; ok {
								return fmt.Errorf("threshold-only config sent thresholds: %v", r["thresholds"])
							}
						}
						return nil
					},
				),
			},
			{
				Config:      config(`threshold = 95` + "\n" + twoLevels("95")),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`Attribute "thresholds" cannot be specified when "threshold" is\s+specified`),
			},
			{
				Config: config(twoLevels("95")),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(res, "threshold", "95"),
					resource.TestCheckResourceAttr(res, "thresholds.#", "2"),
					resource.TestCheckTypeSetElemNestedAttrs(res, "thresholds.*", map[string]string{
						"name": "Page on-call", "severity": "critical", "value": "95", "recovery": "90",
					}),
					resource.TestCheckTypeSetElemNestedAttrs(res, "thresholds.*", map[string]string{
						"name": "Heads up", "severity": "warning", "value": "80",
					}),
					resource.TestCheckResourceAttr("data.axiom_monitor.test", "threshold", "95"),
					resource.TestCheckResourceAttr("data.axiom_monitor.test", "thresholds.#", "2"),
					resource.TestCheckTypeSetElemNestedAttrs("data.axiom_monitor.test", "thresholds.*", map[string]string{
						"name": "Page on-call", "severity": "critical", "value": "95", "recovery": "90",
					}),
				),
			},
			{ResourceName: res, ImportState: true, ImportStateVerify: true},
			{
				// threshold follows the most severe level as it moves.
				Config: config(twoLevels("97")),
				Check:  resource.TestCheckResourceAttr(res, "threshold", "97"),
			},
			{
				// A level value unknown until apply plans threshold as unknown.
				Config: config(`thresholds = [
    { name = "Page on-call", severity = "critical", value = terraform_data.level.output },
    { name = "Heads up", severity = "warning", value = 80 },
  ]
}
resource "terraform_data" "level" {
  input = 99`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(res, "threshold", "99"),
					resource.TestCheckResourceAttr(res, "thresholds.#", "2"),
				),
			},
			{
				// Removing thresholds removes the levels with an empty list,
				// sent with the configured threshold.
				Config: config(`threshold = 90`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(res, "threshold", "90"),
					resource.TestCheckNoResourceAttr(res, "thresholds"),
					resource.TestCheckNoResourceAttr("data.axiom_monitor.test", "thresholds"),
					lastRequest(`"threshold":90,"thresholds":[]`),
				),
			},
			{
				// With levels gone, a threshold-only update sends no thresholds.
				Config: config(`threshold = 85`),
				Check:  lastRequest(`"threshold":85`),
			},
			{Config: config(twoLevels("95"))},
			{
				// With neither attribute set, threshold becomes 0.
				Config: config(``),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(res, "threshold", "0"),
					resource.TestCheckNoResourceAttr(res, "thresholds"),
					lastRequest(`"threshold":0,"thresholds":[]`),
				),
			},
			{
				// One named level, as the Console can leave a monitor, is
				// removed the same way.
				PreConfig: func() {
					mu.Lock()
					defer mu.Unlock()
					for _, m := range stored {
						m["threshold"] = 95.0
						m["thresholds"] = []any{map[string]any{"name": "CPU", "severity": "critical", "value": 95.0}}
					}
				},
				Config: config(`threshold = 95`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckNoResourceAttr(res, "thresholds"),
					lastRequest(`"threshold":95,"thresholds":[]`),
				),
			},
		},
	})
}
