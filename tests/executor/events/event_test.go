package events_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/dejitarudemon/mundarium/internal/repositories"
	repoevents "github.com/dejitarudemon/mundarium/internal/repositories/events"
	"github.com/google/uuid"
)

func TestExecutor_Event_CreateRequiredAndFull(t *testing.T) {
	resetDB(t)

	t.Run("required_fields", func(t *testing.T) {
		id := createEventViaTool(t, map[string]any{
			"importance":           40,
			"created_at_game_time": 100,
			"summary":              "Baron orders the miller hanged",
		})
		got := getEventDB(t, id)
		if got.Summary != "Baron orders the miller hanged" || got.Importance != 40 || got.CreatedAtGameTime != 100 {
			t.Fatalf("unexpected event: %+v", got)
		}
		if got.LocationID != nil {
			t.Fatalf("want nil location, got %v", got.LocationID)
		}
		if len(got.Tags) != 0 {
			t.Fatalf("want empty tags, got %v", got.Tags)
		}
	})

	t.Run("all_optional_fields_with_nested", func(t *testing.T) {
		causeID := createEventViaTool(t, map[string]any{
			"importance":           10,
			"created_at_game_time": 50,
			"summary":              "prior quarrel",
		})
		entityID := newEntityID()
		loc := repositories.ID(42)

		id := createEventViaTool(t, map[string]any{
			"location_id":          loc,
			"importance":           85,
			"created_at_game_time": 120,
			"summary":              "execution at the square",
			"tags":                 []string{"politics", "crime"},
			"details":              map[string]any{"place": "square", "weapon": "axe"},
			"entities": []map[string]any{
				{"entity_id": entityID.String(), "role": "actor", "details": map[string]any{"title": "baron"}},
			},
			"causals": []map[string]any{
				{"from": eventIDString(causeID), "cause": "quarrel escalated"},
			},
		})

		got := getEventDB(t, id)
		if got.LocationID == nil || *got.LocationID != loc {
			t.Fatalf("location: %+v", got.LocationID)
		}
		if got.Importance != 85 || got.CreatedAtGameTime != 120 {
			t.Fatalf("fields: %+v", got)
		}
		if len(got.Tags) != 2 {
			t.Fatalf("tags: %v", got.Tags)
		}
		if !strings.Contains(string(got.Details), "axe") {
			t.Fatalf("details: %s", got.Details)
		}
		if len(got.Entities) != 1 || got.Entities[0].EntityID != entityID || got.Entities[0].Role != repoevents.EventEntityRoleActor {
			t.Fatalf("entities: %#v", got.Entities)
		}

		cause := getEventDB(t, causeID)
		if len(cause.Causals) != 1 || cause.Causals[0].To != id || cause.Causals[0].Cause != "quarrel escalated" {
			t.Fatalf("incoming causal on cause: %#v", cause.Causals)
		}
	})
}

