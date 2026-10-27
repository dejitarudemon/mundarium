package events_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/dejitarudemon/mundarium/internal/repositories"
	"github.com/dejitarudemon/mundarium/internal/repositories/entities"
	"github.com/dejitarudemon/mundarium/internal/repositories/events"
	"github.com/lib/pq"
	"gorm.io/gorm"
)

func createRepoEvent(t *testing.T, importance int8, summary string, tags ...string) *events.Event {
	t.Helper()
	ev, err := testRepo.CreateEvent(context.Background(), &events.Event{
		Importance:        importance,
		CreatedAtGameTime: 10,
		Summary:           summary,
		Tags:              pq.StringArray(tags),
	})
	if err != nil {
		t.Fatalf("CreateEvent(%q): %v", summary, err)
	}
	return ev
}

func TestEventsRepository_CreateGetPreload(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	cause := createRepoEvent(t, 40, "cause", "alpha")
	effect := createRepoEvent(t, 50, "effect", "beta")

	entityID := entities.New()
	if err := testRepo.CreateEventEntity(ctx, &events.EventEntity{
		EntityID: entityID,
		EventID:  cause.ID,
		Role:     events.EventEntityRoleActor,
	}); err != nil {
		t.Fatalf("CreateEventEntity: %v", err)
	}
	if err := testRepo.CreateEventCausal(ctx, &events.EventCausal{
		From:  cause.ID,
		To:    effect.ID,
		Cause: "leads to",
	}); err != nil {
		t.Fatalf("CreateEventCausal: %v", err)
	}

	got, err := testRepo.GetEvent(ctx, cause.ID)
	if err != nil {
		t.Fatalf("GetEvent: %v", err)
	}
	if got.Summary != "cause" {
		t.Fatalf("summary: want cause, got %q", got.Summary)
	}
	if len(got.Entities) != 1 || got.Entities[0].EntityID != entityID {
		t.Fatalf("preload Entities: %#v", got.Entities)
	}
	if len(got.Causals) != 1 || got.Causals[0].To != effect.ID {
		t.Fatalf("preload Causals: %#v", got.Causals)
	}
}

func TestEventsRepository_PatchEventFields(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	ev := createRepoEvent(t, 10, "old")
	if err := testRepo.PatchEventImportance(ctx, ev.ID, 101); !errors.Is(err, events.ErrEventImportanceOutOfRange) {
		t.Fatalf("want ErrEventImportanceOutOfRange, got %v", err)
	}
	if err := testRepo.PatchEventImportance(ctx, ev.ID, 55); err != nil {
		t.Fatalf("PatchEventImportance: %v", err)
	}
	if err := testRepo.PatchEventSummary(ctx, ev.ID, "new"); err != nil {
		t.Fatalf("PatchEventSummary: %v", err)
	}
	if err := testRepo.PatchEventDetails(ctx, ev.ID, repositories.JSON(`{"k":1}`)); err != nil {
		t.Fatalf("PatchEventDetails: %v", err)
	}

	got, err := testRepo.GetEvent(ctx, ev.ID)
	if err != nil {
		t.Fatalf("GetEvent: %v", err)
	}
	if got.Importance != 55 || got.Summary != "new" {
		t.Fatalf("unexpected event: %+v", got)
	}
	var details map[string]any
	if err := json.Unmarshal(got.Details, &details); err != nil || details["k"] != float64(1) {
		t.Fatalf("Details: %s (%v)", got.Details, err)
	}
}

func TestEventsRepository_FindEventsByLocationID(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	locA := repositories.ID(101)
	locB := repositories.ID(202)
	a, err := testRepo.CreateEvent(ctx, &events.Event{
		LocationID:        &locA,
		Importance:        10,
		CreatedAtGameTime: 1,
		Summary:           "at-a",
	})
	if err != nil {
		t.Fatalf("CreateEvent a: %v", err)
	}
	_, err = testRepo.CreateEvent(ctx, &events.Event{
		LocationID:        &locB,
		Importance:        10,
		CreatedAtGameTime: 2,
		Summary:           "at-b",
	})
	if err != nil {
		t.Fatalf("CreateEvent b: %v", err)
	}
	_ = createRepoEvent(t, 10, "nowhere")

	got, err := testRepo.FindEvents(ctx, events.FindEventsFilter{LocationID: &locA})
	if err != nil {
		t.Fatalf("FindEvents: %v", err)
	}
	if len(got) != 1 || got[0].ID != a.ID {
		t.Fatalf("want only event a, got %#v", got)
	}
}

