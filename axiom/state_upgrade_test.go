package axiom

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpgradeResourceState_V0Passthrough(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	server := providerserver.NewProtocol6(NewAxiomProvider())()

	tests := []struct {
		typeName    string
		newResource func() resource.Resource
	}{
		{typeName: "axiom_monitor", newResource: NewMonitorResource},
		{typeName: "axiom_notifier", newResource: NewNotifierResource},
		{typeName: "axiom_user", newResource: NewUserResource},
		{typeName: "axiom_dataset", newResource: NewDatasetResource},
		{typeName: "axiom_virtual_field", newResource: NewVirtualFieldResource},
		{typeName: "axiom_token", newResource: NewTokenResource},
	}

	for _, test := range tests {
		t.Run(test.typeName, func(t *testing.T) {
			t.Parallel()

			resp, err := server.UpgradeResourceState(ctx, &tfprotov6.UpgradeResourceStateRequest{
				TypeName: test.typeName,
				Version:  0,
				RawState: &tfprotov6.RawState{JSON: []byte(`{"id":"imported_id"}`)},
			})
			require.NoError(t, err)
			require.NotNil(t, resp)
			require.Empty(t, protocolErrorDiagnostics(resp.Diagnostics))
			require.NotNil(t, resp.UpgradedState)

			var schemaResp resource.SchemaResponse
			test.newResource().Schema(ctx, resource.SchemaRequest{}, &schemaResp)
			require.False(t, schemaResp.Diagnostics.HasError(), "schema diagnostics: %v", schemaResp.Diagnostics)

			state, err := resp.UpgradedState.Unmarshal(schemaResp.Schema.Type().TerraformType(ctx))
			require.NoError(t, err)

			var attributes map[string]tftypes.Value
			require.NoError(t, state.As(&attributes))
			var id string
			require.NoError(t, attributes["id"].As(&id))
			assert.Equal(t, "imported_id", id)
		})
	}
}

func protocolErrorDiagnostics(diagnostics []*tfprotov6.Diagnostic) []string {
	var errors []string
	for _, diagnostic := range diagnostics {
		if diagnostic.Severity == tfprotov6.DiagnosticSeverityError {
			errors = append(errors, diagnostic.Summary+": "+diagnostic.Detail)
		}
	}
	return errors
}
