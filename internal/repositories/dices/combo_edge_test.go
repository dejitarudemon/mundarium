package dices

import (
	"reflect"
	"testing"
)

func TestRoll_KeepHighest3of4d6(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)
	die := Die{Sides: 6, Count: 4, Keep: Keep{Highest: 3}}
	result := repo.Roll(DicesGroup{Dices: []Die{die}})

	if len(result.Dices) != 1 || len(result.Dices[0].Rolls) != 3 {
		t.Fatalf("got %+v, want 3 kept rolls", result.Dices)
	}
	assertFacesInRange(t, result.Dices[0].Rolls, 1, 6)
	assertTotalMatchesContributions(t, result)
}

func TestRoll_KeepLowestWithModifier(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)
	die := Die{Sides: 6, Count: 4, Keep: Keep{Lowest: 2}, Modifier: 5}
	result := repo.Roll(DicesGroup{Dices: []Die{die}})

	if len(result.Dices[0].Rolls) != 2 {
		t.Fatalf("kept=%d, want 2", len(result.Dices[0].Rolls))
	}
	want := sumFaces(result.Dices[0].Rolls) + 5
	if result.Total != want {
		t.Fatalf("Total=%d, want %d", result.Total, want)
	}
}

func TestRoll_BlowThenMultiplier(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)
	die := Die{
		Sides:      1,
		Count:      2,
		Blow:       Blow{Equal: 1, Count: 3},
		Multiplier: 2,
	}
	result := repo.Roll(DicesGroup{Dices: []Die{die}})

	assertUintSliceEqual(t, result.Dices[0].Rolls, []uint{4, 4})
	if result.Total != 16 {
		t.Fatalf("Total=%d, want 16", result.Total)
	}
}

func TestRoll_RerollThenKeepHighest(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)
	die := Die{
		Sides:  1,
		Count:  3,
		Reroll: Reroll{Equal: 1, Count: 2},
		Keep:   Keep{Highest: 2},
	}
	result := repo.Roll(DicesGroup{Dices: []Die{die}})

	if len(result.Dices[0].Rolls) != 2 {
		t.Fatalf("kept=%d, want 2", len(result.Dices[0].Rolls))
	}
	assertUintSliceEqual(t, result.Dices[0].Rolls, []uint{1, 1})
	if result.Total != 2 {
		t.Fatalf("Total=%d, want 2", result.Total)
	}
}

func TestRoll_IndependentGroupSettings(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)
	a := Die{Sides: 1, Count: 2, Modifier: 1}
	b := Die{Sides: 1, Count: 3, Keep: Keep{Highest: 1}, Multiplier: 2}
	result := repo.Roll(DicesGroup{Dices: []Die{a, b}})

	if !reflect.DeepEqual(result.Dices[0].Die, a) || !reflect.DeepEqual(result.Dices[1].Die, b) {
		t.Fatalf("group settings mixed: %+v", result.Dices)
	}
	if len(result.Dices[0].Rolls) != 2 || len(result.Dices[1].Rolls) != 1 {
		t.Fatalf("unexpected rolls: %+v", result.Dices)
	}
	want := contribution(a, result.Dices[0].Rolls) + contribution(b, result.Dices[1].Rolls)
	if result.Total != want {
		t.Fatalf("Total=%d, want %d", result.Total, want)
	}
}

func TestRoll_SubGroupWithKeep(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)
	positive := Die{Sides: 1, Count: 2}
	negative := Die{Sides: 1, Count: 3, Keep: Keep{Highest: 2}, Sub: true}
	result := repo.Roll(DicesGroup{Dices: []Die{positive, negative}})

	assertUintSliceEqual(t, result.Dices[0].Rolls, []uint{1, 1})
	assertUintSliceEqual(t, result.Dices[1].Rolls, []uint{1, 1})
	if result.Total != 0 {
		t.Fatalf("Total=%d, want 0 (2 - 2)", result.Total)
	}
}

