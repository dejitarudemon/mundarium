package events_test

import (
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestExecutor_EventCausal_GraphPatterns(t *testing.T) {
	resetDB(t)

	mk := func(summary string, ticks int) string {
		id := createEventViaTool(t, map[string]any{
			"importance": 40, "created_at_game_time": ticks, "summary": summary,
		})
		return eventIDString(id)
	}

	link := func(from, to, cause string) {
		t.Helper()
		mustExecute(t, "create_event_causal", map[string]any{
			"from": from, "to": to, "cause": cause,
		})
	}

	t.Run("chain_A_B_C", func(t *testing.T) {
		resetDB(t)
		a, b, c := mk("A", 1), mk("B", 2), mk("C", 3)
		link(a, b, "A causes B")
		link(b, c, "B causes C")

		fromA := findCausals(t, map[string]any{"from": a})
		if len(fromA) != 1 || fromA[0]["to"] != b {
			t.Fatalf("from A: %#v", fromA)
		}
		toC := findCausals(t, map[string]any{"to": c})
		if len(toC) != 1 || toC[0]["from"] != b {
			t.Fatalf("to C: %#v", toC)
		}
		byB := findCausals(t, map[string]any{"event_id": b})
		if len(byB) != 2 {
			t.Fatalf("event_id B should see both ends, got %#v", byB)
		}
		if countCausals(t) != 2 {
			t.Fatalf("want 2 edges in DB, got %d", countCausals(t))
		}
	})

	t.Run("multiple_causes_to_C", func(t *testing.T) {
		resetDB(t)
		a, b, c := mk("A", 1), mk("B", 2), mk("C", 3)
		link(a, c, "A→C")
		link(b, c, "B→C")
		toC := findCausals(t, map[string]any{"to": c})
		if len(toC) != 2 {
			t.Fatalf("want 2 causes of C, got %#v", toC)
		}
	})

	t.Run("cycle_A_B_C_A", func(t *testing.T) {
		resetDB(t)
		a, b, c := mk("A", 1), mk("B", 2), mk("C", 3)
		link(a, b, "A→B")
		link(b, c, "B→C")
		link(c, a, "C→A")
		if countCausals(t) != 3 {
			t.Fatalf("want cycle of 3, got %d", countCausals(t))
		}
	})

	t.Run("self_link_rejected", func(t *testing.T) {
		resetDB(t)
		a := mk("A", 1)
		before := countCausals(t)
		invokeID, callID := "inv-self", "call-self"
		raw, err := executeNamed(t, invokeID, callID, "create_event_causal", map[string]any{
			"from": a, "to": a, "cause": "self",
		})
		if err == nil {
			t.Fatalf("self-link must fail, raw=%s", raw)
		}
		if countCausals(t) != before {
			t.Fatalf("self-link must not persist, count=%d", countCausals(t))
		}
		assertJournalFailed(t, invokeID, callID, "create_event_causal")
	})

	t.Run("reverse_time_order_allowed", func(t *testing.T) {
		resetDB(t)
		// Cause later in game time than effect.
		effect := mk("effect-early", 10)
		cause := mk("cause-later", 500)
		link(cause, effect, "future cause of past effect")
		edges := findCausals(t, map[string]any{"from": cause, "to": effect})
		if len(edges) != 1 {
			t.Fatalf("want reverse-time edge, got %#v", edges)
		}
	})

	t.Run("duplicate_update_find_missing", func(t *testing.T) {
		resetDB(t)
		a, b := mk("A", 1), mk("B", 2)
		link(a, b, "first")

		raw, err := execute(t, "create_event_causal", map[string]any{
			"from": a, "to": b, "cause": "dup",
		})
		if err == nil {
			t.Fatalf("duplicate causal must fail, raw=%s", raw)
		}
		if countCausals(t) != 1 {
			t.Fatalf("dup must not add row, count=%d", countCausals(t))
		}

		mustExecute(t, "update_event_causal_cause", map[string]any{
			"from": a, "to": b, "cause": "updated reason",
		})
		edges := findCausals(t, map[string]any{"from": a, "to": b})
		if len(edges) != 1 || edges[0]["cause"] != "updated reason" {
			t.Fatalf("updated cause: %#v", edges)
		}

		missing := uuid.New().String()
		raw, err = execute(t, "create_event_causal", map[string]any{
			"from": missing, "to": b, "cause": "ghost",
		})
		if err == nil {
			t.Fatalf("missing from must fail, raw=%s", raw)
		}
		errText := strings.ToLower(err.Error())
		if !strings.Contains(errText, "foreign") &&
			!strings.Contains(errText, "violat") &&
			!strings.Contains(errText, "not found") &&
			!strings.Contains(errText, "constraint") &&
			!strings.Contains(errText, "event") {
			t.Fatalf("want referential error, got %v", err)
		}
		if countCausals(t) != 1 {
			t.Fatalf("missing endpoint must not leave orphan edge, count=%d", countCausals(t))
		}
	})
}
