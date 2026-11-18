package axiom

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"regexp"
	"slices"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/ephemeral"
	ephemeralschema "github.com/hashicorp/terraform-plugin-framework/ephemeral/schema"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	fwresource "github.com/hashicorp/terraform-plugin-framework/resource"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/echoprovider"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/axiomhq/axiom-go/axiom"
)

// fakeTokensAPI is an in-memory stand-in for the Axiom tokens API that
// records which tokens were created and which were deleted.
type fakeTokensAPI struct {
	mu       sync.Mutex
	nextID   int
	requests []axiom.CreateTokenRequest
	live     map[string]axiom.APIToken
	deleted  []string

	regenerated int
}

func newFakeTokensAPI(t *testing.T) (*fakeTokensAPI, *httptest.Server) {
	t.Helper()

	// axiom-go lets these variables override the provider configuration,
	// which would send requests to a real deployment instead of the fake.
	for _, env := range []string{"AXIOM_URL", "AXIOM_TOKEN", "AXIOM_ORG_ID", "AXIOM_EDGE_URL", "AXIOM_EDGE", "AXIOM_API_TOKEN", "AXIOM_BASE_URL"} {
		t.Setenv(env, "")
	}

	api := &fakeTokensAPI{live: map[string]axiom.APIToken{}}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /v2/tokens", api.createToken)
	mux.HandleFunc("GET /v2/tokens/{id}", api.getToken)
	mux.HandleFunc("POST /v2/tokens/{id}/regenerate", api.regenerateToken)
	mux.HandleFunc("DELETE /v2/tokens/{id}", api.deleteToken)

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	return api, srv
}

func (api *fakeTokensAPI) createToken(w http.ResponseWriter, r *http.Request) {
	var req axiom.CreateTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	api.mu.Lock()
	defer api.mu.Unlock()

	writeJSON(w, http.StatusOK, api.issue(req))
}

// issue stores a new token and returns it with its value. The caller must
// hold the lock.
func (api *fakeTokensAPI) issue(req axiom.CreateTokenRequest) axiom.CreateTokenResponse {
	api.nextID++
	token := axiom.APIToken{
		ID:                       fmt.Sprintf("token-%d", api.nextID),
		Name:                     req.Name,
		Description:              req.Description,
		ExpiresAt:                req.ExpiresAt,
		DatasetCapabilities:      req.DatasetCapabilities,
		OrganisationCapabilities: req.OrganisationCapabilities,
	}
	api.requests = append(api.requests, req)
	api.live[token.ID] = token

	return axiom.CreateTokenResponse{APIToken: token, Token: "xaat-fake-" + token.ID}
}

func (api *fakeTokensAPI) getToken(w http.ResponseWriter, r *http.Request) {
	api.mu.Lock()
	defer api.mu.Unlock()

	token, ok := api.live[r.PathValue("id")]
	if !ok {
		writeError(w, http.StatusNotFound, "token not found")
		return
	}

	writeJSON(w, http.StatusOK, token)
}

// regenerateToken replaces a token with a new one that has a new ID, like the
// real API. The grace period of the previous token is not modeled.
func (api *fakeTokensAPI) regenerateToken(w http.ResponseWriter, r *http.Request) {
	var req axiom.RegenerateTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	api.mu.Lock()
	defer api.mu.Unlock()

	id := r.PathValue("id")
	if _, ok := api.live[id]; !ok {
		writeError(w, http.StatusNotFound, "token not found")
		return
	}
	if req.NewToken == nil {
		writeError(w, http.StatusBadRequest, "newToken is required")
		return
	}

	delete(api.live, id)
	api.regenerated++
	writeJSON(w, http.StatusOK, api.issue(*req.NewToken))
}

