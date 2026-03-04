package axiom

import (
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/axiomhq/axiom-go/axiom"
)

func TestExtractRotationGracePeriod(t *testing.T) {
	t.Parallel()

	t.Run("defaults when unset", func(t *testing.T) {
		t.Parallel()

		config := TokensResourceModel{RotationGracePeriod: types.StringNull()}

		gracePeriod, diags := extractRotationGracePeriod(config)

		require.False(t, diags.HasError())
		assert.Equal(t, defaultRotationGracePeriod, gracePeriod)
	})

	t.Run("uses configured duration", func(t *testing.T) {
		t.Parallel()

		config := TokensResourceModel{RotationGracePeriod: types.StringValue("45s")}

		gracePeriod, diags := extractRotationGracePeriod(config)

		require.False(t, diags.HasError())
		assert.Equal(t, 45*time.Second, gracePeriod)
	})

	t.Run("rejects invalid duration", func(t *testing.T) {
		t.Parallel()

		config := TokensResourceModel{RotationGracePeriod: types.StringValue("soon")}

		_, diags := extractRotationGracePeriod(config)

		require.True(t, diags.HasError())
		assert.Contains(t, diags[0].Summary(), "Invalid Rotation Grace Period")
	})

	t.Run("rejects negative duration", func(t *testing.T) {
		t.Parallel()

		config := TokensResourceModel{RotationGracePeriod: types.StringValue("-1s")}

		_, diags := extractRotationGracePeriod(config)

		require.True(t, diags.HasError())
		assert.Contains(t, diags[0].Detail(), "zero or greater")
	})
}

func TestValidateDatasetCapabilities(t *testing.T) {
	t.Parallel()

	err := validateDatasetCapabilities(map[string]axiom.DatasetCapabilities{
		"dataset-a": {},
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least one dataset capability")

	err = validateDatasetCapabilities(map[string]axiom.DatasetCapabilities{
		"dataset-a": {Ingest: []axiom.Action{axiom.ActionCreate}},
	})

	require.NoError(t, err)
}
