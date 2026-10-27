package events_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/dejitarudemon/mundarium/internal/repositories/events"
	"github.com/google/uuid"
)

func TestID_JSONRoundTrip(t *testing.T) {
	id := events.New()

	raw, err := json.Marshal(id)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if bytes.HasPrefix(raw, []byte("[")) {
		t.Fatalf("ID must marshal as a UUID string, got %s", raw)
	}
	wantQuoted := `"` + id.String() + `"`
	if string(raw) != wantQuoted {
		t.Fatalf("Marshal: want %s, got %s", wantQuoted, raw)
	}

	var got events.ID
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got != id {
		t.Fatalf("round-trip: want %s, got %s", id, got)
	}
}

func TestID_UnmarshalJSON_Invalid(t *testing.T) {
	cases := []string{
		`"not-a-uuid"`,
		`""`,
		`123`,
		`null`,
		`["a","b"]`,
	}
	for _, raw := range cases {
		var id events.ID
		err := json.Unmarshal([]byte(raw), &id)
		if err == nil {
			t.Fatalf("Unmarshal(%s): want error", raw)
		}
		if !strings.Contains(err.Error(), "events.ID") && raw != `null` && raw != `123` && !strings.HasPrefix(raw, `[`) {
			// Parse errors are wrapped; type errors from json.Unmarshal may not include the prefix.
			t.Logf("Unmarshal(%s): %v", raw, err)
		}
	}
}

func TestID_UnmarshalJSON_ValidString(t *testing.T) {
	u := uuid.MustParse("550e8400-e29b-41d4-a716-446655440000")
	raw := []byte(`"550e8400-e29b-41d4-a716-446655440000"`)

	var id events.ID
	if err := json.Unmarshal(raw, &id); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if uuid.UUID(id) != u {
		t.Fatalf("want %s, got %s", u, id)
	}
}
