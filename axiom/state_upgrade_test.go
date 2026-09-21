package axiom

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestImportUpgradesFromVersion0 asserts that every importable resource can
// upgrade the version-0 state produced by the Pulumi Terraform bridge during
// import into its current schema without error, preserving the imported id so
// Read can subsequently hydrate the resource.
func TestImportUpgradesFromVersion0(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	server := providerserver.NewProtocol6(NewAxiomProvider())()

	const importedID = "imported_id"

	// Every resource whose schema is at version 1 needs an explicit version-0
	// upgrader. axiom_dashboard is intentionally omitted: its schema is still at
	// version 0, so the framework round-trips version-0 state on its own.
	resources := map[string]func() resource.Resource{
		"axiom_monitor":       NewMonitorResource,
		"axiom_notifier":      NewNotifierResource,
		"axiom_user":          NewUserResource,
		"axiom_dataset":       NewDatasetResource,
		"axiom_virtual_field": NewVirtualFieldResource,
		"axiom_token":         NewTokenResource,
	}

	for typeName, newResource := range resources {
		t.Run(typeName, func(t *testing.T) {
			t.Parallel()

			var schemaResp resource.SchemaResponse
			newResource().(schemaProvider).Schema(ctx, resource.SchemaRequest{}, &schemaResp)
			schemaType := schemaResp.Schema.Type().TerraformType(ctx)

			upgradeResp, err := server.UpgradeResourceState(ctx, &tfprotov6.UpgradeResourceStateRequest{
				TypeName: typeName,
				Version:  0,
				RawState: &tfprotov6.RawState{JSON: []byte(`{"id":"` + importedID + `"}`)},
			})
			require.NoError(t, err)
			require.False(t, protoDiagnosticsHasError(upgradeResp.Diagnostics), protoDiagnostics(upgradeResp.Diagnostics))
			require.NotNil(t, upgradeResp.UpgradedState)

			upgradedVal, err := upgradeResp.UpgradedState.Unmarshal(schemaType)
			require.NoError(t, err)

			state := tfsdk.State{Schema: schemaResp.Schema, Raw: upgradedVal}

			var id types.String
			require.False(t, state.GetAttribute(ctx, path.Root("id"), &id).HasError())
			assert.Equal(t, importedID, id.ValueString())
		})
	}
}
