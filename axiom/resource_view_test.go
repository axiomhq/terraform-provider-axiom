package axiom

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"sync"
	"testing"

	"github.com/axiomhq/axiom-go/axiom"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestViewResourceLifecycle(t *testing.T) {
	for _, env := range []string{"AXIOM_URL", "AXIOM_TOKEN", "AXIOM_ORG_ID", "AXIOM_EDGE_URL", "AXIOM_EDGE", "AXIOM_API_TOKEN", "AXIOM_BASE_URL"} {
		t.Setenv(env, "")
	}

	const initialQuery = "['logs'] | where level == 'error'"
	const updatedQuery = "union ['logs'], ['archive'] | where status >= 500"
	var mu sync.Mutex
	stored := map[string]map[string]any{}
	mux := http.NewServeMux()

	// Match the API contract: name is the ID, datasets are derived from APL,
	// and updates cannot rename a view. Responses have no separate ID field.
	writeView := func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		var body struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			APLQuery    string `json:"aplQuery"`
		}
		decoder := json.NewDecoder(r.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&body); err != nil {
			t.Error(err)
			writeError(w, http.StatusBadRequest, "invalid view")
			return
		}
		name := body.Name
		if r.Method == http.MethodPut {
			name = r.PathValue("id")
			if stored[name] == nil {
				writeError(w, http.StatusNotFound, "view not found")
				return
			}
		} else if stored[name] != nil {
			writeError(w, http.StatusBadRequest, "view already exists")
			return
		}
		var datasets []string
		switch body.APLQuery {
		case initialQuery:
			datasets = []string{"logs"}
		case updatedQuery:
			datasets = []string{"archive", "logs"}
		default:
			writeError(w, http.StatusBadRequest, "invalid APL query")
			return
		}
		view := map[string]any{"name": name, "aplQuery": body.APLQuery, "datasets": datasets}
		if body.Description != "" {
			view["description"] = body.Description
		}
		stored[name] = view
		writeJSON(w, http.StatusOK, view)
	}
	mux.HandleFunc("POST /v2/views", writeView)
	mux.HandleFunc("PUT /v2/views/{id}", writeView)
	mux.HandleFunc("GET /v2/views/{id}", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if view := stored[r.PathValue("id")]; view != nil {
			writeJSON(w, http.StatusOK, view)
		} else {
			writeError(w, http.StatusNotFound, "view not found")
		}
	})
	mux.HandleFunc("DELETE /v2/views/{id}", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if stored[r.PathValue("id")] == nil {
			writeError(w, http.StatusNotFound, "view not found")
			return
		}
		delete(stored, r.PathValue("id"))
		w.WriteHeader(http.StatusNoContent)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	config := func(name, query, extra string) string {
		return fmt.Sprintf(`
provider "axiom" {
  api_token = "xaat-test-view"
  base_url  = %q
}
resource "axiom_view" "test" {
  name      = %q
  apl_query = %q
  %s
}
`, srv.URL, name, query, extra)
	}

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"axiom": providerserver.NewProtocol6WithError(NewAxiomProvider()),
		},
		Steps: []resource.TestStep{
			{
				Config:      config("", initialQuery, ""),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`Attribute name string length must be at least 1`),
			},
			{
				Config:      config("production-errors", "", ""),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`Attribute apl_query string length must be at least 1`),
			},
			{
				Config:      config("production-errors", initialQuery, `datasets = ["logs"]`),
				PlanOnly:    true,
				ExpectError: regexp.MustCompile(`Invalid Configuration for Read-Only Attribute`),
			},
			{
				Config:      config("production-errors", "invalid APL", ""),
				ExpectError: regexp.MustCompile(`invalid APL query`),
			},
			{
				Config: config("production-errors", initialQuery, ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("axiom_view.test", "id", "production-errors"),
					resource.TestCheckResourceAttr("axiom_view.test", "name", "production-errors"),
					resource.TestCheckResourceAttr("axiom_view.test", "apl_query", initialQuery),
					resource.TestCheckResourceAttr("axiom_view.test", "description", ""),
					resource.TestCheckResourceAttr("axiom_view.test", "datasets.#", "1"),
					resource.TestCheckTypeSetElemAttr("axiom_view.test", "datasets.*", "logs"),
				),
			},
			{ResourceName: "axiom_view.test", ImportState: true, ImportStateVerify: true},
			{
				Config: config("production-errors", updatedQuery, `description = "Server errors across current and archived logs"`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("axiom_view.test", "id", "production-errors"),
					resource.TestCheckResourceAttr("axiom_view.test", "apl_query", updatedQuery),
					resource.TestCheckResourceAttr("axiom_view.test", "description", "Server errors across current and archived logs"),
					resource.TestCheckResourceAttr("axiom_view.test", "datasets.#", "2"),
					resource.TestCheckTypeSetElemAttr("axiom_view.test", "datasets.*", "logs"),
					resource.TestCheckTypeSetElemAttr("axiom_view.test", "datasets.*", "archive"),
				),
			},
			{ResourceName: "axiom_view.test", ImportState: true, ImportStateVerify: true},
			{
				Config: config("production-errors", updatedQuery, ""),
				Check:  resource.TestCheckResourceAttr("axiom_view.test", "description", ""),
			},
			{
				PreConfig: func() {
					mu.Lock()
					defer mu.Unlock()
					stored["production-errors"]["description"] = "Changed outside Terraform"
					stored["production-errors"]["aplQuery"] = initialQuery
					stored["production-errors"]["datasets"] = []string{"logs"}
				},
				Config: config("production-errors", updatedQuery, ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("axiom_view.test", "description", ""),
					resource.TestCheckResourceAttr("axiom_view.test", "apl_query", updatedQuery),
					resource.TestCheckResourceAttr("axiom_view.test", "datasets.#", "2"),
				),
			},
			{
				PreConfig: func() {
					mu.Lock()
					defer mu.Unlock()
					delete(stored, "production-errors")
				},
				Config: config("production-errors", updatedQuery, ""),
				Check:  resource.TestCheckResourceAttr("axiom_view.test", "id", "production-errors"),
			},
			{
				Config: config("renamed-errors", initialQuery, ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("axiom_view.test", "id", "renamed-errors"),
					resource.TestCheckResourceAttr("axiom_view.test", "name", "renamed-errors"),
					resource.TestCheckResourceAttr("axiom_view.test", "datasets.#", "1"),
				),
			},
		},
	})
	mu.Lock()
	defer mu.Unlock()
	assert.Empty(t, stored, "destroy and name replacement should not leave views behind")
}

