package locations_test

import (
	"errors"
	"testing"

	journal "github.com/dejitarudemon/mundarium/internal/libs/journals/execution"
	"github.com/dejitarudemon/mundarium/internal/repositories"
	svcerrs "github.com/dejitarudemon/mundarium/internal/services/errs"
	svclocations "github.com/dejitarudemon/mundarium/internal/services/locations"
)

func TestCreateSubtype(t *testing.T) {
	resetDB(t)
	rec := &completeRecorder{}
	ctx, cancel := newExecCtx(t, rec)
	defer cancel()

	out, err := testSvc.CreateSubtype(ctx, svclocations.CreateSubtypeInput{
		Key:          "TaVeRn",
		Type:         "structure",
		DefiningTags: []string{"rest", "sleep"},
		ExpectedTags: []string{"village"},
		Description:  "used for safe, paid recreation",
	})
	if err != nil {
		t.Fatalf("CreateSubtype: %v", err)
	}
	if out.ID == 0 || out.Key != "tavern" {
		t.Fatalf("unexpected output: %+v", out)
	}
	if len(rec.calls) != 1 || rec.calls[0].Action != journal.Action_Create || rec.calls[0].Entity != "LOCATION_SUBTYPE" {
		t.Fatalf("unexpected complete calls: %#v", rec.calls)
	}

	got, err := testSvc.GetSubtype(ctx, out.ID)
	if err != nil {
		t.Fatalf("GetSubtype: %v", err)
	}
	if got.Key != "tavern" || got.Type != "structure" || got.Description != "used for safe, paid recreation" {
		t.Fatalf("unexpected got: %+v", got)
	}
	if len(got.DefiningTags) != 2 || got.DefiningTags[0] != "rest" {
		t.Fatalf("unexpected defining tags: %#v", got.DefiningTags)
	}
}

func TestCreateSubtype_ValidationErrors(t *testing.T) {
	resetDB(t)
	rec := &completeRecorder{}
	ctx, cancel := newExecCtx(t, rec)
	defer cancel()

	t.Run("empty key", func(t *testing.T) {
		_, err := testSvc.CreateSubtype(ctx, svclocations.CreateSubtypeInput{
			Key:  "  ",
			Type: "structure",
		})
		if _, ok := errors.AsType[svcerrs.InvalidRequest](err); !ok {
			t.Fatalf("want errs.InvalidRequest for empty key, got %v", err)
		}
		if len(rec.failCalls) == 0 {
			t.Fatal("want Fail called on validation error")
		}
	})

	t.Run("unknown type", func(t *testing.T) {
		before := len(rec.failCalls)
		_, err := testSvc.CreateSubtype(ctx, svclocations.CreateSubtypeInput{
			Key:  "tavern",
			Type: "not-a-type",
		})
		if err == nil {
			t.Fatal("expected type error")
		}
		if len(rec.failCalls) <= before {
			t.Fatal("want Fail called on validation error")
		}
	})
}

func TestGetSubtype_NotFound(t *testing.T) {
	resetDB(t)
	ctx, cancel := newExecCtx(t, nil)
	defer cancel()

	_, err := testSvc.GetSubtype(ctx, 999)
	if _, ok := errors.AsType[svcerrs.NotFound](err); !ok {
		t.Fatalf("want errs.NotFound, got %v", err)
	}
}

func TestFindSubtypes(t *testing.T) {
	resetDB(t)
	rec := &completeRecorder{}
	ctx, cancel := newExecCtx(t, rec)
	defer cancel()

	tavern, err := testSvc.CreateSubtype(ctx, svclocations.CreateSubtypeInput{
		Key:  "tavern",
		Type: "structure",
	})
	if err != nil {
		t.Fatalf("seed tavern: %v", err)
	}
	if _, err := testSvc.CreateSubtype(ctx, svclocations.CreateSubtypeInput{
		Key:  "castle",
		Type: "structure",
	}); err != nil {
		t.Fatalf("seed castle: %v", err)
	}
	if _, err := testSvc.CreateSubtype(ctx, svclocations.CreateSubtypeInput{
		Key:  "cave",
		Type: "room",
	}); err != nil {
		t.Fatalf("seed cave: %v", err)
	}

	byKeys, err := testSvc.FindSubtypes(ctx, svclocations.FindSubtypesInput{
		Keys: []string{"tav", "cas"},
	})
	if err != nil {
		t.Fatalf("FindSubtypes by keys: %v", err)
	}
	if len(byKeys.Subtypes) != 2 {
		t.Fatalf("want 2 subtypes, got %#v", byKeys.Subtypes)
	}

	structure := "structure"
	byType, err := testSvc.FindSubtypes(ctx, svclocations.FindSubtypesInput{Type: &structure})
	if err != nil {
		t.Fatalf("FindSubtypes by type: %v", err)
	}
	if len(byType.Subtypes) != 2 {
		t.Fatalf("want 2 structures, got %#v", byType.Subtypes)
	}

	byID, err := testSvc.FindSubtypes(ctx, svclocations.FindSubtypesInput{ID: new(tavern.ID)})
	if err != nil {
		t.Fatalf("FindSubtypes by id: %v", err)
	}
	if len(byID.Subtypes) != 1 || byID.Subtypes[0].ID != tavern.ID {
		t.Fatalf("want only tavern, got %#v", byID.Subtypes)
	}

	if len(rec.calls) == 0 {
		t.Fatal("want Complete called")
	}
	last := rec.calls[len(rec.calls)-1]
	if last.Action != journal.Action_Get || last.Entity != "LOCATION_SUBTYPE" {
		t.Fatalf("unexpected last complete: %#v", last)
	}
}

func TestFindSubtypes_ZeroID(t *testing.T) {
	resetDB(t)
	ctx, cancel := newExecCtx(t, &completeRecorder{})
	defer cancel()

	_, err := testSvc.FindSubtypes(ctx, svclocations.FindSubtypesInput{ID: new(repositories.ID(0))})
	if _, ok := errors.AsType[svcerrs.InvalidRequest](err); !ok {
		t.Fatalf("want errs.InvalidRequest, got %v", err)
	}
}
