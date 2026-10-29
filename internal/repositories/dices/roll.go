package dices

import (
	"sort"
)

// Roll evaluates a dice group using D&D-style mechanics:
// each die is rolled in [1, sides], may reroll or explode within its own
// budget, then keep filters are applied and the group contribution is totaled.
func (r *DicesRepository) Roll(throw DicesGroup) ThrowResult {
	out := ThrowResult{
		Dices: make([]DieResult, 0, len(throw.Dices)),
	}

	for _, die := range throw.Dices {
		faces := applyKeep(r.rollDie(die), die.Keep)

		out.Dices = append(out.Dices, DieResult{
			Die:   die,
			Rolls: faces,
		})
		out.Total += contribution(die, faces)
	}

	return out
}

// rollDie rolls die.Count independent dice. Reroll and blow budgets are
// restored for every die (they are not shared across the group).
func (r *DicesRepository) rollDie(die Die) []uint {
	faces := make([]uint, 0, die.Count)
	for range die.Count {
		faces = append(faces, r.rollOne(die.Sides, die.Reroll, die.Blow))
	}
	return faces
}

func (r *DicesRepository) rollOne(sides uint8, reroll Reroll, blow Blow) uint {
	if sides == 0 {
		return 0
	}

	var total uint
	rerollsLeft := reroll.Count
	blowsLeft := blow.Count

	for {
		face := uint8(r.randomizer.Intn(int(sides))) + 1

		if faceMatches(face, reroll.Equal, reroll.Lt, reroll.Gt) && rerollsLeft > 0 {
			rerollsLeft--
			continue
		}

		total += uint(face)

		if faceMatches(face, blow.Equal, blow.Lt, blow.Gt) && blowsLeft > 0 {
			blowsLeft--
			continue
		}

		return total
	}
}

func applyKeep(faces []uint, keep Keep) []uint {
	if len(faces) == 0 {
		return faces
	}

	if keep.Highest == 0 && keep.Lowest == 0 {
		return faces
	}

	sort.SliceStable(faces, func(i, j int) bool {
		return faces[i] < faces[j]
	})

	if n := int(keep.Highest); 0 < n && n < len(faces) {
		faces = faces[len(faces)-n:]
	}
	if n := int(keep.Lowest); 0 < n && n < len(faces) {
		faces = faces[:n]
	}

	return faces
}

func contribution(die Die, faces []uint) int {
	sum := 0
	for _, face := range faces {
		sum += int(face)
	}

	mult := die.Multiplier
	if mult == 0 {
		mult = 1
	}

	value := mult * (sum + die.Modifier)
	if die.Sub {
		return -value
	}
	return value
}

func faceMatches(face, equal, lowerThan, greaterThan uint8) bool {
	return (equal != 0 && face == equal) ||
		(greaterThan != 0 && face > greaterThan) ||
		(lowerThan != 0 && face < lowerThan)
}
