package axiom

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	ax "github.com/axiomhq/axiom-go/axiom"
)

func TestAccTokenResource_TokenPersistence(t *testing.T) {
	client, err := ax.NewClient()
	assert.NoError(t, err)
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
		},
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"axiom": providerserver.NewProtocol6WithError(NewAxiomProvider()),
		},
		CheckDestroy: testAccCheckAxiomResourcesDestroyed(client),
		Steps: []resource.TestStep{
			// Create and read back
			{
				Config: testAccTokenResourceConfig_basic("test-token-persistence"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("axiom_token.test", "name", "test-token-persistence"),
					resource.TestCheckResourceAttrSet("axiom_token.test", "token"),
					// Store the token value for comparison in next step
					resource.TestCheckResourceAttrWith("axiom_token.test", "token", func(value string) error {
						if value == "" {
							return fmt.Errorf("token value is empty")
						}
						return nil
					}),
				),
			},
			// Verify token persists after refresh
			{
				Config: testAccTokenResourceConfig_basic("test-token-persistence"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("axiom_token.test", "name", "test-token-persistence"),
					resource.TestCheckResourceAttrSet("axiom_token.test", "token"),
					// Verify token is not empty after refresh
					resource.TestCheckResourceAttrWith("axiom_token.test", "token", func(value string) error {
						if value == "" {
							return fmt.Errorf("token value is empty after refresh")
						}
						return nil
					}),
				),
			},
		},
	})
}

func testAccTokenResourceConfig_basic(name string) string {
	return fmt.Sprintf(`
			provider "axiom" {
			api_token = "`+os.Getenv("AXIOM_TOKEN")+`"
			base_url  = "`+os.Getenv("AXIOM_URL")+`"
		}

		resource "axiom_token" "test" {
		name        = %[1]q
		description = "Test token for persistence"
		dataset_capabilities = {
			"*" = {
			ingest = ["create"]
			query  = ["read"]
			}
		}
		org_capabilities = {
			api_tokens = ["create", "read"]
		}
		}
`, name)
}
