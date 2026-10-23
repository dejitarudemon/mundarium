package providers_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/dejitarudemon/mundarium/internal/libs/providers"
)

func TestRequest_Copy(t *testing.T) {
	src := providers.Request{Args: json.RawMessage(`{"q":1}`)}
	got := src.Copy()
	if !bytes.Equal(got.Args, src.Args) {
		t.Fatalf("copy mismatch: %s", got.Args)
	}

	got.Args[0] = 'X'
	if src.Args[0] != '{' {
		t.Fatalf("Copy must isolate Args, src=%s", src.Args)
	}
}
