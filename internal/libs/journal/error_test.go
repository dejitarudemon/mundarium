package journal_test

import (
	"errors"
	"testing"

	"github.com/dejitarudemon/mundarium/internal/libs/journal"
)

func TestErrorText_RoundTrip(t *testing.T) {
	if got := journal.ErrorTextFrom(nil); got != "" || got.Err() != nil {
		t.Fatalf("nil: want empty, got %q err=%v", got, got.Err())
	}

	src := journal.ErrorTextFrom(errors.New("boom"))
	if src.Error() != "boom" {
		t.Fatalf("Error: want boom, got %q", src.Error())
	}

	val, err := src.Value()
	if err != nil || val != "boom" {
		t.Fatalf("Value: want boom, got %v %v", val, err)
	}

	var loaded journal.ErrorText
	if err := loaded.Scan([]byte("boom")); err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if loaded.Err() == nil || loaded.Error() != "boom" {
		t.Fatalf("Scan result: %q", loaded)
	}

	var empty journal.ErrorText
	if err := empty.Scan(nil); err != nil || empty.Err() != nil {
		t.Fatalf("Scan nil: got %q err=%v", empty, err)
	}
}
