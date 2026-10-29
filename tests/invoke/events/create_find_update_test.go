package events_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/dejitarudemon/mundarium/internal/repositories"
	"github.com/google/uuid"
)

func TestInvoke_CreateEvent_Required(t *testing.T) {
	resetDB(t)

	invoke(t, "Создай запись о том, что барон приказал казнить мельника. Важность события — 70. Поле created_at_game_time (игровые ticks) = 10.")

	seq := snapshotToolSeq()
	requireTool(t, seq, "create_event")

	var args struct {
		Importance        int8               `json:"importance"`
		CreatedAtGameTime repositories.Ticks `json:"created_at_game_time"`
		Summary           string             `json:"summary"`
	}
	unmarshalArgs(t, firstToolArgs(seq, "create_event"), &args)
	if args.Importance != 70 {
		t.Fatalf("ArgumentError: importance want 70 got %d", args.Importance)
	}
	if args.CreatedAtGameTime != 10 {
		t.Fatalf("ArgumentError: created_at_game_time want 10 ticks, got %d", args.CreatedAtGameTime)
	}
	if strings.TrimSpace(args.Summary) == "" {
		t.Fatal("ArgumentError: empty summary")
	}

	if countEvents(t) != 1 {
		t.Fatalf("ExecutionError: want 1 event in DB, got %d", countEvents(t))
	}
	ev := listEvents(t)[0]
	if ev.Importance != 70 || ev.CreatedAtGameTime != 10 {
		t.Fatalf("DB mismatch: %+v", ev)
	}
}

func TestInvoke_CreateEvent_WithOptionalFields(t *testing.T) {
	resetDB(t)

	invoke(t, `Создай событие: в локации 7 произошла засада у ворот.
Важность 80. created_at_game_time (ticks) = 15. Теги: combat, quest. В details укажи weapon=bow. Кратко опиши суть в summary.`)

	seq := snapshotToolSeq()
	requireTool(t, seq, "create_event")

	var args struct {
		LocationID        *repositories.ID   `json:"location_id"`
		Importance        int8               `json:"importance"`
		CreatedAtGameTime repositories.Ticks `json:"created_at_game_time"`
		Tags              []string           `json:"tags"`
		Details           map[string]any     `json:"details"`
		Summary           string             `json:"summary"`
	}
	unmarshalArgs(t, firstToolArgs(seq, "create_event"), &args)
	if args.Importance != 80 {
		t.Fatalf("ArgumentError: importance=%d", args.Importance)
	}
	if args.CreatedAtGameTime != 15 {
		t.Fatalf("ArgumentError: game time=%d", args.CreatedAtGameTime)
	}
	if args.LocationID == nil || *args.LocationID != 7 {
		t.Fatalf("ArgumentError: location_id=%v", args.LocationID)
	}
	if len(args.Tags) == 0 {
		t.Fatal("ArgumentError: expected tags")
	}
	if args.Details["weapon"] != "bow" {
		t.Fatalf("ArgumentError: details=%#v", args.Details)
	}

	ev := listEvents(t)
	if len(ev) != 1 {
		t.Fatalf("want 1 event, got %d", len(ev))
	}
	if ev[0].LocationID == nil || *ev[0].LocationID != 7 || ev[0].Importance != 80 {
		t.Fatalf("DB: %+v", ev[0])
	}
}

func TestInvoke_FindEvents_Filters(t *testing.T) {
	resetDB(t)

	loc := repositories.ID(3)
	miller := seedEvent(t, "Барон приказал казнить мельника", 70, 10,
		withTags("politics", "crime"), withLocation(loc), withDetails(`{"victim":"miller"}`))
	_ = seedEvent(t, "Открылся рыночный фестиваль", 20, 12, withTags("fest"), withLocation(repositories.ID(9)))
	_ = seedEvent(t, "Засада у ворот", 80, 30, withTags("combat", "quest"), withLocation(loc))

	t.Run("query_text", func(t *testing.T) {
		resetToolSeq()
		invoke(t, "Найди событие, связанное с казнью мельника.")
		seq := snapshotToolSeq()
		requireTool(t, seq, "find_events")
		var args struct {
			Query *string `json:"query"`
			ID    *string `json:"id"`
		}
		unmarshalArgs(t, firstToolArgs(seq, "find_events"), &args)
		if (args.Query == nil || strings.TrimSpace(*args.Query) == "") && args.ID == nil {
			t.Fatal("ArgumentError: find_events without query or id")
		}
	})

	t.Run("by_uuid", func(t *testing.T) {
		resetToolSeq()
		invoke(t, fmt.Sprintf("Найди событие с UUID %s.", miller.ID.String()))
		seq := snapshotToolSeq()
		requireTool(t, seq, "find_events")
		var args struct {
			ID *string `json:"id"`
		}
		unmarshalArgs(t, firstToolArgs(seq, "find_events"), &args)
		if args.ID == nil || *args.ID != miller.ID.String() {
			t.Fatalf("ArgumentError: id=%v want %s", args.ID, miller.ID)
		}
	})

	t.Run("by_tags_importance_location_time", func(t *testing.T) {
		resetToolSeq()
		invoke(t, `Найди события с тегом combat, важностью не ниже 75, в локации 3, с игровым временем от 20 до 40 включительно.
Одним send_message перечисли найденные id.`)
		seq := snapshotToolSeq()
		requireTool(t, seq, "find_events")
		var args map[string]any
		unmarshalArgs(t, firstToolArgs(seq, "find_events"), &args)
		if tags, _ := args["tags"].([]any); len(tags) == 0 {
			t.Fatalf("ArgumentError: expected tags filter, args=%s", compactJSON(firstToolArgs(seq, "find_events")))
		}
	})
}