func TestViewResourceAPIErrors(t *testing.T) {
	for _, env := range []string{"AXIOM_URL", "AXIOM_TOKEN", "AXIOM_ORG_ID", "AXIOM_EDGE_URL", "AXIOM_EDGE"} {
		t.Setenv(env, "")
	}
	for _, tc := range []struct {
		method string
		status int
	}{
		{http.MethodGet, http.StatusNotFound},
		{http.MethodGet, http.StatusForbidden},
		{http.MethodPut, http.StatusForbidden},
		{http.MethodDelete, http.StatusNotFound},
		{http.MethodDelete, http.StatusForbidden},
	} {
		t.Run(fmt.Sprintf("%s/%d", tc.method, tc.status), func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, tc.method, r.Method)
				assert.Equal(t, "/v2/views/errors%2Fteam%3Fprod", r.URL.EscapedPath())
				assert.Empty(t, r.URL.RawQuery)
				writeError(w, tc.status, "view API error")
			}))
			defer srv.Close()
			client, err := axiom.NewClient(axiom.SetURL(srv.URL), axiom.SetAPITokenConfig("xaat-test-view"))
			require.NoError(t, err)
			r := NewViewResource()
			var configure frameworkresource.ConfigureResponse
			r.(frameworkresource.ResourceWithConfigure).Configure(t.Context(), frameworkresource.ConfigureRequest{ProviderData: client}, &configure)
			require.False(t, configure.Diagnostics.HasError())
			var schema frameworkresource.SchemaResponse
			r.Schema(t.Context(), frameworkresource.SchemaRequest{}, &schema)
			state := tfsdk.State{Schema: schema.Schema}
			diags := state.Set(t.Context(), types.ObjectValueMust(schema.Schema.Type().(types.ObjectType).AttributeTypes(), map[string]attr.Value{
				"id":          types.StringValue("errors/team?prod"),
				"name":        types.StringValue("errors/team?prod"),
				"description": types.StringValue(""),
				"apl_query":   types.StringValue("['logs']"),
				"datasets":    types.SetValueMust(types.StringType, []attr.Value{types.StringValue("logs")}),
			}))
			require.False(t, diags.HasError(), "%v", diags)

			var result diag.Diagnostics
			switch tc.method {
			case http.MethodGet:
				resp := frameworkresource.ReadResponse{State: state}
				r.Read(t.Context(), frameworkresource.ReadRequest{State: state}, &resp)
				result = resp.Diagnostics
				assert.Equal(t, tc.status == http.StatusNotFound, resp.State.Raw.IsNull())
				if tc.status != http.StatusNotFound {
					assert.True(t, state.Raw.Equal(resp.State.Raw), "failed reads must preserve state")
				}
			case http.MethodPut:
				resp := frameworkresource.UpdateResponse{State: state}
				r.Update(t.Context(), frameworkresource.UpdateRequest{
					State: state,
					Plan:  tfsdk.Plan{Schema: schema.Schema, Raw: state.Raw},
				}, &resp)
				result = resp.Diagnostics
				assert.True(t, state.Raw.Equal(resp.State.Raw), "failed updates must preserve state")
			case http.MethodDelete:
				resp := frameworkresource.DeleteResponse{State: state}
				r.Delete(t.Context(), frameworkresource.DeleteRequest{State: state}, &resp)
				result = resp.Diagnostics
			}
			if tc.status == http.StatusNotFound {
				assert.False(t, result.HasError(), "%v", result)
			} else {
				require.True(t, result.HasError())
				assert.Contains(t, result.Errors()[0].Detail(), "view API error")
			}
		})
	}
}
