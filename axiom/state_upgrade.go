package axiom

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource"
)

// schemaProvider is satisfied by every resource in this provider, since they
// all expose their schema through the framework's Schema method.
type schemaProvider interface {
	Schema(context.Context, resource.SchemaRequest, *resource.SchemaResponse)
}

// passthroughImportStateUpgraders returns a state upgrader that accepts schema
// version 0 and re-emits it unchanged under the resource's current (version 1)
// schema.
//
// Native `terraform import` already produces state tagged with the current
// schema version, so this path is not exercised by Terraform itself. The Pulumi
// Terraform bridge, however, tags freshly imported state as version 0 and then
// calls UpgradeResourceState(version: 0). Without a registered version-0
// upgrader the framework aborts with "Unable to Upgrade Resource State",
// breaking imports of any version-1 resource.
//
// Because the attribute shape has never differed between version 0 and 1 (all
// resources were introduced at version 1), the prior schema is just the current
// schema stamped as version 0, and upgrading is a straight passthrough of the
// decoded raw state. Read then hydrates the remaining attributes from the API
// using the imported id.
//
// Resources whose schema is still at version 0 (e.g. axiom_dashboard) do not
// need this: the framework round-trips state whenever the incoming version
// already matches the current schema version.
func passthroughImportStateUpgraders(ctx context.Context, r schemaProvider) map[int64]resource.StateUpgrader {
	var schemaResp resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &schemaResp)

	priorSchema := schemaResp.Schema
	priorSchema.Version = 0

	return map[int64]resource.StateUpgrader{
		0: {
			PriorSchema:   &priorSchema,
			StateUpgrader: passthroughStateUpgradeV0,
		},
	}
}

// passthroughStateUpgradeV0 copies the prior (version 0) state into the current
// schema. The prior schema is structurally identical to the current one, so the
// decoded raw value already conforms to the current schema type and can be
// emitted verbatim.
func passthroughStateUpgradeV0(_ context.Context, req resource.UpgradeStateRequest, resp *resource.UpgradeStateResponse) {
	if req.State == nil {
		return
	}

	resp.State.Raw = req.State.Raw
}
