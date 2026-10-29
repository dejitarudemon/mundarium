package dices

import (
	"testing"
)

func TestRollOne_NoRerollOrdinary(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)
	got := repo.rollOne(6, Reroll{}, Blow{})
	if got < 1 || got > 6 {
		t.Fatalf("roll=%d, want in [1,6]", got)
	}
}

func TestRollOne_RerollReplacesNotAdds(t *testing.T) {
	t.Parallel()

	// Sides=1 always yields face 1. Three forced rerolls discard prior faces;
	// the accepted face remains a single 1 (not 1+1+...).
	repo := newTestRepo(t)
	got := repo.rollOne(1, Reroll{Equal: 1, Count: 3}, Blow{})
	if got != 1 {
		t.Fatalf("roll=%d, want 1 (reroll replaces, does not accumulate)", got)
	}
}

func TestRollOne_RerollBudgetExhaustedTerminates(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)
	got := repo.rollOne(1, Reroll{Equal: 1, Count: 3}, Blow{})
	if got != 1 {
		t.Fatalf("roll=%d, want 1", got)
	}
}

func TestRollOne_RerollConditionNotMetNoExtraRolls(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)
	// Face is always 1; Equal=2 never matches, so Count budget is unused.
	got := repo.rollOne(1, Reroll{Equal: 2, Count: 10}, Blow{})
	if got != 1 {
		t.Fatalf("roll=%d, want 1", got)
	}
}

func TestRollDie_RerollBudgetIndependentPerDie(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)
	faces := repo.rollDie(Die{
		Sides:  1,
		Count:  2,
		Reroll: Reroll{Equal: 1, Count: 3},
	})
	assertUintSliceEqual(t, faces, []uint{1, 1})
}

func TestRollOne_NoBlowOrdinary(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)
	got := repo.rollOne(6, Reroll{}, Blow{})
	if got < 1 || got > 6 {
		t.Fatalf("roll=%d, want in [1,6]", got)
	}
}

func TestRollOne_BlowAddsAndCaps(t *testing.T) {
	t.Parallel()

	// Four faces of 1: original + three explosions.
	repo := newTestRepo(t)
	got := repo.rollOne(1, Reroll{}, Blow{Equal: 1, Count: 3})
	if got != 4 {
		t.Fatalf("roll=%d, want 4", got)
	}
}

func TestRollOne_BlowAlwaysMatchingTerminates(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)
	got := repo.rollOne(1, Reroll{}, Blow{Equal: 1, Count: 5})
	if got != 6 {
		t.Fatalf("roll=%d, want 6", got)
	}
}

func TestRollDie_BlowBudgetIndependentPerDie(t *testing.T) {
	t.Parallel()

	repo := newTestRepo(t)
	faces := repo.rollDie(Die{
		Sides: 1,
		Count: 2,
		Blow:  Blow{Equal: 1, Count: 3},
	})
	assertUintSliceEqual(t, faces, []uint{4, 4})
}

func TestRollOne_RerollPriorityOverBlow(t *testing.T) {
	t.Parallel()

	// Two rerolls discard faces; then two blows accumulate; final accepted face
	// without remaining blow budget → total 3.
	repo := newTestRepo(t)
	got := repo.rollOne(1,
		Reroll{Equal: 1, Count: 2},
		Blow{Equal: 1, Count: 2},
	)
	if got != 3 {
		t.Fatalf("roll=%d, want 3", got)
	}
}
