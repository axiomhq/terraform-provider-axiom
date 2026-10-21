package axiom

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/stretchr/testify/assert"
)

func TestLabelResourceLifecycle(t *testing.T) {
	for _, env := range []string{"AXIOM_URL", "AXIOM_TOKEN", "AXIOM_ORG_ID", "AXIOM_EDGE_URL", "AXIOM_EDGE", "AXIOM_API_TOKEN", "AXIOM_BASE_URL"} {
		t.Setenv(env, "")
	}
	var mu sync.Mutex
	var stored map[string]any
	mux := http.NewServeMux()
	handler := func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if r.Method != http.MethodPost && stored == nil {
			writeError(w, 404, "label not found")
			return
		}
		switch r.Method {
		case http.MethodPost, http.MethodPut:
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
				w.WriteHeader(400)
				return
			}
			color, ok := body["color"].(map[string]any)
			if !assert.True(t, ok) {
				w.WriteHeader(400)
				return
			}
			assert.Equal(t, "palette", color["type"])
			stored = body
			stored["id"] = "lbl_test"
			stored["resources_count"] = map[string]int{"dashboards": 0, "datasets": 0, "monitors": 0, "notifiers": 0, "total": 0}
			if stored["description"] == "" {
				delete(stored, "description")
			}
			status := 200
			if r.Method == http.MethodPost {
				status = 201
			}
			writeJSON(w, status, stored)
		case http.MethodGet:
			stored["assigned_resources"] = []any{}
			writeJSON(w, 200, stored)
		case http.MethodDelete:
			stored = nil
			w.WriteHeader(204)
		default:
			w.WriteHeader(405)
		}
	}
	mux.HandleFunc("/v2/labels", handler)
	mux.HandleFunc("/v2/labels/lbl_test", handler)
	srv := httptest.NewServer(mux)
	defer srv.Close()
	config := func(body string) string {
		return fmt.Sprintf(`provider "axiom" {
  api_token = "xaat-local-test"
  base_url = %q
}
resource "axiom_label" "test" {
  %s
}`, srv.URL, body)
	}
	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"axiom": providerserver.NewProtocol6WithError(NewAxiomProvider()),
		},
		Steps: []resource.TestStep{
			{
				Config: config(`name = "Production"`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("axiom_label.test", "id", "lbl_test"),
					resource.TestCheckResourceAttr("axiom_label.test", "color", "blue"),
					resource.TestCheckResourceAttr("axiom_label.test", "description", ""),
				),
			},
			{
				Config: config("name = \"Production services\"\ndescription = \"Customer-facing services\"\ncolor = \"iris\""),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("axiom_label.test", "name", "Production services"),
					resource.TestCheckResourceAttr("axiom_label.test", "description", "Customer-facing services"),
					resource.TestCheckResourceAttr("axiom_label.test", "color", "iris"),
				),
			},
			{ResourceName: "axiom_label.test", ImportState: true, ImportStateVerify: true},
			{
				Config: config(`name = "Production"`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("axiom_label.test", "description", ""),
					resource.TestCheckResourceAttr("axiom_label.test", "color", "blue"),
				),
			},
			{
				PreConfig: func() { mu.Lock(); stored = nil; mu.Unlock() },
				Config:    config(`name = "Production"`),
				Check:     resource.TestCheckResourceAttr("axiom_label.test", "id", "lbl_test"),
			},
			{Config: config("name = \"Production\"\ncolor = \"#123456\""), ExpectError: regexp.MustCompile("Attribute color value must be one of")},
			{Config: config(`name = "  Production  "`), ExpectError: regexp.MustCompile("Use single spaces")},
			{Config: config(`name = "Production"`)},
		},
	})
	mu.Lock()
	defer mu.Unlock()
	assert.Nil(t, stored, "destroy should delete the label")
}
