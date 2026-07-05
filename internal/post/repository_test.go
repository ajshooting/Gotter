package post

import (
	"errors"
	"strings"
	"testing"
)

func TestNormalizeBody(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    string
		wantErr error
	}{
		{name: "trims whitespace", input: "  hello  ", want: "hello"},
		{name: "empty", input: "   ", wantErr: ErrEmptyBody},
		{name: "too long", input: strings.Repeat("あ", MaxBodyLength+1), wantErr: ErrBodyTooLong},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := NormalizeBody(tt.input)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("body = %q, want %q", got, tt.want)
			}
		})
	}
}