func TestInvoke_UpdateEvent_Fields(t *testing.T) {
	resetDB(t)

	ev := seedEvent(t, "Барон приказал казнить мельника", 70, 10,
		withTags("politics"), withLocation(repositories.ID(1)), withDetails(`{"victim":"miller"}`))

	t.Run("importance", func(t *testing.T) {
		resetToolSeq()
		invoke(t, "Найди событие о казни мельника и измени его важность на 85.")
		seq := snapshotToolSeq()
		requireTool(t, seq, "find_events")
		requireTool(t, seq, "update_event_importance")

		var args struct {
			ID         string `json:"id"`
			Importance int8   `json:"importance"`
		}
		unmarshalArgs(t, firstToolArgs(seq, "update_event_importance"), &args)
		if args.ID != ev.ID.String() {
			t.Fatalf("ReferenceError: updated id=%s want %s", args.ID, ev.ID)
		}
		if args.Importance != 85 {
			t.Fatalf("ArgumentError: importance=%d", args.Importance)
		}
		got := getEvent(t, ev.ID)
		if got.Importance != 85 {
			t.Fatalf("DB importance=%d", got.Importance)
		}
		if got.Summary != ev.Summary || got.CreatedAtGameTime != ev.CreatedAtGameTime {
			t.Fatalf("untouched fields changed: %+v", got)
		}
	})

	t.Run("summary_details_location", func(t *testing.T) {
		resetToolSeq()
		before := getEvent(t, ev.ID)
		invoke(t, fmt.Sprintf(
			`Для события %s: (1) замени summary на "Revised miller execution order"; (2) поставь details {"victim":"miller","place":"square"}; (3) поставь location_id=42.
Не меняй importance и created_at_game_time.`,
			ev.ID.String(),
		))
		seq := snapshotToolSeq()
		requireTool(t, seq, "update_event_summary")
		requireTool(t, seq, "update_event_details")
		requireTool(t, seq, "update_event_location")

		got := getEvent(t, ev.ID)
		if got.Summary != "Revised miller execution order" {
			t.Fatalf("summary=%q", got.Summary)
		}
		if !strings.Contains(string(got.Details), "square") {
			t.Fatalf("details=%s", got.Details)
		}
		if got.LocationID == nil || *got.LocationID != 42 {
			t.Fatalf("location=%v", got.LocationID)
		}
		if got.Importance != before.Importance || got.CreatedAtGameTime != before.CreatedAtGameTime {
			t.Fatalf("immutable/untouched fields changed: before importance=%d time=%d after %+v",
				before.Importance, before.CreatedAtGameTime, got)
		}
	})
}

func TestInvoke_CreateEvent_RejectsInventedSuccessWithoutTool(t *testing.T) {
	// Covered implicitly by required create: if no create_event, countEvents fails.
	// Keep a focused missing-id update path for FalseSuccess / ToolResultHandling.
	resetDB(t)
	missing := uuid.New().String()
	invokeID := invoke(t, fmt.Sprintf(
		`Обнови важность события %s на 10. Если инструмент вернул ошибку — сообщи об ошибке через send_message, не утверждай успех.`,
		missing,
	))
	seq := snapshotToolSeq()
	if countTool(seq, "update_event_importance") < 1 && countTool(seq, "find_events") < 1 {
		t.Fatalf("ToolSelectionError: expected update or find, got %v", toolNames(seq))
	}
	if countEvents(t) != 0 {
		t.Fatal("must not create events on missing-id update")
	}
	if text, ok := lastPlayerMessage(t, invokeID); ok {
		lower := strings.ToLower(text)
		if strings.Contains(lower, "успеш") && !strings.Contains(lower, "ошиб") && !strings.Contains(lower, "error") && !strings.Contains(lower, "не ") {
			t.Fatalf("FalseSuccess: player message claims success: %q", text)
		}
	}
}
