package axiom

import (
	"errors"
	"fmt"
	"testing"

	ax "github.com/axiomhq/axiom-go/axiom"
)

func TestIsNotFoundError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "axiom ErrNotFound",
			err:  ax.ErrNotFound,
			want: true,
		},
		{
			name: "wrapped value HTTPError 404",
			err:  fmt.Errorf("wrapped: %w", ax.HTTPError{Status: 404, Message: "Not Found"}),
			want: true,
		},
		{
			name: "wrapped pointer HTTPError 404",
			err:  fmt.Errorf("wrapped: %w", &ax.HTTPError{Status: 404, Message: "Not Found"}),
			want: true,
		},
		{
			name: "other status",
			err:  ax.HTTPError{Status: 500, Message: "Internal Server Error"},
			want: false,
		},
		{
			name: "generic error",
			err:  errors.New("boom"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isNotFoundError(tt.err)
			if got != tt.want {
				t.Fatalf("isNotFoundError() = %v, want %v", got, tt.want)
			}
		})
	}
}
