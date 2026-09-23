package axiom

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	frameworkresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/stretchr/testify/require"

	ax "github.com/axiomhq/axiom-go/axiom"
)

func TestAccAxiomResources_import(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("acceptance tests skipped unless TF_ACC is set")
	}
	testAccPreCheck(t)

	client, err := ax.NewClient()
	require.NoError(t, err)

	datasetName := "tf-import-" + uuid.NewString()
	config := testAccAxiomImportConfig(datasetName, "tf-import-"+uuid.NewString(),
		"tf-import-"+uuid.NewString(), "tf-import-"+uuid.NewString(), "tf-import-"+uuid.NewString())
	mplQuery := fmt.Sprintf("`%s`:`http_request_duration_seconds` | align to 1m using avg", datasetName)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"axiom": providerserver.NewProtocol6WithError(NewAxiomProvider()),
		},
		CheckDestroy: testAccCheckAxiomResourcesDestroyed(client),
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckAxiomResourcesExist(client, "axiom_dataset.test"),
					testAccCheckAxiomResourcesExist(client, "axiom_virtual_field.test"),
					testAccCheckAxiomResourcesExist(client, "axiom_notifier.test"),
					testAccCheckAxiomResourcesExist(client, "axiom_monitor.mpl"),
					testAccCheckAxiomResourcesExist(client, "axiom_monitor.apl"),
					testAccCheckAxiomResourcesExist(client, "axiom_token.test"),
					resource.TestCheckResourceAttr("axiom_monitor.mpl", "mpl_query", mplQuery),
					resource.TestCheckNoResourceAttr("axiom_monitor.mpl", "apl_query"),
				),
			},
			{
				ResourceName:      "axiom_dataset.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				// The importer accepts a plain ID; Read gets the dataset from the API.
				ResourceName:      "axiom_virtual_field.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				ResourceName:      "axiom_notifier.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// axiom_monitor.mpl is deliberately not imported: GET /monitors returns an
			// MPL monitor's query in aplQuery with mplQuery empty, so an id-only
			// import cannot tell the two apart and lands the query in apl_query.
			// Importing MPL monitors needs the API to return mplQuery.
			{
				ResourceName:      "axiom_monitor.apl",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				ResourceName:      "axiom_token.test",
				ImportState:       true,
				ImportStateVerify: true,
				// token: the API never returns the secret; Read in resource_tokens.go copies prior state.
				// rotation_grace_period: config-only, so the API does not return it.
				ImportStateVerifyIgnore: []string{"token", "rotation_grace_period"},
			},
		},
	})
}

func TestAccAxiomResources_upgrade_from_schema_v0(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("acceptance tests skipped unless TF_ACC is set")
	}
	testAccPreCheck(t)

	ctx := context.Background()
	v0Server := providerserver.NewProtocol6(schemaV0Provider{NewAxiomProvider()})()
	v0Schema, err := v0Server.GetProviderSchema(ctx, &tfprotov6.GetProviderSchemaRequest{})
	require.NoError(t, err)
	require.NotNil(t, v0Schema)
	require.Empty(t, v0Schema.Diagnostics)

	realServer := providerserver.NewProtocol6(NewAxiomProvider())()
	realSchema, err := realServer.GetProviderSchema(ctx, &tfprotov6.GetProviderSchemaRequest{})
	require.NoError(t, err)
	require.NotNil(t, realSchema)
	require.Empty(t, realSchema.Diagnostics)

	// If the wrapper stopped lowering schema versions, the plan would also pass
	// on main without exercising UpgradeResourceState(0).
	for _, typeName := range []string{
		"axiom_monitor", "axiom_notifier", "axiom_user",
		"axiom_dataset", "axiom_virtual_field", "axiom_token",
	} {
		require.NotNil(t, v0Schema.ResourceSchemas[typeName], typeName)
		require.Equal(t, int64(0), v0Schema.ResourceSchemas[typeName].Version, typeName)
		require.NotNil(t, realSchema.ResourceSchemas[typeName], typeName)
		require.Equal(t, int64(1), realSchema.ResourceSchemas[typeName].Version, typeName)
	}

	client, err := ax.NewClient()
	require.NoError(t, err)

	config := testAccAxiomImportConfig("tf-import-"+uuid.NewString(), "tf-import-"+uuid.NewString(),
		"tf-import-"+uuid.NewString(), "tf-import-"+uuid.NewString(), "tf-import-"+uuid.NewString())
	check := resource.ComposeTestCheckFunc(
		testAccCheckAxiomResourcesExist(client, "axiom_dataset.test"),
		testAccCheckAxiomResourcesExist(client, "axiom_virtual_field.test"),
		testAccCheckAxiomResourcesExist(client, "axiom_notifier.test"),
		testAccCheckAxiomResourcesExist(client, "axiom_monitor.mpl"),
		testAccCheckAxiomResourcesExist(client, "axiom_monitor.apl"),
		testAccCheckAxiomResourcesExist(client, "axiom_token.test"),
	)

	resource.Test(t, resource.TestCase{
		CheckDestroy: testAccCheckAxiomResourcesDestroyed(client),
		Steps: []resource.TestStep{
			{
				ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
					"axiom": providerserver.NewProtocol6WithError(schemaV0Provider{NewAxiomProvider()}),
				},
				Config: config,
				Check:  check,
			},
			{
				ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
					"axiom": providerserver.NewProtocol6WithError(NewAxiomProvider()),
				},
				Config: config,
				// Terraform reads version-0 state with a version-1 provider and calls
				// UpgradeResourceState(0). On main this fails with "Unable to Upgrade
				// Resource State"; a non-empty plan means the passthrough corrupted state.
				PlanOnly: true,
			},
			{
				ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
					"axiom": providerserver.NewProtocol6WithError(NewAxiomProvider()),
				},
				Config: config,
				Check:  check,
			},
		},
	})
}

