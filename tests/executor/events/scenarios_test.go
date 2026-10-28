package events_test

import (
	"testing"

	"github.com/google/uuid"
)

func TestExecutor_ScenarioA_RevengeStory(t *testing.T) {
	resetDB(t)

	baron := newEntityID()
	miller := newEntityID()
	son := newEntityID()
	bandit := newEntityID()

	e1 := createEventViaTool(t, map[string]any{
		"importance": 90, "created_at_game_time": 100,
		"summary": "Baron orders the miller executed",
		"tags":    []string{"revenge", "politics"},
		"details": map[string]any{"victim": "miller"},
		"entities": []map[string]any{
			{"entity_id": baron.String(), "role": "actor"},
			{"entity_id": miller.String(), "role": "affected"},
			{"entity_id": son.String(), "role": "informed"},
		},
	})
	e2 := createEventViaTool(t, map[string]any{
		"importance": 80, "created_at_game_time": 110,
		"summary": "Miller's son swears revenge",
		"tags":    []string{"revenge"},
		"entities": []map[string]any{
			{"entity_id": son.String(), "role": "actor"},
		},
		"causals": []map[string]any{
			{"from": eventIDString(e1), "cause": "execution drove the oath"},
		},
	})
	e3 := createEventViaTool(t, map[string]any{
		"importance": 60, "created_at_game_time": 130,
		"summary": "Son joins a bandit gang",
		"tags":    []string{"revenge", "crime"},
		"entities": []map[string]any{
			{"entity_id": son.String(), "role": "actor"},
			{"entity_id": bandit.String(), "role": "participant"},
		},
		"causals": []map[string]any{
			{"from": eventIDString(e2), "cause": "oath pushed him to outlaws"},
		},
	})
	e4 := createEventViaTool(t, map[string]any{
		"importance": 75, "created_at_game_time": 160,
		"summary": "Bandits raid a caravan",
		"tags":    []string{"crime"},
		"entities": []map[string]any{
			{"entity_id": son.String(), "role": "participant"},
			{"entity_id": bandit.String(), "role": "actor"},
		},
		"causals": []map[string]any{
			{"from": eventIDString(e3), "cause": "gang membership enabled the raid"},
		},
	})

	mustExecute(t, "update_event_entity_known_at", map[string]any{
		"event_id": eventIDString(e1), "entity_id": son.String(),
		"known_at_game_time": 105, "belief": 95,
	})

	byQuery := findEvents(t, map[string]any{"query": "miller"})
	if len(byQuery) < 2 {
		t.Fatalf("want miller matches in summary/details, got %#v", byQuery)
	}
	byTag := findEvents(t, map[string]any{"tags": []string{"revenge"}})
	ids := map[string]bool{}
	for _, ev := range byTag {
		ids[ev["id"].(string)] = true
	}
	for _, want := range []string{eventIDString(e1), eventIDString(e2), eventIDString(e3)} {
		if !ids[want] {
			t.Fatalf("missing revenge-tagged event %s in %#v", want, byTag)
		}
	}

	sonLinks := findEntities(t, map[string]any{"entity_id": son.String()})
	if len(sonLinks) < 3 {
		t.Fatalf("son should participate in oath/gang/raid, got %#v", sonLinks)
	}
	known := findEntities(t, map[string]any{
		"event_id": eventIDString(e1), "known_only": true,
	})
	if len(known) != 1 || known[0]["entity_id"] != son.String() {
		t.Fatalf("son knowledge of execution: %#v", known)
	}

	// Walk causal chain e1→e2→e3→e4 via find tools.
	step := eventIDString(e1)
	for _, next := range []string{eventIDString(e2), eventIDString(e3), eventIDString(e4)} {
		edges := findCausals(t, map[string]any{"from": step})
		if len(edges) != 1 || edges[0]["to"] != next {
			t.Fatalf("chain from %s: want →%s, got %#v", step, next, edges)
		}
		step = next
	}
}

