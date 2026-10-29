package dices

import (
	"testing"
)

func TestContribution(t *testing.T) {
	t.Parallel()

	faces := []uint{2, 5, 4} // sum = 11

	tests := []struct {
		name       string
		modifier   int
		multiplier int
		sub        bool
		want       int
	}{
		{name: "plain sum", modifier: 0, multiplier: 1, want: 11},
		{name: "positive modifier", modifier: 3, multiplier: 1, want: 14},
		{name: "negative modifier", modifier: -3, multiplier: 1, want: 8},
		{name: "multiplier 2", modifier: 0, multiplier: 2, want: 22},
		{name: "modifier then multiplier", modifier: 3, multiplier: 2, want: 28},
		{name: "sub negates", modifier: 3, multiplier: 2, sub: true, want: -28},
		{name: "multiplier 0 means 1", modifier: 0, multiplier: 0, want: 11},
		{name: "negative multiplier", modifier: 0, multiplier: -2, want: -22},
		{name: "sub with negative multiplier", modifier: 0, multiplier: -2, sub: true, want: 22},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			die := Die{
				Modifier:   tt.modifier,
				Multiplier: tt.multiplier,
				Sub:        tt.sub,
			}
			if got := contribution(die, faces); got != tt.want {
				t.Fatalf("contribution=%d, want %d", got, tt.want)
			}
		})
	}
}

func TestContribution_EmptyRollsWithModifier(t *testing.T) {
	t.Parallel()

	die := Die{Modifier: 5, Multiplier: 1}
	if got := contribution(die, nil); got != 5 {
		t.Fatalf("contribution=%d, want 5", got)
	}
}
