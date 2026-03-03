package axiom

import "testing"

func TestProviderUserAgent(t *testing.T) {
	original := providerVersion
	t.Cleanup(func() {
		providerVersion = original
	})

	tests := []struct {
		name     string
		version  string
		expected string
	}{
		{
			name:     "empty falls back to dev",
			version:  "",
			expected: "terraform-provider-axiom/vdev",
		},
		{
			name:     "plain semver gets v prefix",
			version:  "1.5.0",
			expected: "terraform-provider-axiom/v1.5.0",
		},
		{
			name:     "v-prefixed semver remains unchanged",
			version:  "v1.5.0",
			expected: "terraform-provider-axiom/v1.5.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			providerVersion = tt.version

			if got := providerUserAgent(); got != tt.expected {
				t.Fatalf("providerUserAgent() = %q, expected %q", got, tt.expected)
			}
		})
	}
}
