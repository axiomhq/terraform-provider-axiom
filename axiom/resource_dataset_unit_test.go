package axiom

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
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

func TestAttributeConflictsWithKind(t *testing.T) {
	t.Parallel()

	mapFields := types.ListValueMust(types.StringType, []attr.Value{types.StringValue("field1")})

	t.Run("conflicts when map fields are set on a metrics dataset", func(t *testing.T) {
		t.Parallel()

		assert.True(t, attributeConflictsWithKind(types.StringValue("otel:metrics:v1"), mapFields, metricsDatasetKind))
	})

	t.Run("conflicts when map fields are empty on a metrics dataset", func(t *testing.T) {
		t.Parallel()

		empty := types.ListValueMust(types.StringType, []attr.Value{})

		assert.True(t, attributeConflictsWithKind(types.StringValue("otel:metrics:v1"), empty, metricsDatasetKind))
	})

	t.Run("does not conflict when map fields are absent on a metrics dataset", func(t *testing.T) {
		t.Parallel()

		assert.False(t, attributeConflictsWithKind(types.StringValue("otel:metrics:v1"), types.ListNull(types.StringType), metricsDatasetKind))
		assert.False(t, attributeConflictsWithKind(types.StringValue("otel:metrics:v1"), types.ListUnknown(types.StringType), metricsDatasetKind))
	})

	t.Run("does not conflict for other kinds", func(t *testing.T) {
		t.Parallel()

		for _, kind := range []string{"axiom:events:v1", "otel:traces:v1", "otel:logs:v1"} {
			assert.False(t, attributeConflictsWithKind(types.StringValue(kind), mapFields, metricsDatasetKind), kind)
		}
	})

	t.Run("does not conflict when kind is not known", func(t *testing.T) {
		t.Parallel()

		assert.False(t, attributeConflictsWithKind(types.StringNull(), mapFields, metricsDatasetKind))
		assert.False(t, attributeConflictsWithKind(types.StringUnknown(), mapFields, metricsDatasetKind))
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
