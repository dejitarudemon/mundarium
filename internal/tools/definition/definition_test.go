package definition_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/dejitarudemon/mundarium/internal/tools/definition"
)

func TestDefinition_Copy(t *testing.T) {
	src := definition.Definition{
		Name:        "create_location",
		Description: "create",
		Parameters:  json.RawMessage(`{"type":"object"}`),
	}

	got := src.Copy()
	if got.Name != src.Name || got.Description != src.Description || !bytes.Equal(got.Parameters, src.Parameters) {
		t.Fatalf("copy mismatch: %#v", got)
	}

	got.Name = "other"
	got.Parameters[0] = 'X'
	if src.Name != "create_location" || src.Parameters[0] != '{' {
		t.Fatalf("Copy must isolate mutable fields, src=%#v", src)
	}
}
