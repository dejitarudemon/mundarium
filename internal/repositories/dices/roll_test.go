package dices

import (
	"reflect"
	"testing"
)

func TestRoll_1d20(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)
	result := repo.Roll(DicesGroup{Dices: []Die{{Sides: 20, Count: 1}}})

	if len(result.Dices) != 1 {
		t.Fatalf("groups=%d, want 1", len(result.Dices))
	}
	if len(result.Dices[0].Rolls) != 1 {
		t.Fatalf("rolls=%d, want 1", len(result.Dices[0].Rolls))
	}
	assertFacesInRange(t, result.Dices[0].Rolls, 1, 20)
	if result.Total != int(result.Dices[0].Rolls[0]) {
		t.Fatalf("Total=%d, want %d", result.Total, result.Dices[0].Rolls[0])
	}
	assertTotalMatchesContributions(t, result)
}

func TestRoll_4d6(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)
	result := repo.Roll(DicesGroup{Dices: []Die{{Sides: 6, Count: 4}}})

	if len(result.Dices) != 1 || len(result.Dices[0].Rolls) != 4 {
		t.Fatalf("got %+v, want 1 group with 4 rolls", result.Dices)
	}
	assertFacesInRange(t, result.Dices[0].Rolls, 1, 6)
	assertTotalMatchesContributions(t, result)
}

func TestRoll_MultipleGroupsPreserveOrder(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)
	d8 := Die{Sides: 8, Count: 1}
	d6 := Die{Sides: 6, Count: 3}
	result := repo.Roll(DicesGroup{Dices: []Die{d8, d6}})

	if len(result.Dices) != 2 {
		t.Fatalf("groups=%d, want 2", len(result.Dices))
	}
	if !reflect.DeepEqual(result.Dices[0].Die, d8) {
		t.Fatalf("first die = %+v, want %+v", result.Dices[0].Die, d8)
	}
	if !reflect.DeepEqual(result.Dices[1].Die, d6) {
		t.Fatalf("second die = %+v, want %+v", result.Dices[1].Die, d6)
	}
	if len(result.Dices[0].Rolls) != 1 || len(result.Dices[1].Rolls) != 3 {
		t.Fatalf("roll counts = %d/%d, want 1/3", len(result.Dices[0].Rolls), len(result.Dices[1].Rolls))
	}
	assertFacesInRange(t, result.Dices[0].Rolls, 1, 8)
	assertFacesInRange(t, result.Dices[1].Rolls, 1, 6)
	assertTotalMatchesContributions(t, result)
}

func TestRoll_ModifierAndMultiplier(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		die  Die
	}{
		{name: "2d6+5", die: Die{Sides: 6, Count: 2, Modifier: 5}},
		{name: "2d6-3", die: Die{Sides: 6, Count: 2, Modifier: -3}},
		{name: "2d6*3", die: Die{Sides: 6, Count: 2, Multiplier: 3}},
		{name: "(2d6+4)*2", die: Die{Sides: 6, Count: 2, Modifier: 4, Multiplier: 2}},
		{name: "multiplier 0 means 1", die: Die{Sides: 6, Count: 2, Multiplier: 0}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repo := newTestRepo(t)
			result := repo.Roll(DicesGroup{Dices: []Die{tt.die}})

			if len(result.Dices) != 1 || len(result.Dices[0].Rolls) != int(tt.die.Count) {
				t.Fatalf("unexpected result: %+v", result)
			}
			assertFacesInRange(t, result.Dices[0].Rolls, 1, uint(tt.die.Sides))

			want := contribution(tt.die, result.Dices[0].Rolls)
			if result.Total != want {
				t.Fatalf("Total=%d, want %d", result.Total, want)
			}
		})
	}
}

func TestRoll_SubtractGroup(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)
	positive := Die{Sides: 6, Count: 2}
	negative := Die{Sides: 4, Count: 1, Sub: true}
	result := repo.Roll(DicesGroup{Dices: []Die{positive, negative}})

	if len(result.Dices) != 2 {
		t.Fatalf("groups=%d, want 2", len(result.Dices))
	}
	assertFacesInRange(t, result.Dices[0].Rolls, 1, 6)
	assertFacesInRange(t, result.Dices[1].Rolls, 1, 4)

	want := contribution(positive, result.Dices[0].Rolls) + contribution(negative, result.Dices[1].Rolls)
	if result.Total != want {
		t.Fatalf("Total=%d, want %d", result.Total, want)
	}
	if contribution(negative, result.Dices[1].Rolls) >= 0 {
		t.Fatalf("sub group contribution must be negative, got %d", contribution(negative, result.Dices[1].Rolls))
	}
}

func TestRoll_IdenticalGroupsPreserved(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)
	die := Die{Sides: 6, Count: 2, Modifier: 1}
	result := repo.Roll(DicesGroup{Dices: []Die{die, die}})

	if len(result.Dices) != 2 {
		t.Fatalf("groups=%d, want 2", len(result.Dices))
	}
	if !reflect.DeepEqual(result.Dices[0].Die, die) || !reflect.DeepEqual(result.Dices[1].Die, die) {
		t.Fatalf("die params rewritten: %+v", result.Dices)
	}
	if len(result.Dices[0].Rolls) != 2 || len(result.Dices[1].Rolls) != 2 {
		t.Fatalf("roll counts overwritten: %+v", result.Dices)
	}
	assertTotalMatchesContributions(t, result)
}

func TestRoll_EmptyGroup(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)
	result := repo.Roll(DicesGroup{})

	if len(result.Dices) != 0 {
		t.Fatalf("groups=%d, want 0", len(result.Dices))
	}
	if result.Total != 0 {
		t.Fatalf("Total=%d, want 0", result.Total)
	}
}
