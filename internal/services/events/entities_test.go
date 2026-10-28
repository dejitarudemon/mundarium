package events_test

import (
	"errors"
	"testing"

	"github.com/dejitarudemon/mundarium/internal/repositories"
	svcerrs "github.com/dejitarudemon/mundarium/internal/services/errs"
	svcevents "github.com/dejitarudemon/mundarium/internal/services/events"
)

func TestEventEntity_AddAndPatch(t *testing.T) {
	resetDB(t)
	ctx, cancel := newExecCtx(t, nil)
	defer cancel()

	ev := createEvent(t, "meeting", 20, 10)
	entityID := newEntityID()

	link, err := testSvc.CreateEventEntity(ctx, svcevents.CreateEventEntityInput{
		EntityID: entityID.String(),
		EventID:  ev.ID,
		Role:     "witness",
		Details:  repositories.JSON(`{"note":"saw it"}`),
	})
	if err != nil {
		t.Fatalf("CreateEventEntity: %v", err)
	}
	if link.Role != "witness" || link.EntityID != entityID.String() {
		t.Fatalf("unexpected link: %+v", link)
	}

	if err := testSvc.PatchEventEntityRole(ctx, entityID, ev.ID, "actor"); err != nil {
		t.Fatalf("PatchEventEntityRole: %v", err)
	}
	if err := testSvc.PatchEventEntityBelief(ctx, entityID, ev.ID, 50); err == nil {
		t.Fatal("want error when belief before known_at")
	} else if _, ok := errors.AsType[svcerrs.InvalidRequest](err); !ok {
		t.Fatalf("want InvalidRequest, got %v", err)
	}
	if err := testSvc.PatchEventEntityKnownAtGameTime(ctx, entityID, ev.ID, 15, 70); err != nil {
		t.Fatalf("PatchEventEntityKnownAtGameTime: %v", err)
	}
	if err := testSvc.PatchEventEntityBelief(ctx, entityID, ev.ID, 40); err != nil {
		t.Fatalf("PatchEventEntityBelief: %v", err)
	}
	if err := testSvc.PatchEventEntityDetails(ctx, entityID, ev.ID, repositories.JSON(`{"source":"rumor"}`)); err != nil {
		t.Fatalf("PatchEventEntityDetails: %v", err)
	}

	got, err := testSvc.GetEventEntity(ctx, entityID, ev.ID)
	if err != nil {
		t.Fatalf("GetEventEntity: %v", err)
	}
	if got.Role != "actor" || got.Belief == nil || *got.Belief != 40 || got.KnownAtGameTime == nil {
		t.Fatalf("unexpected entity: %+v", got)
	}

	found, err := testSvc.FindEventEntities(ctx, svcevents.FindEventEntitiesInput{
		EventID:   &ev.ID,
		KnownOnly: true,
	})
	if err != nil {
		t.Fatalf("FindEventEntities: %v", err)
	}
	if len(found.Entities) != 1 || found.Entities[0].EntityID != entityID.String() {
		t.Fatalf("want known entity, got %#v", found.Entities)
	}
}
