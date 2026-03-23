package axiom

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"

	"github.com/axiomhq/axiom-go/axiom"
)

func TestDatasetCreateRequestFromPlan(t *testing.T) {
	t.Parallel()

	request := datasetCreateRequestFromPlan(DatasetResourceModel{
		Name:               types.StringValue("dataset-a"),
		Kind:               types.StringValue("axiom:events:v1"),
		Description:        types.StringValue("dataset description"),
		EdgeDeployment:     types.StringValue("cloud.eu-central-1.aws"),
		UseRetentionPeriod: types.BoolValue(true),
		RetentionDays:      types.Int64Value(30),
	})

	assert.Equal(t, axiom.DatasetCreateRequest{
		Name:               "dataset-a",
		Kind:               "axiom:events:v1",
		Description:        "dataset description",
		EdgeDeployment:     "cloud.eu-central-1.aws",
		UseRetentionPeriod: true,
		RetentionDays:      30,
	}, request)
}

func TestDatasetCreateRequestFromPlan_NullEdgeDeployment(t *testing.T) {
	t.Parallel()

	request := datasetCreateRequestFromPlan(DatasetResourceModel{
		Name:           types.StringValue("dataset-a"),
		Kind:           types.StringValue("axiom:events:v1"),
		EdgeDeployment: types.StringNull(),
	})

	assert.Empty(t, request.EdgeDeployment)
}

func TestFlattenDataset_EdgeDeployment(t *testing.T) {
	t.Parallel()

	t.Run("sets edge deployment in state", func(t *testing.T) {
		t.Parallel()

		state := flattenDataset(&axiom.Dataset{
			ID:             "dataset-a",
			Name:           "dataset-a",
			Kind:           "axiom:events:v1",
			EdgeDeployment: "cloud.eu-central-1.aws",
		}, "")

		assert.Equal(t, "cloud.eu-central-1.aws", state.EdgeDeployment.ValueString())
	})

	t.Run("uses org default edge deployment when dataset omits it", func(t *testing.T) {
		t.Parallel()

		state := flattenDataset(&axiom.Dataset{
			ID:   "dataset-a",
			Name: "dataset-a",
			Kind: "axiom:events:v1",
		}, "cloud.us-east-1.aws")

		assert.Equal(t, "cloud.us-east-1.aws", state.EdgeDeployment.ValueString())
	})

	t.Run("keeps edge deployment null when absent", func(t *testing.T) {
		t.Parallel()

		state := flattenDataset(&axiom.Dataset{
			ID:   "dataset-a",
			Name: "dataset-a",
			Kind: "axiom:events:v1",
		}, "")

		assert.True(t, state.EdgeDeployment.IsNull())
	})
}

func TestSelectDefaultEdgeDeployment(t *testing.T) {
	t.Parallel()

	t.Run("returns first default edge deployment", func(t *testing.T) {
		t.Parallel()

		selected := selectDefaultEdgeDeployment([]*axiom.Organization{
			{DefaultEdgeDeployment: "cloud.eu-central-1.aws"},
			{DefaultEdgeDeployment: "cloud.us-east-1.aws"},
		})

		assert.Equal(t, "cloud.eu-central-1.aws", selected)
	})

	t.Run("skips nil and empty organizations", func(t *testing.T) {
		t.Parallel()

		selected := selectDefaultEdgeDeployment([]*axiom.Organization{
			nil,
			{},
			{DefaultEdgeDeployment: "cloud.us-east-1.aws"},
		})

		assert.Equal(t, "cloud.us-east-1.aws", selected)
	})

	t.Run("returns empty when none configured", func(t *testing.T) {
		t.Parallel()

		selected := selectDefaultEdgeDeployment([]*axiom.Organization{
			{},
		})

		assert.Empty(t, selected)
	})
}

func TestEdgeDeploymentValue(t *testing.T) {
	t.Parallel()

	t.Run("returns empty string for null", func(t *testing.T) {
		t.Parallel()

		assert.Empty(t, edgeDeploymentValue(types.StringNull()))
	})

	t.Run("returns empty string for unknown", func(t *testing.T) {
		t.Parallel()

		assert.Empty(t, edgeDeploymentValue(types.StringUnknown()))
	})

	t.Run("returns value when known", func(t *testing.T) {
		t.Parallel()

		assert.Equal(t, "cloud.eu-central-1.aws", edgeDeploymentValue(types.StringValue("cloud.eu-central-1.aws")))
	})
}
