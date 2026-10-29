package dices

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestThrowResult_StructureAndJSON(t *testing.T) {
	t.Parallel()

	repo := newSeededRepo(t, 42)
	first := Die{Sides: 6, Count: 2, Modifier: 1}
	second := Die{Sides: 6, Count: 2, Modifier: 1} // identical params
	third := Die{Sides: 8, Count: 1, Multiplier: 2}
	result := repo.Roll(DicesGroup{Dices: []Die{first, second, third}})

	if len(result.Dices) != 3 {
		t.Fatalf("groups=%d, want 3", len(result.Dices))
	}
	if !reflect.DeepEqual(result.Dices[0].Die, first) {
		t.Fatalf("first die mismatch: %+v", result.Dices[0].Die)
	}
	if !reflect.DeepEqual(result.Dices[1].Die, second) {
		t.Fatalf("second die mismatch: %+v", result.Dices[1].Die)
	}
	if !reflect.DeepEqual(result.Dices[2].Die, third) {
		t.Fatalf("third die mismatch: %+v", result.Dices[2].Die)
	}
	if len(result.Dices[0].Rolls) != 2 || len(result.Dices[1].Rolls) != 2 || len(result.Dices[2].Rolls) != 1 {
		t.Fatalf("roll lengths overwritten: %+v", result.Dices)
	}
	assertTotalMatchesContributions(t, result)

	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded ThrowResult
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !reflect.DeepEqual(decoded, result) {
		t.Fatalf("round-trip mismatch:\n got %+v\nwant %+v", decoded, result)
	}
}