func (api *fakeTokensAPI) deleteToken(w http.ResponseWriter, r *http.Request) {
	api.mu.Lock()
	defer api.mu.Unlock()

	id := r.PathValue("id")
	if _, ok := api.live[id]; !ok {
		writeError(w, http.StatusNotFound, "token not found")
		return
	}

	delete(api.live, id)
	api.deleted = append(api.deleted, id)
	w.WriteHeader(http.StatusNoContent)
}

func (api *fakeTokensAPI) snapshot() (requests []axiom.CreateTokenRequest, live, deleted int) {
	api.mu.Lock()
	defer api.mu.Unlock()

	return slices.Clone(api.requests), len(api.live), len(api.deleted)
}

var testEphemeralTokenProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"axiom": providerserver.NewProtocol6WithError(NewAxiomProvider()),
	// The echo provider copies the ephemeral value into the state of a test
	// resource so that the test can assert on it.
	"echo": echoprovider.NewProviderServer(),
}

var testEphemeralTokenVersionChecks = []tfversion.TerraformVersionCheck{
	tfversion.SkipBelow(tfversion.Version1_10_0),
}

const testEphemeralTokenEcho = `
provider "echo" {
  data = ephemeral.axiom_token.test
}

resource "echo" "test" {}
`

func TestEphemeralToken_deletedOnClose(t *testing.T) {
	api, srv := newFakeTokensAPI(t)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testEphemeralTokenProviderFactories,
		TerraformVersionChecks:   testEphemeralTokenVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testRBACProviderConfig(srv.URL) + `
ephemeral "axiom_token" "test" {
  name        = "run-token"
  description = "created for a single run"

  dataset_capabilities = {
    "logs" = { ingest = ["create"] }
  }

  org_capabilities = {
    dashboards = ["read"]
  }
}
` + testEphemeralTokenEcho,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("echo.test", tfjsonpath.New("data").AtMapKey("token"), knownvalue.StringRegexp(regexp.MustCompile(`^xaat-fake-token-\d+$`))),
					statecheck.ExpectKnownValue("echo.test", tfjsonpath.New("data").AtMapKey("id"), knownvalue.StringRegexp(regexp.MustCompile(`^token-\d+$`))),
					statecheck.ExpectKnownValue("echo.test", tfjsonpath.New("data").AtMapKey("name"), knownvalue.StringExact("run-token")),
				},
			},
		},
	})

	requests, live, deleted := api.snapshot()
	require.NotEmpty(t, requests, "expected the ephemeral resource to create tokens")
	assert.Zero(t, live, "every token created during the run must be deleted on close")
	assert.Equal(t, len(requests), deleted)

	for _, req := range requests {
		assert.Equal(t, "run-token", req.Name)
		assert.Equal(t, "created for a single run", req.Description)
		assert.True(t, req.ExpiresAt.IsZero())
		require.Len(t, req.DatasetCapabilities, 1)
		assert.Equal(t, []axiom.Action{axiom.ActionCreate}, req.DatasetCapabilities["logs"].Ingest)
		assert.Empty(t, req.DatasetCapabilities["logs"].Query)
		assert.Equal(t, []axiom.Action{axiom.ActionRead}, req.OrganisationCapabilities.Dashboards)
	}
}

func TestEphemeralToken_keptOnClose(t *testing.T) {
	api, srv := newFakeTokensAPI(t)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testEphemeralTokenProviderFactories,
		TerraformVersionChecks:   testEphemeralTokenVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testRBACProviderConfig(srv.URL) + `
ephemeral "axiom_token" "test" {
  name            = "handoff-token"
  expires_at      = "2030-01-01T00:00:00Z"
  delete_on_close = false

  org_capabilities = {
    dashboards = ["read"]
  }
}
` + testEphemeralTokenEcho,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("echo.test", tfjsonpath.New("data").AtMapKey("token"), knownvalue.StringRegexp(regexp.MustCompile(`^xaat-fake-token-\d+$`))),
				},
			},
		},
	})

	requests, live, deleted := api.snapshot()
	require.NotEmpty(t, requests)
	assert.Zero(t, deleted, "tokens must be kept when delete_on_close is false")
	assert.Equal(t, len(requests), live)

	for _, req := range requests {
		assert.Equal(t, "2030-01-01T00:00:00Z", req.ExpiresAt.Format("2006-01-02T15:04:05Z07:00"))
	}
}