func TestExecutor_Event_FindFiltersAndUpdates(t *testing.T) {
	resetDB(t)

	locA := repositories.ID(1)
	locB := repositories.ID(2)
	a := createEventViaTool(t, map[string]any{
		"location_id": locA, "importance": 20, "created_at_game_time": 10,
		"summary": "sword training", "tags": []string{"combat"},
		"details": map[string]any{"weapon": "sword"},
	})
	b := createEventViaTool(t, map[string]any{
		"location_id": locB, "importance": 70, "created_at_game_time": 30,
		"summary": "ambush-at-gate", "tags": []string{"combat", "quest"},
		"details": map[string]any{"weapon": "bow"},
	})
	_ = createEventViaTool(t, map[string]any{
		"importance": 90, "created_at_game_time": 50,
		"summary": "council debate", "tags": []string{"politics"},
		"details": map[string]any{"topic": "taxes"},
	})

	t.Run("find_by_uuid", func(t *testing.T) {
		found := findEvents(t, map[string]any{"id": eventIDString(a)})
		if len(found) != 1 || found[0]["id"] != eventIDString(a) {
			t.Fatalf("want only a, got %#v", found)
		}
		if found[0]["summary"] != "sword training" {
			t.Fatalf("summary: %#v", found[0])
		}
		// find_events does not preload associations (API note vs GetEvent).
		ents, _ := found[0]["entities"].([]any)
		if len(ents) != 0 {
			t.Fatalf("find_events should not preload entities, got %#v", ents)
		}
	})

	t.Run("find_by_query_summary_and_details", func(t *testing.T) {
		bySummary := findEvents(t, map[string]any{"query": "ambush-at-gate"})
		if len(bySummary) != 1 || bySummary[0]["id"] != eventIDString(b) {
			t.Fatalf("summary query: %#v", bySummary)
		}
		byDetails := findEvents(t, map[string]any{"query": "sword"})
		if len(byDetails) != 1 || byDetails[0]["id"] != eventIDString(a) {
			t.Fatalf("details query: %#v", byDetails)
		}
	})

	t.Run("find_by_location_tags_importance_time_limit", func(t *testing.T) {
		byLoc := findEvents(t, map[string]any{"location_id": locA})
		if len(byLoc) != 1 || byLoc[0]["id"] != eventIDString(a) {
			t.Fatalf("location: %#v", byLoc)
		}
		byTag := findEvents(t, map[string]any{"tags": []string{"quest"}})
		if len(byTag) != 1 || byTag[0]["id"] != eventIDString(b) {
			t.Fatalf("tags: %#v", byTag)
		}
		minImp := 60
		maxImp := 80
		byImp := findEvents(t, map[string]any{"min_importance": minImp, "max_importance": maxImp})
		if len(byImp) != 1 || byImp[0]["id"] != eventIDString(b) {
			t.Fatalf("importance: %#v", byImp)
		}
		from, to := 25, 40
		byTime := findEvents(t, map[string]any{"created_at_from": from, "created_at_to": to})
		if len(byTime) != 1 || byTime[0]["id"] != eventIDString(b) {
			t.Fatalf("time: %#v", byTime)
		}
		limited := findEvents(t, map[string]any{"tags": []string{"combat"}, "limit": 1})
		if len(limited) != 1 {
			t.Fatalf("limit: %#v", limited)
		}
	})

	t.Run("updates_preserve_untouched_fields", func(t *testing.T) {
		before := getEventDB(t, a)
		gameTime := before.CreatedAtGameTime

		mustExecute(t, "update_event_summary", map[string]any{
			"id": eventIDString(a), "summary": "revised sword drill",
		})
		mustExecute(t, "update_event_details", map[string]any{
			"id": eventIDString(a), "details": map[string]any{"weapon": "longsword"},
		})
		mustExecute(t, "update_event_importance", map[string]any{
			"id": eventIDString(a), "importance": 33,
		})
		mustExecute(t, "update_event_location", map[string]any{
			"id": eventIDString(a), "location_id": 99,
		})

		got := getEventDB(t, a)
		if got.Summary != "revised sword drill" {
			t.Fatalf("summary: %q", got.Summary)
		}
		if !strings.Contains(string(got.Details), "longsword") {
			t.Fatalf("details: %s", got.Details)
		}
		if got.Importance != 33 {
			t.Fatalf("importance: %d", got.Importance)
		}
		if got.LocationID == nil || *got.LocationID != 99 {
			t.Fatalf("location: %v", got.LocationID)
		}
		if got.CreatedAtGameTime != gameTime {
			t.Fatalf("created_at_game_time must be immutable: before=%d after=%d", gameTime, got.CreatedAtGameTime)
		}
		if len(got.Tags) != len(before.Tags) {
			t.Fatalf("tags must be unchanged: before=%v after=%v", before.Tags, got.Tags)
		}
	})

	t.Run("missing_and_invalid_ids", func(t *testing.T) {
		missing := uuid.New().String()
		raw, err := execute(t, "update_event_summary", map[string]any{
			"id": missing, "summary": "ghost",
		})
		if err == nil {
			t.Fatalf("want error for missing id, raw=%s", raw)
		}
		if !strings.Contains(strings.ToLower(err.Error()), "not found") &&
			!strings.Contains(strings.ToLower(err.Error()), "record") {
			t.Fatalf("want not-found style error, got %v", err)
		}

		raw, err = execute(t, "find_events", map[string]any{"id": missing})
		if err != nil {
			t.Fatalf("find missing id should succeed with empty list: %v", err)
		}
		var out struct {
			Events []json.RawMessage `json:"events"`
		}
		if err := json.Unmarshal(raw, &out); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if len(out.Events) != 0 {
			t.Fatalf("want empty, got %s", raw)
		}

		raw, err = execute(t, "update_event_summary", map[string]any{
			"id": "not-a-uuid", "summary": "bad",
		})
		if err == nil {
			t.Fatalf("want error for invalid uuid, raw=%s", raw)
		}
	})
}
