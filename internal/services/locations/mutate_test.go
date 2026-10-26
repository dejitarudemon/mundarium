package locations_test

import (
	"errors"
	"testing"

	journal "github.com/dejitarudemon/mundarium/internal/libs/journals/execution"
	"github.com/dejitarudemon/mundarium/internal/repositories"
	repolocations "github.com/dejitarudemon/mundarium/internal/repositories/locations"
	"gorm.io/gorm"
)

func TestMoveLocation(t *testing.T) {
	resetDB(t)
	root := seedRoot(t, "root")
	other := seedRoot(t, "other")
	child := seedChild(t, root.ID, "child")

	rec := &completeRecorder{}
	ctx, cancel := newExecCtx(t, rec)
	defer cancel()

	if err := testSvc.MoveLocation(ctx, child.ID, new(other.ID)); err != nil {
		t.Fatalf("MoveLocation: %v", err)
	}

	got, err := testRepo.GetLocation(ctx, child.ID)
	if err != nil {
		t.Fatalf("GetLocation: %v", err)
	}
	if got.ParentID == nil || *got.ParentID != other.ID {
		t.Fatalf("want parent %d, got %v", other.ID, got.ParentID)
	}
	if len(rec.calls) != 1 || rec.calls[0].Action != journal.Action_Patch {
		t.Fatalf("unexpected complete calls: %#v", rec.calls)
	}
}

func TestMoveLocation_RejectsCycle(t *testing.T) {
	resetDB(t)
	root := seedRoot(t, "root")
	child := seedChild(t, root.ID, "child")

	ctx, cancel := newExecCtx(t, nil)
	defer cancel()

	err := testSvc.MoveLocation(ctx, root.ID, new(child.ID))
	if !errors.Is(err, repolocations.ErrLocationWouldCreateCycle) {
		t.Fatalf("want ErrLocationWouldCreateCycle, got %v", err)
	}
}

func TestRenameLocation(t *testing.T) {
	resetDB(t)
	loc := seedRoot(t, "old")
	rec := &completeRecorder{}
	ctx, cancel := newExecCtx(t, rec)
	defer cancel()

	if err := testSvc.RenameLocation(ctx, loc.ID, "new"); err != nil {
		t.Fatalf("RenameLocation: %v", err)
	}

	got, err := testRepo.GetLocation(ctx, loc.ID)
	if err != nil {
		t.Fatalf("GetLocation: %v", err)
	}
	if got.Name != "new" {
		t.Fatalf("want name %q, got %q", "new", got.Name)
	}
	if len(rec.calls) != 1 || rec.calls[0].Action != journal.Action_Patch {
		t.Fatalf("unexpected complete calls: %#v", rec.calls)
	}
}

func TestRenameLocation_NotFound(t *testing.T) {
	resetDB(t)
	ctx, cancel := newExecCtx(t, nil)
	defer cancel()

	err := testSvc.RenameLocation(ctx, 999, "x")
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected ErrRecordNotFound, got %v", err)
	}
}

func TestUpdateLocationDetails(t *testing.T) {
	resetDB(t)
	loc := seedRoot(t, "place")
	rec := &completeRecorder{}
	ctx, cancel := newExecCtx(t, rec)
	defer cancel()

	details := repositories.JSON(`{"a":1}`)
	if err := testSvc.UpdateLocationDetails(ctx, loc.ID, details); err != nil {
		t.Fatalf("UpdateLocationDetails: %v", err)
	}

	got, err := testRepo.GetLocation(ctx, loc.ID)
	if err != nil {
		t.Fatalf("GetLocation: %v", err)
	}
	if string(got.Details) != string(details) {
		t.Fatalf("want details %s, got %s", details, got.Details)
	}
	if len(rec.calls) != 1 {
		t.Fatalf("expected Complete call, got %#v", rec.calls)
	}
}

func TestUpdateLocationDetails_NotFound(t *testing.T) {
	resetDB(t)
	ctx, cancel := newExecCtx(t, nil)
	defer cancel()

	err := testSvc.UpdateLocationDetails(ctx, 999, repositories.JSON(`{}`))
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected ErrRecordNotFound, got %v", err)
	}
}

func TestUpdateLocationDescription(t *testing.T) {
	resetDB(t)
	loc := seedRoot(t, "place")
	rec := &completeRecorder{}
	ctx, cancel := newExecCtx(t, rec)
	defer cancel()

	if err := testSvc.UpdateLocationDescription(ctx, loc.ID, "updated desc"); err != nil {
		t.Fatalf("UpdateLocationDescription: %v", err)
	}

	got, err := testRepo.GetLocation(ctx, loc.ID)
	if err != nil {
		t.Fatalf("GetLocation: %v", err)
	}
	if got.Description != "updated desc" {
		t.Fatalf("want description updated, got %q", got.Description)
	}
	if len(rec.calls) != 1 {
		t.Fatalf("expected Complete call, got %#v", rec.calls)
	}
}

func TestUpdateLocationDescription_NotFound(t *testing.T) {
	resetDB(t)
	ctx, cancel := newExecCtx(t, nil)
	defer cancel()

	err := testSvc.UpdateLocationDescription(ctx, 999, "x")
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected ErrRecordNotFound, got %v", err)
	}
}