// schemaV0Provider wraps the real provider so every resource reports schema
// version 0, letting terraform write version-0 state that the real provider
// must then upgrade.
type schemaV0Provider struct{ provider.Provider }

func (p schemaV0Provider) Resources(ctx context.Context) []func() frameworkresource.Resource {
	inner := p.Provider.Resources(ctx)
	out := make([]func() frameworkresource.Resource, 0, len(inner))
	for _, newResource := range inner {
		out = append(out, func() frameworkresource.Resource { return schemaV0Resource{newResource()} })
	}
	return out
}

type schemaV0Resource struct{ frameworkresource.Resource }

var (
	_ frameworkresource.ResourceWithConfigure   = schemaV0Resource{}
	_ frameworkresource.ResourceWithImportState = schemaV0Resource{}
	_ frameworkresource.ResourceWithModifyPlan  = schemaV0Resource{}
)

func (r schemaV0Resource) Schema(ctx context.Context, req frameworkresource.SchemaRequest, resp *frameworkresource.SchemaResponse) {
	r.Resource.Schema(ctx, req, resp)
	resp.Schema.Version = 0
}

func (r schemaV0Resource) Configure(ctx context.Context, req frameworkresource.ConfigureRequest, resp *frameworkresource.ConfigureResponse) {
	if inner, ok := r.Resource.(frameworkresource.ResourceWithConfigure); ok {
		inner.Configure(ctx, req, resp)
	}
}

func (r schemaV0Resource) ImportState(ctx context.Context, req frameworkresource.ImportStateRequest, resp *frameworkresource.ImportStateResponse) {
	if inner, ok := r.Resource.(frameworkresource.ResourceWithImportState); ok {
		inner.ImportState(ctx, req, resp)
	}
}

func (r schemaV0Resource) ModifyPlan(ctx context.Context, req frameworkresource.ModifyPlanRequest, resp *frameworkresource.ModifyPlanResponse) {
	if inner, ok := r.Resource.(frameworkresource.ResourceWithModifyPlan); ok {
		inner.ModifyPlan(ctx, req, resp)
	}
}

func testAccAxiomImportConfig(datasetName, notifierName, mplMonitorName, aplMonitorName, tokenName string) string {
	mplQuery := fmt.Sprintf("`%s`:`http_request_duration_seconds` | align to 1m using avg", datasetName)
	return fmt.Sprintf(`
provider "axiom" {
  api_token = %q
  base_url  = %q
}

resource "axiom_dataset" "test" {
  name        = %q
  description = "import test"
}

resource "axiom_virtual_field" "test" {
  name        = "vf_import"
  expression  = "a * b"
  description = "import test"
  dataset     = axiom_dataset.test.id
}

resource "axiom_notifier" "test" {
  name = %q
  properties = {
    slack = {
      slack_url = "https://hooks.slack.com/services/EXAMPLE/EXAMPLE/EXAMPLE"
    }
  }
}

resource "axiom_monitor" "mpl" {
  depends_on = [axiom_dataset.test]

  name             = %q
  mpl_query        = %q
  interval_minutes = 5
  operator         = "Above"
  range_minutes    = 5
  threshold        = 1
  type             = "Threshold"
}

resource "axiom_monitor" "apl" {
  depends_on = [axiom_dataset.test]

  name             = %q
  apl_query        = "['%s'] | summarize count()"
  interval_minutes = 5
  operator         = "Above"
  range_minutes    = 5
  threshold        = 1
  type             = "Threshold"
  notifier_ids     = [axiom_notifier.test.id]
}

resource "axiom_token" "test" {
  depends_on = [axiom_dataset.test]

  name        = %q
  description = "import test"
  expires_at  = "2027-06-29T13:02:54Z"
  dataset_capabilities = {
    %q = {
      ingest = ["create"]
      query  = ["read"]
    }
  }
  org_capabilities = {
    api_tokens = ["read"]
  }
}
`, os.Getenv("AXIOM_TOKEN"), os.Getenv("AXIOM_URL"), datasetName, notifierName,
		mplMonitorName, mplQuery, aplMonitorName, datasetName, tokenName, datasetName)
}
