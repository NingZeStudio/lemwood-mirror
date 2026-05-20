package config

import "testing"

func TestNormalizeMaxVersions(t *testing.T) {
	tests := []struct {
		name string
		in   int
		want int
	}{
		{name: "negative uses default", in: -1, want: 3},
		{name: "zero uses default", in: 0, want: 3},
		{name: "positive keeps value", in: 5, want: 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NormalizeMaxVersions(tt.in); got != tt.want {
				t.Fatalf("NormalizeMaxVersions(%d) = %d, want %d", tt.in, got, tt.want)
			}
		})
	}
}
