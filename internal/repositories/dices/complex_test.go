package dices

import (
	"testing"
)

// Complex seeded scenarios exercise interactions that are hard to hit with
// Sides=1 alone: conditional reroll/blow on multi-face dice, keep over exploded
// totals, independent per-die budgets, and multi-group signed arithmetic.

func TestComplex_RerollLtConsumesBudgetThenKeepsLowFace(t *testing.T) {
	t.Parallel()

	// Stream d6@42: 6,6,3,1,2,...
	// Fourth die rolls 1 → reroll once → accepts 2 even though 2 < 3 (budget gone).
	result := newSeededRepo(t, deterministicSeed).Roll(DicesGroup{Dices: []Die{{
		Sides:  6,
		Count:  4,
		Reroll: Reroll{Lt: 3, Count: 1},
	}}})

	assertUintSliceEqual(t, result.Dices[0].Rolls, []uint{6, 6, 3, 2})
	if result.Total != 17 {
		t.Fatalf("Total=%d, want 17", result.Total)
	}
}

func TestComplex_RerollEqualOnLargeSides(t *testing.T) {
	t.Parallel()

	result := newSeededRepo(t, deterministicSeed).Roll(DicesGroup{Dices: []Die{{
		Sides:  6,
		Count:  4,
		Reroll: Reroll{Equal: 1, Count: 2},
	}}})

	assertUintSliceEqual(t, result.Dices[0].Rolls, []uint{6, 6, 3, 2})
	if result.Total != 17 {
		t.Fatalf("Total=%d, want 17", result.Total)
	}
}

func TestComplex_RerollGtDiscardsHighFaces(t *testing.T) {
	t.Parallel()

	// Reroll faces > 4 up to twice, then accept whatever follows.
	// Stream: 6,6 → both discarded → 3 accepted; then 1,2,2.
	result := newSeededRepo(t, deterministicSeed).Roll(DicesGroup{Dices: []Die{{
		Sides:  6,
		Count:  4,
		Reroll: Reroll{Gt: 4, Count: 2},
	}}})

	assertUintSliceEqual(t, result.Dices[0].Rolls, []uint{3, 1, 2, 2})
	if result.Total != 8 {
		t.Fatalf("Total=%d, want 8", result.Total)
	}
}

func TestComplex_BlowGtChainsOnHighFaces(t *testing.T) {
	t.Parallel()

	// Blow while face > 4: first die 6+6+3=15, then 1, 2.
	result := newSeededRepo(t, deterministicSeed).Roll(DicesGroup{Dices: []Die{{
		Sides: 6,
		Count: 3,
		Blow:  Blow{Gt: 4, Count: 2},
	}}})

	assertUintSliceEqual(t, result.Dices[0].Rolls, []uint{15, 1, 2})
	if result.Total != 18 {
		t.Fatalf("Total=%d, want 18", result.Total)
	}
}

func TestComplex_RerollBudgetIndependentWhenEarlyDiceExhaust(t *testing.T) {
	t.Parallel()

	// seed 99 stream: 6,2,1,1,3,...
	// die2 spends both rerolls on 2 then 1, and must still keep the final 1.
	// die3 gets a fresh budget and accepts 3 without needing it.
	result := newSeededRepo(t, 99).Roll(DicesGroup{Dices: []Die{{
		Sides:  6,
		Count:  3,
		Reroll: Reroll{Lt: 3, Count: 2},
	}}})

	assertUintSliceEqual(t, result.Dices[0].Rolls, []uint{6, 1, 3})
	if result.Total != 10 {
		t.Fatalf("Total=%d, want 10", result.Total)
	}
}

func TestComplex_RerollBlowKeepModifierMultiplier(t *testing.T) {
	t.Parallel()

	// Per-die: reroll <3 once, explode on 6 up to twice.
	// Faces before keep: [15, 2, 4, 3]; keep highest 2 → [4, 15]
	// contribution: 2 * (4+15 + 3) = 44
	result := newSeededRepo(t, deterministicSeed).Roll(DicesGroup{Dices: []Die{{
		Sides:      6,
		Count:      4,
		Reroll:     Reroll{Lt: 3, Count: 1},
		Blow:       Blow{Equal: 6, Count: 2},
		Keep:       Keep{Highest: 2},
		Modifier:   3,
		Multiplier: 2,
	}}})

	assertUintSliceEqual(t, result.Dices[0].Rolls, []uint{4, 15})
	if result.Total != 44 {
		t.Fatalf("Total=%d, want 44", result.Total)
	}
	assertTotalMatchesContributions(t, result)
}

func TestComplex_KeepSelectsAmongExplodedTotals(t *testing.T) {
	t.Parallel()

	result := newSeededRepo(t, deterministicSeed).Roll(DicesGroup{Dices: []Die{{
		Sides: 6,
		Count: 3,
		Blow:  Blow{Equal: 6, Count: 3},
		Keep:  Keep{Highest: 1},
	}}})

	// Exploded faces [15, 1, 2]; keep highest → [15]
	assertUintSliceEqual(t, result.Dices[0].Rolls, []uint{15})
	if result.Total != 15 {
		t.Fatalf("Total=%d, want 15", result.Total)
	}
}