func TestEventsRepository_PatchEventLocation(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	ev := createRepoEvent(t, 10, "unplaced")
	if err := testRepo.PatchEventLocation(ctx, ev.ID, 0); !errors.Is(err, events.ErrEventLocationRequired) {
		t.Fatalf("want ErrEventLocationRequired, got %v", err)
	}

	locID := repositories.ID(42)
	if err := testRepo.PatchEventLocation(ctx, ev.ID, locID); err != nil {
		t.Fatalf("PatchEventLocation: %v", err)
	}
	got, err := testRepo.GetEvent(ctx, ev.ID)
	if err != nil {
		t.Fatalf("GetEvent: %v", err)
	}
	if got.LocationID == nil || *got.LocationID != locID {
		t.Fatalf("want location %d, got %v", locID, got.LocationID)
	}

	other := repositories.ID(99)
	if err := testRepo.PatchEventLocation(ctx, ev.ID, other); err != nil {
		t.Fatalf("PatchEventLocation replace: %v", err)
	}
	got, err = testRepo.GetEvent(ctx, ev.ID)
	if err != nil {
		t.Fatalf("GetEvent: %v", err)
	}
	if got.LocationID == nil || *got.LocationID != other {
		t.Fatalf("want location %d, got %v", other, got.LocationID)
	}
}

