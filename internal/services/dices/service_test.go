package dices_test

import (
	"math/rand"
	"reflect"
	"testing"

	repodices "github.com/dejitarudemon/mundarium/internal/repositories/dices"
	servicedices "github.com/dejitarudemon/mundarium/internal/services/dices"
)

func TestNewService_NilRepository(t *testing.T) {
	t.Parallel()

	if _, err := servicedices.NewService(nil, nil); err == nil {
		t.Fatal("want error for nil repository")
	}
}

func TestService_Roll(t *testing.T) {
	t.Parallel()

	const seed int64 = 42
	throw := repodices.DicesGroup{Dices: []repodices.Die{
		{Sides: 6, Count: 4},
		{Sides: 20, Count: 1, Modifier: 2},
	}}

	svc, err := servicedices.NewService(
		repodices.NewDicesRepository(rand.New(rand.NewSource(seed)), nil),
		nil,
	)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}

	got := svc.Roll(throw)
	want := repodices.NewDicesRepository(rand.New(rand.NewSource(seed)), nil).Roll(throw)

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Roll mismatch:\n got %+v\nwant %+v", got, want)
	}
}
