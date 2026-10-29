package dices

import (
	"testing"
)

func TestApplyKeep(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		faces []uint
		keep  Keep
		want  []uint
	}{
		{
			name:  "no keep preserves order",
			faces: []uint{2, 6, 1, 4},
			keep:  Keep{},
			want:  []uint{2, 6, 1, 4},
		},
		{
			name:  "highest 3",
			faces: []uint{2, 6, 1, 4},
			keep:  Keep{Highest: 3},
			want:  []uint{2, 4, 6},
		},
		{
			name:  "lowest 2",
			faces: []uint{2, 6, 1, 4},
			keep:  Keep{Lowest: 2},
			want:  []uint{1, 2},
		},
		{
			name:  "highest 2 with duplicates",
			faces: []uint{5, 5, 2, 3},
			keep:  Keep{Highest: 2},
			want:  []uint{5, 5},
		},
		{
			name:  "highest equals len sorts all",
			faces: []uint{2, 6, 1, 4},
			keep:  Keep{Highest: 4},
			want:  []uint{1, 2, 4, 6},
		},
		{
			name:  "highest greater than len sorts all",
			faces: []uint{2, 6, 1, 4},
			keep:  Keep{Highest: 5},
			want:  []uint{1, 2, 4, 6},
		},
		{
			name:  "lowest 1",
			faces: []uint{2, 6, 1, 4},
			keep:  Keep{Lowest: 1},
			want:  []uint{1},
		},
		{
			name:  "highest 1",
			faces: []uint{2, 6, 1, 4},
			keep:  Keep{Highest: 1},
			want:  []uint{6},
		},
		{
			name:  "empty input",
			faces: []uint{},
			keep:  Keep{Highest: 2},
			want:  []uint{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := applyKeep(append([]uint(nil), tt.faces...), tt.keep)
			assertUintSliceEqual(t, got, tt.want)
		})
	}
}
