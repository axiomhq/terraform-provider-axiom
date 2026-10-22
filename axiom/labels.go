package axiom

import (
	"context"
	"fmt"
	"net/http"

	"github.com/axiomhq/axiom-go/axiom"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func labelsAttribute() schema.SetAttribute {
	return schema.SetAttribute{
		Optional:            true,
		Computed:            true,
		ElementType:         types.StringType,
		MarkdownDescription: "Set of existing Axiom label IDs assigned to this resource. Omit to leave assignments unchanged; use an empty set to clear all assignments.",
	}
}

type labelTarget struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

type labelAssignment struct {
	Target   labelTarget `json:"target"`
	LabelIDs []string    `json:"labelIds"`
}

// syncLabels runs after the resource state is saved so assignment failures do
// not orphan a newly created resource. A nil config means refresh/import.
func syncLabels(ctx context.Context, client *axiom.Client, kind, id string, config *tfsdk.Config, planned types.Set, state *tfsdk.State) diag.Diagnostics {
	var diags diag.Diagnostics
	labels := types.SetNull(types.StringType)
	if config != nil {
		diags.Append(config.GetAttribute(ctx, path.Root("labels"), &labels)...)
		if diags.HasError() {
			return diags
		}
		if !labels.IsNull() {
			labels = planned
		}
	}
	target := labelTarget{Type: kind, ID: id}
	var result labelAssignment
	if !labels.IsNull() && !labels.IsUnknown() {
		ids := []string{}
		diags.Append(labels.ElementsAs(ctx, &ids, false)...)
		if diags.HasError() {
			return diags
		}
		err := client.Call(ctx, http.MethodPut, "/v2/labels/assignments", labelAssignment{Target: target, LabelIDs: ids}, &result)
		if err != nil {
			diags.AddError("Unable to update labels", err.Error())
			return diags
		}
	} else {
		var results []labelAssignment
		err := client.Call(ctx, http.MethodPost, "/v2/labels/assignments", struct {
			Targets []labelTarget `json:"targets"`
		}{[]labelTarget{target}}, &results)
		if err != nil {
			diags.AddError("Unable to read labels", err.Error())
			return diags
		}
		if len(results) != 1 || results[0].Target != target {
			diags.AddError("Unable to read labels", "API returned an unexpected label assignment target.")
			return diags
		}
		result = results[0]
	}
	if result.LabelIDs == nil {
		result.LabelIDs = []string{}
	}
	actual, valueDiags := types.SetValueFrom(ctx, types.StringType, result.LabelIDs)
	diags.Append(valueDiags...)
	diags.Append(state.SetAttribute(ctx, path.Root("labels"), actual)...)
	if !labels.IsNull() && !labels.IsUnknown() && !labels.Equal(actual) {
		diags.AddError("Label assignments differ", fmt.Sprintf("Axiom did not assign all requested labels to %s %q. Check that the label IDs exist in this organization.", kind, id))
	}
	return diags
}