func TestComplex_SubKeepLowestWithModifier(t *testing.T) {
	t.Parallel()

	// Raw [6,6,3], keep lowest 2 → [3,6], contribution -(3+6+1) = -10
	result := newSeededRepo(t, deterministicSeed).Roll(DicesGroup{Dices: []Die{{
		Sides:    6,
		Count:    3,
		Keep:     Keep{Lowest: 2},
		Sub:      true,
		Modifier: 1,
	}}})

	assertUintSliceEqual(t, result.Dices[0].Rolls, []uint{3, 6})
	if result.Total != -10 {
		t.Fatalf("Total=%d, want -10", result.Total)
	}
}

func TestComplex_MultiGroupMixedMechanics(t *testing.T) {
	t.Parallel()

	attack := Die{Sides: 6, Count: 4, Keep: Keep{Highest: 3}, Modifier: 2}
	explode := Die{Sides: 8, Count: 1, Blow: Blow{Equal: 8, Count: 2}}
	penalty := Die{Sides: 4, Count: 2, Keep: Keep{Lowest: 1}, Sub: true}

	result := newSeededRepo(t, deterministicSeed).Roll(DicesGroup{
		Dices: []Die{attack, explode, penalty},
	})

	if len(result.Dices) != 3 {
		t.Fatalf("groups=%d, want 3", len(result.Dices))
	}
	assertUintSliceEqual(t, result.Dices[0].Rolls, []uint{3, 6, 6})
	assertUintSliceEqual(t, result.Dices[1].Rolls, []uint{10})
	assertUintSliceEqual(t, result.Dices[2].Rolls, []uint{1})

	want := contribution(attack, result.Dices[0].Rolls) +
		contribution(explode, result.Dices[1].Rolls) +
		contribution(penalty, result.Dices[2].Rolls)
	if result.Total != want || result.Total != 26 {
		t.Fatalf("Total=%d, want 26 (contrib sum %d)", result.Total, want)
	}
}

func TestComplex_RerollPriorityStillHoldsOnLargeSides(t *testing.T) {
	t.Parallel()

	// Same condition Equal=6 for both: faces are discarded by reroll first.
	// Stream starts 6,6,3 → two rerolls burn the sixes, accept 3 (no blow left unused on 3).
	got := newSeededRepo(t, deterministicSeed).rollOne(
		6,
		Reroll{Equal: 6, Count: 2},
		Blow{Equal: 6, Count: 2},
	)
	if got != 3 {
		t.Fatalf("rollOne=%d, want 3", got)
	}
}

func TestComplex_ApplyKeepHighestThenLowest(t *testing.T) {
	t.Parallel()

	// Documented invalid combo, but current behavior: keep 4 highest of
	// [1,3,5,7,9] → [3,5,7,9], then keep 2 lowest → [3,5].
	got := applyKeep([]uint{9, 1, 5, 3, 7}, Keep{Highest: 4, Lowest: 2})
	assertUintSliceEqual(t, got, []uint{3, 5})
}

func TestComplex_NegativeMultiplierWithSubAndKeep(t *testing.T) {
	t.Parallel()

	result := newSeededRepo(t, deterministicSeed).Roll(DicesGroup{Dices: []Die{{
		Sides:      6,
		Count:      4,
		Keep:       Keep{Highest: 2},
		Multiplier: -3,
		Sub:        true,
	}}})

	// Raw [6,6,3,1] → keep hi2 [6,6]; value = -3*(12+0) = -36; Sub → +36
	assertUintSliceEqual(t, result.Dices[0].Rolls, []uint{6, 6})
	if result.Total != 36 {
		t.Fatalf("Total=%d, want 36", result.Total)
	}
}

func TestComplex_BlowLtExplodesLowFaces(t *testing.T) {
	t.Parallel()

	// Blow while face < 3. Stream 6,6,3,1,2,2,4,...
	// die1: 6 no; die2: 6 no; die3: 3 no; die4 needs separate run with count 1 after consuming...
	// Use Count=1 starting mid-logic via rollOne after seeding fresh and checking die that hits lows.
	// Full 4 dice:
	// d1:6 →6; d2:6 →6; d3:3 →3; d4:1 blow, 2 blow (budget 2→0), 2 stop → 1+2+2=5
	result := newSeededRepo(t, deterministicSeed).Roll(DicesGroup{Dices: []Die{{
		Sides: 6,
		Count: 4,
		Blow:  Blow{Lt: 3, Count: 2},
	}}})

	assertUintSliceEqual(t, result.Dices[0].Rolls, []uint{6, 6, 3, 5})
	if result.Total != 20 {
		t.Fatalf("Total=%d, want 20", result.Total)
	}
}