func TestExecutor_ScenarioB_TimeTravelCycle(t *testing.T) {
	resetDB(t)

	// Insert order ≠ game-time order.
	future := createEventViaTool(t, map[string]any{
		"importance": 50, "created_at_game_time": 900,
		"summary": "Future messenger arrives",
	})
	past := createEventViaTool(t, map[string]any{
		"importance": 50, "created_at_game_time": 100,
		"summary": "Past decision is made",
	})
	mid := createEventViaTool(t, map[string]any{
		"importance": 50, "created_at_game_time": 400,
		"summary": "Mid timeline hinge",
	})

	mustExecute(t, "create_event_causal", map[string]any{
		"from": eventIDString(past), "to": eventIDString(mid), "cause": "past shapes mid",
	})
	mustExecute(t, "create_event_causal", map[string]any{
		"from": eventIDString(mid), "to": eventIDString(future), "cause": "mid shapes future",
	})
	mustExecute(t, "create_event_causal", map[string]any{
		"from": eventIDString(future), "to": eventIDString(past), "cause": "future informs past",
	})

	for _, id := range []struct {
		id    string
		ticks float64
	}{
		{eventIDString(past), 100},
		{eventIDString(mid), 400},
		{eventIDString(future), 900},
	} {
		found := findEvents(t, map[string]any{"id": id.id})
		if len(found) != 1 {
			t.Fatalf("find %s: %#v", id.id, found)
		}
		got := found[0]["created_at_game_time"].(float64)
		if got != id.ticks {
			t.Fatalf("game time for %s: want %v got %v", id.id, id.ticks, got)
		}
	}

	// Search can return future-relative events without chronological causal constraints.
	from := 500
	futureHits := findEvents(t, map[string]any{"created_at_from": from})
	if len(futureHits) != 1 || futureHits[0]["id"] != eventIDString(future) {
		t.Fatalf("future window: %#v", futureHits)
	}

	if countCausals(t) != 3 {
		t.Fatalf("cycle must persist, count=%d", countCausals(t))
	}

	raw, err := execute(t, "create_event_causal", map[string]any{
		"from": eventIDString(past), "to": eventIDString(past), "cause": "self",
	})
	if err == nil {
		t.Fatalf("direct self-causality must fail, raw=%s", raw)
	}
	if countCausals(t) != 3 {
		t.Fatalf("self-link must not add edge, count=%d", countCausals(t))
	}
}

func TestExecutor_ScenarioC_AtomicNestedCreate(t *testing.T) {
	resetDB(t)

	beforeEvents := countEvents(t)
	beforeEntities := countEntities(t)
	beforeCausals := countCausals(t)

	missingCause := uuid.New().String()
	entityID := newEntityID()

	invokeID, callID := "inv-atomic", "call-atomic"
	raw, err := executeNamed(t, invokeID, callID, "create_event", map[string]any{
		"importance":           70,
		"created_at_game_time": 42,
		"summary":              "should roll back entirely",
		"tags":                 []string{"atomic"},
		"details":              map[string]any{"marker": "partial-must-not-remain"},
		"entities": []map[string]any{
			{"entity_id": entityID.String(), "role": "actor"},
		},
		"causals": []map[string]any{
			{"from": missingCause, "cause": "ghost cause"},
		},
	})
	if err == nil {
		t.Fatalf("nested invalid causal must fail create_event, raw=%s", raw)
	}
	if len(raw) != 0 {
		t.Fatalf("failed create must not return payload, got %s", raw)
	}

	if countEvents(t) != beforeEvents || countEntities(t) != beforeEntities || countCausals(t) != beforeCausals {
		t.Fatalf("partial write leaked: events=%d entities=%d causals=%d",
			countEvents(t), countEntities(t), countCausals(t))
	}

	leaks := findEvents(t, map[string]any{"query": "partial-must-not-remain"})
	if len(leaks) != 0 {
		t.Fatalf("rolled-back event still findable: %#v", leaks)
	}
	assertJournalFailed(t, invokeID, callID, "create_event")
}
