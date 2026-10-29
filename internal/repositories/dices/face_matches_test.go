package dices

import (
	"testing"
)

func TestFaceMatches(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		face        uint8
		equal       uint8
		lowerThan   uint8
		greaterThan uint8
		want        bool
	}{
		{name: "equal match", face: 6, equal: 6, want: true},
		{name: "equal miss", face: 5, equal: 6, want: false},
		{name: "lt true", face: 2, lowerThan: 3, want: true},
		{name: "lt boundary equal is false", face: 3, lowerThan: 3, want: false},
		{name: "gt true", face: 5, greaterThan: 4, want: true},
		{name: "gt boundary equal is false", face: 4, greaterThan: 4, want: false},
		{name: "no conditions", face: 6, want: false},
		{name: "or equal wins", face: 1, equal: 1, lowerThan: 5, greaterThan: 10, want: true},
		{name: "or lt wins", face: 2, equal: 9, lowerThan: 3, greaterThan: 10, want: true},
		{name: "or gt wins", face: 8, equal: 1, lowerThan: 2, greaterThan: 7, want: true},
		{name: "or all miss", face: 5, equal: 1, lowerThan: 3, greaterThan: 7, want: false},
		{name: "zero thresholds ignored", face: 1, equal: 0, lowerThan: 0, greaterThan: 0, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := faceMatches(tt.face, tt.equal, tt.lowerThan, tt.greaterThan)
			if got != tt.want {
				t.Fatalf("faceMatches=%v, want %v", got, tt.want)
			}
		})
	}
}
