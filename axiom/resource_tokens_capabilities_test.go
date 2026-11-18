package axiom

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTokenResource_orgCapabilities checks the organisation capabilities that
// are sent to and read back from the API, using a server that stores the
// request body as is.
func TestTokenResource_orgCapabilities(t *testing.T) {
	var (
		mu     sync.Mutex
		stored map[string]any
	)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /v2/tokens", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		body, err := io.ReadAll(r.Body)
		if !assert.NoError(t, err) || !assert.NoError(t, json.Unmarshal(body, &stored)) {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		stored["id"] = "token-1"

		w.Header().Set("Content-Type", "application/json")
		res := map[string]any{"token": "xaat-fake"}
		for k, v := range stored {
			res[k] = v
		}
		assert.NoError(t, json.NewEncoder(w).Encode(res))
	})
	mux.HandleFunc("GET /v2/tokens/{id}", func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		assert.NoError(t, json.NewEncoder(w).Encode(stored))
	})
	mux.HandleFunc("DELETE /v2/tokens/{id}", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	orgCapabilities := tfjsonpath.New("org_capabilities")
	read := knownvalue.ListExact([]knownvalue.Check{knownvalue.StringExact("read")})

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testRBACProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testRBACProviderConfig(srv.URL) + `
resource "axiom_token" "test" {
  name = "capabilities"

  org_capabilities = {
    # No longer a capability. Terraform drops attributes that an object
    # argument does not have, so this line is ignored rather than rejected.
    flows        = ["read"]
    integrations = ["read"]
    labels       = ["create", "read"]
    rbac         = ["read"]
    views        = ["read"]
  }
}
`,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("axiom_token.test", orgCapabilities.AtMapKey("labels"), knownvalue.ListExact([]knownvalue.Check{
						knownvalue.StringExact("create"),
						knownvalue.StringExact("read"),
					})),
					statecheck.ExpectKnownValue("axiom_token.test", orgCapabilities.AtMapKey("views"), read),
					statecheck.ExpectKnownValue("axiom_token.test", orgCapabilities.AtMapKey("integrations"), read),
					statecheck.ExpectKnownValue("axiom_token.test", orgCapabilities.AtMapKey("rbac"), read),
					statecheck.ExpectKnownValue("axiom_token.test", orgCapabilities.AtMapKey("users"), knownvalue.ListExact(nil)),
				},
			},
		},
	})

	mu.Lock()
	defer mu.Unlock()

	sent, ok := stored["orgCapabilities"].(map[string]any)
	require.True(t, ok, "orgCapabilities must be sent as an object")
	assert.Equal(t, map[string]any{
		"integrations": []any{"read"},
		"labels":       []any{"create", "read"},
		"rbac":         []any{"read"},
		"views":        []any{"read"},
	}, sent)
}
