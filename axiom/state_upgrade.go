package axiom

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource"
)

// v0PassthroughUpgraders returns a version-0 state upgrader that copies state
// into the current schema unchanged.
//
// The framework only round-trips state on its own when the request version
// equals the schema version. Some protocol consumers (for example the Pulumi
// Terraform bridge when its own schema-version metadata is missing) request an
// upgrade from version 0 for resources at version 1. No released provider ever
// shipped these schemas at version 0, so version-0 state already has the
// current shape and can be copied verbatim.
//
// If a resource's attributes ever change alongside a version bump, that
// resource needs a real upgrader with the old shape as PriorSchema instead of
// this helper.
func v0PassthroughUpgraders(ctx context.Context, r resource.Resource) map[int64]resource.StateUpgrader {
	var schemaResp resource.SchemaResponse
	r.Schema(ctx, resource.SchemaRequest{}, &schemaResp)

	prior := schemaResp.Schema
	prior.Version = 0

	return map[int64]resource.StateUpgrader{
		0: {
			PriorSchema: &prior,
			StateUpgrader: func(_ context.Context, req resource.UpgradeStateRequest, resp *resource.UpgradeStateResponse) {
				resp.State.Raw = req.State.Raw
			},
		},
	}
}