func TestEphemeralToken_validation(t *testing.T) {
	api, srv := newFakeTokensAPI(t)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testEphemeralTokenProviderFactories,
		TerraformVersionChecks:   testEphemeralTokenVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testRBACProviderConfig(srv.URL) + `
ephemeral "axiom_token" "test" {
  name            = "handoff-token"
  delete_on_close = false
}
` + testEphemeralTokenEcho,
				ExpectError: regexp.MustCompile(`expires_at is required when delete_on_close is false`),
			},
			{
				Config: testRBACProviderConfig(srv.URL) + `
ephemeral "axiom_token" "test" {
  name = "run-token"

  dataset_capabilities = {
    "logs" = { ingest = ["delete"] }
  }
}
` + testEphemeralTokenEcho,
				ExpectError: regexp.MustCompile(`(?s)ingest.*value must be one of`),
			},
		},
	})

	requests, _, _ := api.snapshot()
	assert.Empty(t, requests, "invalid configurations must not create tokens")
}

// TestEphemeralToken_capabilitiesMatchResource guards against the ephemeral
// resource drifting from the capabilities the axiom_token resource accepts.
func TestEphemeralToken_capabilitiesMatchResource(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	var resourceResp fwresource.SchemaResponse
	NewTokenResource().Schema(ctx, fwresource.SchemaRequest{}, &resourceResp)

	var ephemeralResp ephemeral.SchemaResponse
	NewTokenEphemeralResource().Schema(ctx, ephemeral.SchemaRequest{}, &ephemeralResp)

	resourceDataset := resourceResp.Schema.Attributes["dataset_capabilities"].(resourceschema.MapNestedAttribute).NestedObject.Attributes
	ephemeralDataset := ephemeralResp.Schema.Attributes["dataset_capabilities"].(ephemeralschema.MapNestedAttribute).NestedObject.Attributes
	assert.ElementsMatch(t, slices.Collect(maps.Keys(resourceDataset)), slices.Collect(maps.Keys(ephemeralDataset)))

	resourceOrg := resourceResp.Schema.Attributes["org_capabilities"].(resourceschema.SingleNestedAttribute).Attributes
	ephemeralOrg := ephemeralResp.Schema.Attributes["org_capabilities"].(ephemeralschema.SingleNestedAttribute).Attributes
	assert.ElementsMatch(t, slices.Collect(maps.Keys(resourceOrg)), slices.Collect(maps.Keys(ephemeralOrg)))
}

const testManagedTokenHandoffEphemeral = `
ephemeral "axiom_token" "test" {
  id = axiom_token_stateless.test.id
}

provider "echo" {
  data = ephemeral.axiom_token.test
}
`

// resetTokenHandoff forgets every handed-over token value, as happens when
// the provider process of a Terraform run exits.
func resetTokenHandoff() {
	tokenHandoff.mu.Lock()
	defer tokenHandoff.mu.Unlock()

	clear(tokenHandoff.values)
}

func testManagedTokenHandoffConfig(url, tokenArgs string, echoResources ...string) string {
	config := testRBACProviderConfig(url) + `
resource "axiom_token_stateless" "test" {
  name = "collector"
  ` + tokenArgs + `

  dataset_capabilities = {
    "logs" = { ingest = ["create"] }
  }
}
` + testManagedTokenHandoffEphemeral

	for _, name := range echoResources {
		config += fmt.Sprintf("\nresource \"echo\" %q {}\n", name)
	}
	return config
}

