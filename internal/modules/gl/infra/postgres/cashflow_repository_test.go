package postgres

import (
	"slices"
	"testing"
)

func TestBuildCashCodeSet(t *testing.T) {
	tests := []struct {
		name     string
		mappings []struct{ Key, Code string }
		want     []string
	}{
		{
			name:     "empty mappings",
			mappings: nil,
			want:     []string{"1001", "1002"},
		},
		{
			name:     "only cash mapping adds bank default",
			mappings: []struct{ Key, Code string }{{Key: "cash", Code: "1001"}},
			want:     []string{"1001", "1002"},
		},
		{
			name:     "only bank mapping adds cash default",
			mappings: []struct{ Key, Code string }{{Key: "bank", Code: "1002"}},
			want:     []string{"1002", "1001"},
		},
		{
			name:     "non numeric bank code keeps exact and adds cash default",
			mappings: []struct{ Key, Code string }{{Key: "bank", Code: "BANK1"}},
			want:     []string{"BANK1", "1001"},
		},
		{
			name:     "leaf bank code keeps exact and root",
			mappings: []struct{ Key, Code string }{{Key: "bank", Code: "1002.01"}},
			want:     []string{"1002.01", "1002", "1001"},
		},
		{
			name: "duplicate codes are deduplicated",
			mappings: []struct{ Key, Code string }{
				{Key: "cash", Code: "1001"},
				{Key: "cash", Code: " 1001 "},
				{Key: "bank", Code: "1002"},
				{Key: "bank", Code: "1002"},
			},
			want: []string{"1001", "1002"},
		},
		{
			name:     "blank code does not satisfy its key",
			mappings: []struct{ Key, Code string }{{Key: "cash", Code: " "}},
			want:     []string{"1001", "1002"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildCashCodeSet(tt.mappings)
			if !slices.Equal(got, tt.want) {
				t.Fatalf("buildCashCodeSet() = %v, want %v", got, tt.want)
			}
		})
	}
}
