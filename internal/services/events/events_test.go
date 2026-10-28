package events_test

import (
	"errors"
	"testing"

	journal "github.com/dejitarudemon/mundarium/internal/libs/journals/execution"
	"github.com/dejitarudemon/mundarium/internal/repositories"
	svcerrs "github.com/dejitarudemon/mundarium/internal/services/errs"
	svcevents "github.com/dejitarudemon/mundarium/internal/services/events"
)

func TestCreateGetEvent_WithEntitiesAndCausals(t *testing.T) {
	resetDB(t)
	ctx, cancel := newExecCtx(t, nil)
	defer cancel()

	effect := createEvent(t, "effect-later", 40, 200)
	entityID := newEntityID()
	locID := repositories.ID(7)

	rec := &completeRecorder{}
	createCtx, createCancel := newExecCtx(t, rec)
	defer createCancel()

	created, err := testSvc.CreateEvent(createCtx, svcevents.CreateEventInput{
		LocationID:        &locID,
		Importance:        80,
		CreatedAtGameTime: 50,
		Summary:           "cause-now",
		Tags:              []string{"quest", "combat"},
		Details:           repositories.JSON(`{"scene":"gate"}`),
		Entities: []svcevents.CreateEventEntityInput{
			{EntityID: entityID.String(), Role: "actor"},
		},
		Causals: []svcevents.CreateEventCausalInput{
			{To: effect.ID, Cause: "leads to"},
		},
	})
	if err != nil {
		t.Fatalf("CreateEvent: %v", err)
	}
	if created.Summary != "cause-now" {
		t.Fatalf("unexpected create output: %+v", created)
	}
	if len(rec.calls) != 1 || rec.calls[0].Action != journal.Action_Create {
		t.Fatalf("unexpected complete: %#v", rec.calls)
	}

	got, err := testSvc.GetEvent(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetEvent: %v", err)
	}
	if got.Summary != "cause-now" || got.Importance != 80 {
		t.Fatalf("unexpected event: %+v", got)
	}
	if got.LocationID == nil || *got.LocationID != locID {
		t.Fatalf("want location %d, got %v", locID, got.LocationID)
	}
	if len(got.Entities) != 1 || got.Entities[0].EntityID != entityID.String() || got.Entities[0].Role != "actor" {
		t.Fatalf("unexpected entities: %#v", got.Entities)
	}
	if len(got.Causals) != 1 || got.Causals[0].To != effect.ID || got.Causals[0].Cause != "leads to" {
		t.Fatalf("unexpected causals: %#v", got.Causals)
	}
}

func TestCreateEvent_RejectsSelfCausal(t *testing.T) {
	resetDB(t)
	// Nested causal To is validated after BeforeCreate assigns ID; self-link needs
	// CreateEventCausal with From==To, or nested To equal to new ID which is impossible
	// before create. Cover standalone self-link:
	ctx, cancel := newExecCtx(t, nil)
	defer cancel()

	ev := createEvent(t, "self", 10, 1)
	_, err := testSvc.CreateEventCausal(ctx, svcevents.CreateEventCausalInput{
		From:  ev.ID,
		To:    ev.ID,
		Cause: "loop",
	})
	if _, ok := errors.AsType[svcerrs.InvalidRequest](err); !ok {
		t.Fatalf("want InvalidRequest for self causal, got %v", err)
	}
}

func TestCreateEventCausal_AllowsCycleAndFutureCause(t *testing.T) {
	resetDB(t)
	ctx, cancel := newExecCtx(t, nil)
	defer cancel()

	past := createEvent(t, "past", 10, 10)
	future := createEvent(t, "future", 10, 1000)

	// Future event causes past event (allowed by Novikov-style service rules).
	edge, err := testSvc.CreateEventCausal(ctx, svcevents.CreateEventCausalInput{
		From:  future.ID,
		To:    past.ID,
		Cause: "retrocausal",
	})
	if err != nil {
		t.Fatalf("CreateEventCausal future->past: %v", err)
	}
	if edge.From != future.ID || edge.To != past.ID {
		t.Fatalf("unexpected edge: %+v", edge)
	}

	// Cycle past -> future -> past.
	if _, err := testSvc.CreateEventCausal(ctx, svcevents.CreateEventCausalInput{
		From:  past.ID,
		To:    future.ID,
		Cause: "closes cycle",
	}); err != nil {
		t.Fatalf("CreateEventCausal cycle: %v", err)
	}
}

func TestFindEventsAndPatchFields(t *testing.T) {
	resetDB(t)
	ctx, cancel := newExecCtx(t, nil)
	defer cancel()

	locA := repositories.ID(1)
	locB := repositories.ID(2)
	a, err := testSvc.CreateEvent(ctx, svcevents.CreateEventInput{
		LocationID:        &locA,
		Importance:        30,
		CreatedAtGameTime: 5,
		Summary:           "at-a",
		Tags:              []string{"sword"},
	})
	if err != nil {
		t.Fatalf("CreateEvent a: %v", err)
	}
	_, err = testSvc.CreateEvent(ctx, svcevents.CreateEventInput{
		LocationID:        &locB,
		Importance:        90,
		CreatedAtGameTime: 6,
		Summary:           "at-b",
		Tags:              []string{"shield"},
	})
	if err != nil {
		t.Fatalf("CreateEvent b: %v", err)
	}

	found, err := testSvc.FindEvents(ctx, svcevents.FindEventsInput{LocationID: &locA, Tags: []string{"sword"}})
	if err != nil {
		t.Fatalf("FindEvents: %v", err)
	}
	if len(found.Events) != 1 || found.Events[0].ID != a.ID {
		t.Fatalf("want only a, got %#v", found.Events)
	}

	if err := testSvc.PatchEventLocation(ctx, a.ID, locB); err != nil {
		t.Fatalf("PatchEventLocation: %v", err)
	}
	if err := testSvc.PatchEventLocation(ctx, a.ID, 0); err == nil {
		t.Fatal("want error for zero location")
	} else if _, ok := errors.AsType[svcerrs.InvalidRequest](err); !ok {
		t.Fatalf("want InvalidRequest, got %v", err)
	}

	if err := testSvc.PatchEventImportance(ctx, a.ID, 55); err != nil {
		t.Fatalf("PatchEventImportance: %v", err)
	}
	if err := testSvc.PatchEventSummary(ctx, a.ID, "moved"); err != nil {
		t.Fatalf("PatchEventSummary: %v", err)
	}
	if err := testSvc.PatchEventDetails(ctx, a.ID, repositories.JSON(`{"k":1}`)); err != nil {
		t.Fatalf("PatchEventDetails: %v", err)
	}

	got, err := testSvc.GetEvent(ctx, a.ID)
	if err != nil {
		t.Fatalf("GetEvent: %v", err)
	}
	if got.LocationID == nil || *got.LocationID != locB || got.Importance != 55 || got.Summary != "moved" {
		t.Fatalf("unexpected patched event: %+v", got)
	}
}
