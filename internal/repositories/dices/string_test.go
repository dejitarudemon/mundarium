package dices

import (
	"testing"
)

func TestString_KeepRerollBlow(t *testing.T) {
	t.Parallel()

	tests := []struct {
		got  string
		want string
	}{
		{Keep{}.String(), ""},
		{Keep{Highest: 3}.String(), "kh3"},
		{Keep{Lowest: 2}.String(), "kl2"},
		{Keep{Highest: 3, Lowest: 1}.String(), "kh3kl1"},
		{Reroll{}.String(), ""},
		{Reroll{Equal: 1, Count: 2}.String(), "r=1x2"},
		{Reroll{Lt: 3, Count: 1}.String(), "r<3x1"},
		{Reroll{Gt: 4, Count: 2}.String(), "r>4x2"},
		{Blow{Equal: 6, Count: 3}.String(), "!=6x3"},
		{Blow{Gt: 4, Count: 2}.String(), "!>4x2"},
	}

	for _, tt := range tests {
		if tt.got != tt.want {
			t.Fatalf("got %q, want %q", tt.got, tt.want)
		}
	}
}

func TestString_DieAndGroup(t *testing.T) {
	t.Parallel()

	die := Die{
		Sides:      6,
		Count:      4,
		Keep:       Keep{Highest: 3},
		Reroll:     Reroll{Lt: 3, Count: 1},
		Blow:       Blow{Equal: 6, Count: 2},
		Modifier:   2,
		Multiplier: 3,
	}
	if got, want := die.String(), "4d6kh3r<3x1!=6x2+2*3"; got != want {
		t.Fatalf("Die.String=%q, want %q", got, want)
	}

	group := DicesGroup{Dices: []Die{
		{Sides: 6, Count: 2, Modifier: 5},
		{Sides: 4, Count: 1, Sub: true},
	}}
	if got, want := group.String(), "2d6+5 - 1d4"; got != want {
		t.Fatalf("DicesGroup.String=%q, want %q", got, want)
	}

	if got := (DicesGroup{}).String(); got != "" {
		t.Fatalf("empty group String=%q, want empty", got)
	}
}

func TestString_Results(t *testing.T) {
	t.Parallel()

	result := ThrowResult{
		Dices: []DieResult{
			{Die: Die{Sides: 6, Count: 4, Keep: Keep{Highest: 3}}, Rolls: []uint{3, 6, 6}},
			{Die: Die{Sides: 4, Count: 1, Sub: true}, Rolls: []uint{2}},
		},
		Total: 13,
	}

	if got, want := result.Dices[0].String(), "4d6kh3:[3 6 6]"; got != want {
		t.Fatalf("DieResult.String=%q, want %q", got, want)
	}
	if got, want := result.String(), "4d6kh3:[3 6 6] + -(1d4:[2]) = 13"; got != want {
		t.Fatalf("ThrowResult.String=%q, want %q", got, want)
	}
	if got, want := (ThrowResult{}).String(), "= 0"; got != want {
		t.Fatalf("empty ThrowResult.String=%q, want %q", got, want)
	}
}
