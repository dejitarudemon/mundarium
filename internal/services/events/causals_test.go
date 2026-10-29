package events_test

import (
	"testing"

	svcevents "github.com/dejitarudemon/mundarium/internal/services/events"
)

func TestEventCausal_PatchAndFind(t *testing.T) {
	resetDB(t)
	ctx, cancel := newExecCtx(t, nil)
	defer cancel()

	a := createEvent(t, "a", 10, 1)
	b := createEvent(t, "b", 10, 2)
	c := createEvent(t, "c", 10, 3)

	if _, err := testSvc.CreateEventCausal(ctx, svcevents.CreateEventCausalInput{
		From: a.ID, To: b.ID, Cause: "ab",
	}); err != nil {
		t.Fatalf("CreateEventCausal ab: %v", err)
	}
	if _, err := testSvc.CreateEventCausal(ctx, svcevents.CreateEventCausalInput{
		From: b.ID, To: c.ID, Cause: "bc",
	}); err != nil {
		t.Fatalf("CreateEventCausal bc: %v", err)
	}

	if err := testSvc.PatchEventCausalCause(ctx, a.ID, b.ID, "therefore"); err != nil {
		t.Fatalf("PatchEventCausalCause: %v", err)
	}

	got, err := testSvc.GetEventCausal(ctx, a.ID, b.ID)
	if err != nil {
		t.Fatalf("GetEventCausal: %v", err)
	}
	if got.Cause != "therefore" {
		t.Fatalf("want therefore, got %q", got.Cause)
	}

	found, err := testSvc.FindEventCausals(ctx, svcevents.FindEventCausalsInput{EventID: &b.ID})
	if err != nil {
		t.Fatalf("FindEventCausals: %v", err)
	}
	if len(found.Causals) != 2 {
		t.Fatalf("want 2 edges involving b, got %#v", found.Causals)
	}
}