// TestEphemeralToken_managedTokenHandoff covers a managed token whose value
// is kept out of the state and reaches consumers through the ephemeral
// resource, only during the apply that issues it.
func TestEphemeralToken_managedTokenHandoff(t *testing.T) {
	api, srv := newFakeTokensAPI(t)

	tokenValue := tfjsonpath.New("data").AtMapKey("token")

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testEphemeralTokenProviderFactories,
		TerraformVersionChecks:   testEphemeralTokenVersionChecks,
		Steps: []resource.TestStep{
			{
				// Creating the token hands its value to the ephemeral resource.
				Config: testManagedTokenHandoffConfig(srv.URL, "", "created"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("axiom_token_stateless.test", tfjsonpath.New("id"), knownvalue.StringExact("token-1")),
					statecheck.ExpectKnownValue("echo.created", tokenValue, knownvalue.StringExact("xaat-fake-token-1")),
				},
			},
			{
				// A run that issues no new value has nothing to hand over.
				// Every Terraform run starts a new provider process, which
				// the test reproduces by clearing the handed-over values.
				PreConfig: resetTokenHandoff,
				Config:    testManagedTokenHandoffConfig(srv.URL, "", "created", "unchanged"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("axiom_token_stateless.test", tfjsonpath.New("id"), knownvalue.StringExact("token-1")),
					statecheck.ExpectKnownValue("echo.unchanged", tokenValue, knownvalue.StringExact("")),
				},
			},
			{
				// Changing the token regenerates it and hands over the new
				// value.
				PreConfig: resetTokenHandoff,
				Config:    testManagedTokenHandoffConfig(srv.URL, `description = "rotated"`, "created", "unchanged", "rotated"),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("axiom_token_stateless.test", tfjsonpath.New("id"), knownvalue.StringExact("token-2")),
					statecheck.ExpectKnownValue("echo.rotated", tokenValue, knownvalue.StringExact("xaat-fake-token-2")),
				},
			},
		},
	})

	requests, _, _ := api.snapshot()
	assert.Len(t, requests, 2, "the token must only be issued on create and on rotation")
	assert.Equal(t, 1, api.regenerated)
}

// TestTokenStatelessResource_noValueInState checks that the resource has no attribute
// that could hold the token value, and that importing a token needs none.
func TestTokenStatelessResource_noValueInState(t *testing.T) {
	_, srv := newFakeTokensAPI(t)

	var schemaResp fwresource.SchemaResponse
	(&TokenStatelessResource{}).Schema(t.Context(), fwresource.SchemaRequest{}, &schemaResp)
	assert.NotContains(t, schemaResp.Schema.Attributes, "token")
	for name, attribute := range schemaResp.Schema.Attributes {
		assert.False(t, attribute.IsSensitive(), "attribute %s", name)
	}

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testRBACProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testRBACProviderConfig(srv.URL) + `
resource "axiom_token_stateless" "test" {
  name = "collector"

  org_capabilities = {
    dashboards = ["read"]
  }
}
`,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("axiom_token_stateless.test", tfjsonpath.New("id"), knownvalue.StringExact("token-1")),
					statecheck.ExpectKnownValue("axiom_token_stateless.test", tfjsonpath.New("name"), knownvalue.StringExact("collector")),
				},
			},
			{
				ResourceName:      "axiom_token_stateless.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestEphemeralToken_idConflicts(t *testing.T) {
	_, srv := newFakeTokensAPI(t)

	resource.UnitTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testEphemeralTokenProviderFactories,
		TerraformVersionChecks:   testEphemeralTokenVersionChecks,
		Steps: []resource.TestStep{
			{
				Config: testRBACProviderConfig(srv.URL) + `
ephemeral "axiom_token" "test" {
  id   = "token-1"
  name = "collector"
}
` + testEphemeralTokenEcho,
				ExpectError: regexp.MustCompile(`name cannot be set together with id`),
			},
			{
				Config: testRBACProviderConfig(srv.URL) + `
ephemeral "axiom_token" "test" {}
` + testEphemeralTokenEcho,
				ExpectError: regexp.MustCompile(`name is required to create a token`),
			},
		},
	})
}