func TestRoll_RerollAndBlowSameCondition(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)
	die := Die{
		Sides:  1,
		Count:  1,
		Reroll: Reroll{Equal: 1, Count: 2},
		Blow:   Blow{Equal: 1, Count: 2},
	}
	result := repo.Roll(DicesGroup{Dices: []Die{die}})

	assertUintSliceEqual(t, result.Dices[0].Rolls, []uint{3})
	if result.Total != 3 {
		t.Fatalf("Total=%d, want 3", result.Total)
	}
}

func TestRoll_EdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		die   Die
		check func(t *testing.T, result ThrowResult)
	}{
		{
			name: "count 0",
			die:  Die{Sides: 6, Count: 0},
			check: func(t *testing.T, result ThrowResult) {
				if len(result.Dices[0].Rolls) != 0 || result.Total != 0 {
					t.Fatalf("got %+v", result)
				}
			},
		},
		{
			name: "sides 0 yields zero faces",
			die:  Die{Sides: 0, Count: 3},
			check: func(t *testing.T, result ThrowResult) {
				assertUintSliceEqual(t, result.Dices[0].Rolls, []uint{0, 0, 0})
				if result.Total != 0 {
					t.Fatalf("Total=%d, want 0", result.Total)
				}
			},
		},
		{
			name: "sides 1",
			die:  Die{Sides: 1, Count: 5},
			check: func(t *testing.T, result ThrowResult) {
				assertUintSliceEqual(t, result.Dices[0].Rolls, []uint{1, 1, 1, 1, 1})
				if result.Total != 5 {
					t.Fatalf("Total=%d, want 5", result.Total)
				}
			},
		},
		{
			name: "sides 255",
			die:  Die{Sides: 255, Count: 4},
			check: func(t *testing.T, result ThrowResult) {
				if len(result.Dices[0].Rolls) != 4 {
					t.Fatalf("rolls=%d, want 4", len(result.Dices[0].Rolls))
				}
				assertFacesInRange(t, result.Dices[0].Rolls, 1, 255)
				assertTotalMatchesContributions(t, result)
			},
		},
		{
			name: "count 255",
			die:  Die{Sides: 1, Count: 255},
			check: func(t *testing.T, result ThrowResult) {
				if len(result.Dices[0].Rolls) != 255 {
					t.Fatalf("rolls=%d, want 255", len(result.Dices[0].Rolls))
				}
				if result.Total != 255 {
					t.Fatalf("Total=%d, want 255", result.Total)
				}
			},
		},
		{
			name: "keep highest greater than count",
			die:  Die{Sides: 1, Count: 3, Keep: Keep{Highest: 10}},
			check: func(t *testing.T, result ThrowResult) {
				assertUintSliceEqual(t, result.Dices[0].Rolls, []uint{1, 1, 1})
			},
		},
		{
			name: "keep lowest greater than count",
			die:  Die{Sides: 1, Count: 3, Keep: Keep{Lowest: 10}},
			check: func(t *testing.T, result ThrowResult) {
				assertUintSliceEqual(t, result.Dices[0].Rolls, []uint{1, 1, 1})
			},
		},
		{
			name: "zero reroll and blow params",
			die:  Die{Sides: 6, Count: 2, Reroll: Reroll{}, Blow: Blow{}},
			check: func(t *testing.T, result ThrowResult) {
				assertFacesInRange(t, result.Dices[0].Rolls, 1, 6)
				assertTotalMatchesContributions(t, result)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repo := newTestRepo(t)
			result := repo.Roll(DicesGroup{Dices: []Die{tt.die}})
			if len(result.Dices) != 1 {
				t.Fatalf("groups=%d, want 1", len(result.Dices))
			}
			tt.check(t, result)
		})
	}
}

func TestRoll_EmptyDicesList(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)
	result := repo.Roll(DicesGroup{Dices: []Die{}})
	if len(result.Dices) != 0 || result.Total != 0 {
		t.Fatalf("got %+v", result)
	}
}
