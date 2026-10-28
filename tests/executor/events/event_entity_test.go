package events_test

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestExecutor_EventEntity_Lifecycle(t *testing.T) {
	resetDB(t)

	eventID := createEventViaTool(t, map[string]any{
		"importance": 50, "created_at_game_time": 10, "summary": "meeting at inn",
	})
	entityID := newEntityID()
	otherEntity := newEntityID()

	mustExecute(t, "create_event_entity", map[string]any{
		"event_id":  eventIDString(eventID),
		"entity_id": entityID.String(),
		"role":      "witness",
		"details":   map[string]any{"seat": "corner"},
	})
	mustExecute(t, "create_event_entity", map[string]any{
		"event_id":  eventIDString(eventID),
		"entity_id": otherEntity.String(),
		"role":      "participant",
	})

	all := findEntities(t, map[string]any{"event_id": eventIDString(eventID)})
	if len(all) != 2 {
		t.Fatalf("want 2 entities, got %#v", all)
	}

	mustExecute(t, "update_event_entity_role", map[string]any{
		"event_id": eventIDString(eventID), "entity_id": entityID.String(), "role": "actor",
	})

	// Belief before known_at must fail.
	raw, err := execute(t, "update_event_entity_belief", map[string]any{
		"event_id": eventIDString(eventID), "entity_id": entityID.String(), "belief": 40,
	})
	if err == nil {
		t.Fatalf("belief before known_at must fail, raw=%s", raw)
	}

	mustExecute(t, "update_event_entity_known_at", map[string]any{
		"event_id":             eventIDString(eventID),
		"entity_id":            entityID.String(),
		"known_at_game_time":   15,
		"belief":               70,
	})

	mustExecute(t, "update_event_entity_belief", map[string]any{
		"event_id": eventIDString(eventID), "entity_id": entityID.String(), "belief": 55,
	})
	mustExecute(t, "update_event_entity_details", map[string]any{
		"event_id":  eventIDString(eventID),
		"entity_id": entityID.String(),
		"details":   map[string]any{"source": "rumor"},
	})

	known := findEntities(t, map[string]any{
		"event_id": eventIDString(eventID), "known_only": true,
	})
	if len(known) != 1 || known[0]["entity_id"] != entityID.String() {
		t.Fatalf("known_only: %#v", known)
	}
	belief, ok := known[0]["belief"].(float64)
	if !ok || int(belief) != 55 {
		t.Fatalf("belief: %#v", known[0]["belief"])
	}
	knownAt, ok := known[0]["known_at_game_time"].(float64)
	if !ok || int(knownAt) != 15 {
		t.Fatalf("known_at: %#v", known[0]["known_at_game_time"])
	}
	if known[0]["role"] != "actor" {
		t.Fatalf("role: %#v", known[0]["role"])
	}

	// Re-assign known_at must fail; known_at stays 15.
	raw, err = execute(t, "update_event_entity_known_at", map[string]any{
		"event_id": eventIDString(eventID), "entity_id": entityID.String(),
		"known_at_game_time": 99, "belief": 10,
	})
	if err == nil {
		t.Fatalf("second known_at must fail, raw=%s", raw)
	}
	errText := strings.ToLower(err.Error())
	if !strings.Contains(errText, "already") && !strings.Contains(errText, "known") {
		t.Fatalf("want already-known style error, got %v", err)
	}

	got := getEventDB(t, eventID)
	var linkFound bool
	for _, link := range got.Entities {
		if link.EntityID != entityID {
			continue
		}
		linkFound = true
		if link.KnownAtGameTime == nil || *link.KnownAtGameTime != 15 {
			t.Fatalf("known_at must remain 15, got %v", link.KnownAtGameTime)
		}
		if link.Belief == nil || *link.Belief != 55 {
			t.Fatalf("belief: %v", link.Belief)
		}
	}
	if !linkFound {
		t.Fatal("entity link missing in DB")
	}

	t.Run("missing_refs", func(t *testing.T) {
		missingEvent := uuid.New().String()
		raw, err := execute(t, "create_event_entity", map[string]any{
			"event_id": missingEvent, "entity_id": newEntityID().String(), "role": "witness",
		})
		if err == nil {
			t.Fatalf("missing event must fail, raw=%s", raw)
		}

		raw, err = execute(t, "update_event_entity_role", map[string]any{
			"event_id": eventIDString(eventID), "entity_id": uuid.New().String(), "role": "actor",
		})
		if err == nil {
			t.Fatalf("missing entity link must fail, raw=%s", raw)
		}
	})
}
