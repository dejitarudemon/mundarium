package dices

import (
	"math/rand"
	"reflect"
	"testing"
)

func newTestRepo(t *testing.T) *DicesRepository {
	t.Helper()
	return NewDicesRepository(nil, nil)
}

func newSeededRepo(t *testing.T, seed int64) *DicesRepository {
	t.Helper()
	return NewDicesRepository(rand.New(rand.NewSource(seed)), nil)
}

func sumFaces(faces []uint) int {
	sum := 0
	for _, face := range faces {
		sum += int(face)
	}
	return sum
}

func assertFacesInRange(t *testing.T, faces []uint, min, max uint) {
	t.Helper()
	for i, face := range faces {
		if face < min || face > max {
			t.Fatalf("face[%d]=%d: want in [%d, %d]", i, face, min, max)
		}
	}
}

func assertTotalMatchesContributions(t *testing.T, result ThrowResult) {
	t.Helper()
	want := 0
	for _, group := range result.Dices {
		want += contribution(group.Die, group.Rolls)
	}
	if result.Total != want {
		t.Fatalf("Total=%d, want sum of contributions %d (groups=%+v)", result.Total, want, result.Dices)
	}
}

func assertUintSliceEqual(t *testing.T, got, want []uint) {
	t.Helper()
	if len(got) == 0 && len(want) == 0 {
		return
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}
