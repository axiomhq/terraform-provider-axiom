package axiom

import (
	"fmt"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

func TestAccTokenResource_RegenerateOnUpdateWithRotationGracePeriod(t *testing.T) {
	var client *ax.Client

	resourceName := "axiom_token.test"
	tokenName := fmt.Sprintf("test-token-rotate-%s", uuid.NewString())
	var initialID string
	var initialToken string

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			var err error
			client, err = ax.NewClient()
			require.NoError(t, err)
		},
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"axiom": providerserver.NewProtocol6WithError(NewAxiomProvider()),
		},
		CheckDestroy: testAccCheckAxiomResourcesDestroyed(client),
		Steps: []resource.TestStep{
			{
				Config: testAccTokenResourceConfigWithDescription(tokenName, "before-rotation", ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "name", tokenName),
					resource.TestCheckResourceAttr(resourceName, "description", "before-rotation"),
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttrSet(resourceName, "token"),
					resource.TestCheckResourceAttrWith(resourceName, "id", func(value string) error {
						if value == "" {
							return fmt.Errorf("token id is empty")
						}
						initialID = value
						return nil
					}),
					resource.TestCheckResourceAttrWith(resourceName, "token", func(value string) error {
						if value == "" {
							return fmt.Errorf("token value is empty")
						}
						initialToken = value
						return nil
					}),
				),
			},
			{
				Config: testAccTokenResourceConfigWithDescription(tokenName, "after-rotation", "2h"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "name", tokenName),
					resource.TestCheckResourceAttr(resourceName, "description", "after-rotation"),
					resource.TestCheckResourceAttrSet(resourceName, "id"),
					resource.TestCheckResourceAttrSet(resourceName, "token"),
					resource.TestCheckResourceAttrWith(resourceName, "id", func(value string) error {
						if value == initialID {
							return fmt.Errorf("expected token id to change after regenerate update, but remained %q", value)
						}
						return nil
					}),
					resource.TestCheckResourceAttrWith(resourceName, "token", func(value string) error {
						if value == initialToken {
							return fmt.Errorf("expected token value to change after regenerate update")
						}
						return nil
					}),
				),
			},
		},
	})
}

func testAccTokenResourceConfig_basic(name string) string {
	return testAccTokenResourceConfigWithDescription(name, "Test token for persistence", "")
}

func testAccTokenResourceConfigWithDescription(name, description, rotationGracePeriod string) string {
	rotationGracePeriodConfig := ""
	if rotationGracePeriod != "" {
		rotationGracePeriodConfig = fmt.Sprintf("\n                rotation_grace_period = %q", rotationGracePeriod)
	}

	return fmt.Sprintf(`
			provider "axiom" {
			api_token = "`+os.Getenv("AXIOM_TOKEN")+`"
			base_url  = "`+os.Getenv("AXIOM_URL")+`"
			}

		resource "axiom_token" "test" {
                name        = %[1]q
		description = %[2]q
		dataset_capabilities = {
                        "*" = {
                        ingest = ["create"]
                        query  = ["read"]
                        }
                }
                org_capabilities = {
                        api_tokens = ["create", "read"]
                }
                %[3]s
                }
`, name, description, rotationGracePeriodConfig)
}