func TestEventsRepository_FindEventsByTags(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	_ = createRepoEvent(t, 10, "a", "sword")
	_ = createRepoEvent(t, 20, "b", "shield")
	_ = createRepoEvent(t, 30, "c", "sword", "quest")

	got, err := testRepo.FindEvents(ctx, events.FindEventsFilter{Tags: []string{"sword"}})
	if err != nil {
		t.Fatalf("FindEvents: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 sword events, got %#v", got)
	}

	got, err = testRepo.FindEvents(ctx, events.FindEventsFilter{Tags: []string{"sword", "shield"}})
	if err != nil {
		t.Fatalf("FindEvents multi: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("want 3 events matching any tag, got %#v", got)
	}
}

func TestEventsRepository_PatchEventEntity_KnowledgeFlow(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	a := createRepoEvent(t, 10, "a")
	entityID := entities.New()
	if err := testRepo.CreateEventEntity(ctx, &events.EventEntity{
		EntityID: entityID,
		EventID:  a.ID,
		Role:     events.EventEntityRoleWitness,
	}); err != nil {
		t.Fatalf("CreateEventEntity: %v", err)
	}

	if err := testRepo.PatchEventEntityBelief(ctx, entityID, a.ID, 50); !errors.Is(err, events.ErrEventEntityNotKnown) {
		t.Fatalf("want ErrEventEntityNotKnown, got %v", err)
	}

	if err := testRepo.PatchEventEntityKnownAtGameTime(ctx, entityID, a.ID, 7, 80); err != nil {
		t.Fatalf("PatchEventEntityKnownAtGameTime: %v", err)
	}
	if err := testRepo.PatchEventEntityKnownAtGameTime(ctx, entityID, a.ID, 99, 10); !errors.Is(err, events.ErrEventEntityAlreadyKnown) {
		t.Fatalf("want ErrEventEntityAlreadyKnown, got %v", err)
	}

	if err := testRepo.PatchEventEntityRole(ctx, entityID, a.ID, events.EventEntityRoleActor); err != nil {
		t.Fatalf("PatchEventEntityRole: %v", err)
	}
	if err := testRepo.PatchEventEntityBelief(ctx, entityID, a.ID, 20); err != nil {
		t.Fatalf("PatchEventEntityBelief: %v", err)
	}
	if err := testRepo.PatchEventEntityDetails(ctx, entityID, a.ID, repositories.JSON(`{"source":"rumor"}`)); err != nil {
		t.Fatalf("PatchEventEntityDetails: %v", err)
	}

	got, err := testRepo.GetEventEntity(ctx, entityID, a.ID)
	if err != nil {
		t.Fatalf("GetEventEntity: %v", err)
	}
	if got.Role != events.EventEntityRoleActor {
		t.Fatalf("role: %v", got.Role)
	}
	if got.KnownAtGameTime == nil || *got.KnownAtGameTime != 7 {
		t.Fatalf("KnownAtGameTime must stay 7, got %v", got.KnownAtGameTime)
	}
	if got.Belief == nil || *got.Belief != 20 {
		t.Fatalf("Belief: %v", got.Belief)
	}
	var details map[string]any
	if err := json.Unmarshal(got.Details, &details); err != nil {
		t.Fatalf("Details JSON: %v (%s)", err, got.Details)
	}
	if details["source"] != "rumor" {
		t.Fatalf("Details: %#v", details)
	}
}

func TestEventsRepository_PatchEventCausalCause(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	a := createRepoEvent(t, 10, "a")
	b := createRepoEvent(t, 10, "b")
	if err := testRepo.CreateEventCausal(ctx, &events.EventCausal{From: a.ID, To: b.ID, Cause: "then"}); err != nil {
		t.Fatalf("CreateEventCausal: %v", err)
	}
	if err := testRepo.PatchEventCausalCause(ctx, a.ID, b.ID, "therefore"); err != nil {
		t.Fatalf("PatchEventCausalCause: %v", err)
	}
	gotEdge, err := testRepo.GetEventCausal(ctx, a.ID, b.ID)
	if err != nil {
		t.Fatalf("GetEventCausal: %v", err)
	}
	if gotEdge.Cause != "therefore" {
		t.Fatalf("cause: %q", gotEdge.Cause)
	}
}

func TestEventsRepository_DeleteEvent_CascadesCausalsAndEntities(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	a := createRepoEvent(t, 10, "a")
	b := createRepoEvent(t, 10, "b")
	entityID := entities.New()
	if err := testRepo.CreateEventEntity(ctx, &events.EventEntity{
		EntityID: entityID,
		EventID:  a.ID,
		Role:     events.EventEntityRoleActor,
	}); err != nil {
		t.Fatalf("CreateEventEntity: %v", err)
	}
	if err := testRepo.CreateEventCausal(ctx, &events.EventCausal{From: a.ID, To: b.ID, Cause: "x"}); err != nil {
		t.Fatalf("CreateEventCausal: %v", err)
	}

	if err := testRepo.DeleteEvent(ctx, a.ID); err != nil {
		t.Fatalf("DeleteEvent: %v", err)
	}
	if _, err := testRepo.GetEvent(ctx, a.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("GetEvent after delete: %v", err)
	}
	if _, err := testRepo.GetEventEntity(ctx, entityID, a.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("GetEventEntity after delete: %v", err)
	}
	if _, err := testRepo.GetEventCausal(ctx, a.ID, b.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("GetEventCausal after delete: %v", err)
	}
	if _, err := testRepo.GetEvent(ctx, b.ID); err != nil {
		t.Fatalf("effect event must remain: %v", err)
	}
}

func TestEventsRepository_GetEvent_NotFound(t *testing.T) {
	resetDB(t)

	_, err := testRepo.GetEvent(context.Background(), events.New())
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("want ErrRecordNotFound, got %v", err)
	}
}

func TestEventsRepository_CreateEventCausal_RejectsSelfLink(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	ev := createRepoEvent(t, 10, "self")
	err := testRepo.CreateEventCausal(ctx, &events.EventCausal{
		From:  ev.ID,
		To:    ev.ID,
		Cause: "loop",
	})
	if !errors.Is(err, events.ErrEventCausalSelfLink) {
		t.Fatalf("want ErrEventCausalSelfLink, got %v", err)
	}
}

func TestEventsRepository_CreateEventCausal_RejectsIncomplete(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	ev := createRepoEvent(t, 10, "endpoint")
	cases := []events.EventCausal{
		{From: ev.ID, To: events.ID{}, Cause: "missing to"},
		{From: events.ID{}, To: ev.ID, Cause: "missing from"},
		{From: events.ID{}, To: events.ID{}, Cause: "missing both"},
	}
	for _, edge := range cases {
		err := testRepo.CreateEventCausal(ctx, &edge)
		if !errors.Is(err, events.ErrEventCausalIncomplete) {
			t.Fatalf("edge=%+v: want ErrEventCausalIncomplete, got %v", edge, err)
		}
	}
}

