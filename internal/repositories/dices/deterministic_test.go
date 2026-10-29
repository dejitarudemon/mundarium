package dices

import (
	"reflect"
	"testing"
)

const deterministicSeed int64 = 42

func TestDeterminism_SameSeedSameResult(t *testing.T) {
	t.Parallel()

	throw := DicesGroup{Dices: []Die{
		{Sides: 6, Count: 4},
		{Sides: 20, Count: 3, Modifier: -2},
		{Sides: 100, Count: 2, Multiplier: 2},
	}}

	a := newSeededRepo(t, deterministicSeed).Roll(throw)
	b := newSeededRepo(t, deterministicSeed).Roll(throw)

	if !reflect.DeepEqual(a, b) {
		t.Fatalf("same seed produced different results:\n a=%+v\n b=%+v", a, b)
	}
}

func TestDeterminism_DifferentSeedsDiverge(t *testing.T) {
	t.Parallel()

	throw := DicesGroup{Dices: []Die{{Sides: 20, Count: 8}}}
	a := newSeededRepo(t, deterministicSeed).Roll(throw)
	b := newSeededRepo(t, deterministicSeed+1).Roll(throw)

	if reflect.DeepEqual(a.Dices[0].Rolls, b.Dices[0].Rolls) {
		t.Fatalf("different seeds unexpectedly matched: %v", a.Dices[0].Rolls)
	}
}

func TestDeterminism_PlainLargeSidesGolden(t *testing.T) {
	t.Parallel()

	result := newSeededRepo(t, deterministicSeed).Roll(DicesGroup{Dices: []Die{
		{Sides: 6, Count: 4},
		{Sides: 20, Count: 2},
	}})

	assertUintSliceEqual(t, result.Dices[0].Rolls, []uint{6, 6, 3, 1})
	assertUintSliceEqual(t, result.Dices[1].Rolls, []uint{4, 6})
	if result.Total != 26 {
		t.Fatalf("Total=%d, want 26", result.Total)
	}
}

func TestDeterminism_D100Golden(t *testing.T) {
	t.Parallel()

	result := newSeededRepo(t, deterministicSeed).Roll(DicesGroup{Dices: []Die{
		{Sides: 100, Count: 5},
	}})

	assertUintSliceEqual(t, result.Dices[0].Rolls, []uint{6, 88, 69, 51, 24})
	if result.Total != 238 {
		t.Fatalf("Total=%d, want 238", result.Total)
	}
}

func TestDeterminism_RerollLargeSidesGolden(t *testing.T) {
	t.Parallel()

	// Raw stream for seed 42 / d6 starts as 6,6,3,1,...
	// Lt=3 rerolls 1 and 2 only, so faces 6,6,3 are accepted as-is.
	result := newSeededRepo(t, deterministicSeed).Roll(DicesGroup{Dices: []Die{{
		Sides:  6,
		Count:  3,
		Reroll: Reroll{Lt: 3, Count: 2},
	}}})

	assertUintSliceEqual(t, result.Dices[0].Rolls, []uint{6, 6, 3})
	if result.Total != 15 {
		t.Fatalf("Total=%d, want 15", result.Total)
	}
}

func TestDeterminism_BlowLargeSidesGolden(t *testing.T) {
	t.Parallel()

	// First die: 6+6+3 (two explosions), second: 1.
	result := newSeededRepo(t, deterministicSeed).Roll(DicesGroup{Dices: []Die{{
		Sides: 6,
		Count: 2,
		Blow:  Blow{Equal: 6, Count: 3},
	}}})

	assertUintSliceEqual(t, result.Dices[0].Rolls, []uint{15, 1})
	if result.Total != 16 {
		t.Fatalf("Total=%d, want 16", result.Total)
	}
}

func TestDeterminism_KeepAfterLargeSidesGolden(t *testing.T) {
	t.Parallel()

	result := newSeededRepo(t, deterministicSeed).Roll(DicesGroup{Dices: []Die{{
		Sides:    6,
		Count:    4,
		Keep:     Keep{Highest: 3},
		Modifier: 2,
	}}})

	assertUintSliceEqual(t, result.Dices[0].Rolls, []uint{3, 6, 6})
	if result.Total != 17 {
		t.Fatalf("Total=%d, want 17", result.Total)
	}
}

func TestDeterminism_RerollAndBlowLargeSidesGolden(t *testing.T) {
	t.Parallel()

	// Stream: 6 (reroll miss, blow hit) → 6 (blow) → 3 (stop) = 15.
	got := newSeededRepo(t, deterministicSeed).rollOne(
		6,
		Reroll{Equal: 1, Count: 2},
		Blow{Equal: 6, Count: 2},
	)
	if got != 15 {
		t.Fatalf("rollOne=%d, want 15", got)
	}
}
