package locations_test

import (
	"errors"
	"testing"

	input "github.com/dejitarudemon/mundarium/internal/libs/inputs/locations"
	libjournal "github.com/dejitarudemon/mundarium/internal/libs/journal"
	"github.com/dejitarudemon/mundarium/internal/libs/types"
	"gorm.io/gorm"
)

func TestCreateLocation(t *testing.T) {
	resetDB(t)
	root := seedRoot(t, "world")
	rec := &completeRecorder{}
	ctx, cancel := newExecCtx(t, rec)
	defer cancel()

	out, err := testSvc.CreateLocation(ctx, input.CreateLocationInput{
		ParentID:    root.ID,
		Type:        "region",
		Name:        "North",
		Description: "cold",
		Details:     types.JSON(`{"climate":"arctic"}`),
	})
	if err != nil {
		t.Fatalf("CreateLocation: %v", err)
	}
	if out.ID == 0 || out.ParentID != root.ID || out.Name != "North" {
		t.Fatalf("unexpected output: %+v", out)
	}
	if len(rec.calls) != 1 || rec.calls[0].Action != libjournal.Action_Create || rec.calls[0].Entity != "LOCATION" {
		t.Fatalf("unexpected complete calls: %#v", rec.calls)
	}

	got, err := testRepo.GetLocation(ctx, out.ID)
	if err != nil {
		t.Fatalf("GetLocation: %v", err)
	}
	if got.Description != "cold" {
		t.Fatalf("want description persisted, got %q", got.Description)
	}
}

func TestCreateLocation_ValidationErrors(t *testing.T) {
	resetDB(t)
	root := seedRoot(t, "world")
	ctx, cancel := newExecCtx(t, nil)
	defer cancel()

	t.Run("zero parent", func(t *testing.T) {
		_, err := testSvc.CreateLocation(ctx, input.CreateLocationInput{
			ParentID: 0,
			Type:     "region",
			Name:     "x",
		})
		if err == nil || err.Error() != "parent id is zero" {
			t.Fatalf("want parent id is zero, got %v", err)
		}
	})

	t.Run("unknown type", func(t *testing.T) {
		_, err := testSvc.CreateLocation(ctx, input.CreateLocationInput{
			ParentID: root.ID,
			Type:     "not-a-type",
			Name:     "x",
		})
		if err == nil {
			t.Fatal("expected type error")
		}
	})

	t.Run("zero subtype id", func(t *testing.T) {
		zero := types.ID(0)
		_, err := testSvc.CreateLocation(ctx, input.CreateLocationInput{
			ParentID:  root.ID,
			Type:      "region",
			SubtypeID: &zero,
			Name:      "x",
		})
		if err == nil || err.Error() != "subtype id is zero" {
			t.Fatalf("want subtype id is zero, got %v", err)
		}
	})
}

func TestCreateLocation_CompleteError(t *testing.T) {
	resetDB(t)
	root := seedRoot(t, "world")
	rec := &completeRecorder{err: errors.New("callback failed")}
	ctx, cancel := newExecCtx(t, rec)
	defer cancel()

	_, err := testSvc.CreateLocation(ctx, input.CreateLocationInput{
		ParentID: root.ID,
		Type:     "region",
		Name:     "X",
	})
	if err == nil || err.Error() != "callback failed" {
		t.Fatalf("want callback failed, got %v", err)
	}

	var count int64
	if err := testConn.DB().Table("locations").Where("name = ?", "X").Count(&count).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Fatalf("create should roll back when Complete fails, count=%d", count)
	}
}

func TestGetLocation(t *testing.T) {
	resetDB(t)
	root := seedRoot(t, "world")
	child := seedChild(t, root.ID, "child")
	rec := &completeRecorder{}
	ctx, cancel := newExecCtx(t, rec)
	defer cancel()

	out, err := testSvc.GetLocation(ctx, child.ID)
	if err != nil {
		t.Fatalf("GetLocation: %v", err)
	}
	if out.ID != child.ID || out.ParentID != root.ID || out.Name != "child" || out.Type != "region" {
		t.Fatalf("unexpected output: %+v", out)
	}
	if len(rec.calls) != 1 || rec.calls[0].Action != libjournal.Action_Get {
		t.Fatalf("unexpected complete calls: %#v", rec.calls)
	}
}

func TestGetLocation_NotFound(t *testing.T) {
	resetDB(t)
	ctx, cancel := newExecCtx(t, nil)
	defer cancel()

	_, err := testSvc.GetLocation(ctx, 999)
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected ErrRecordNotFound, got %v", err)
	}
}

func TestFindChildLocations(t *testing.T) {
	resetDB(t)
	root := seedRoot(t, "root")
	a := seedChild(t, root.ID, "A")
	_ = seedChild(t, root.ID, "B")
	_ = seedChild(t, a.ID, "A1")

	rec := &completeRecorder{}
	ctx, cancel := newExecCtx(t, rec)
	defer cancel()

	depth0, err := testSvc.FindChildLocations(ctx, root.ID, 0)
	if err != nil {
		t.Fatalf("depth 0: %v", err)
	}
	if len(depth0) != 0 {
		t.Fatalf("depth 0 want empty, got %#v", depth0)
	}

	depth1, err := testSvc.FindChildLocations(ctx, root.ID, 1)
	if err != nil {
		t.Fatalf("depth 1: %v", err)
	}
	if len(depth1[1]) != 2 {
		t.Fatalf("depth 1 want 2, got %#v", depth1)
	}

	depth2, err := testSvc.FindChildLocations(ctx, root.ID, 2)
	if err != nil {
		t.Fatalf("depth 2: %v", err)
	}
	if len(depth2[1]) != 2 || len(depth2[2]) != 1 {
		t.Fatalf("depth 2 want 2+1, got %#v", depth2)
	}
	if depth2[2][0].Name != "A1" {
		t.Fatalf("want grandchild A1, got %+v", depth2[2][0])
	}
	if len(rec.calls) < 1 {
		t.Fatal("expected Complete to be called")
	}
}

func TestFindChildLocations_NotFound(t *testing.T) {
	resetDB(t)
	ctx, cancel := newExecCtx(t, nil)
	defer cancel()

	_, err := testSvc.FindChildLocations(ctx, 999, 1)
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected ErrRecordNotFound, got %v", err)
	}
}

func TestMoveLocation(t *testing.T) {
	resetDB(t)
	root := seedRoot(t, "root")
	other := seedRoot(t, "other")
	child := seedChild(t, root.ID, "child")

	rec := &completeRecorder{}
	ctx, cancel := newExecCtx(t, rec)
	defer cancel()

	if err := testSvc.MoveLocation(ctx, child.ID, other.ID); err != nil {
		t.Fatalf("MoveLocation: %v", err)
	}

	got, err := testRepo.GetLocation(ctx, child.ID)
	if err != nil {
		t.Fatalf("GetLocation: %v", err)
	}
	if got.ParentID != other.ID {
		t.Fatalf("want parent %d, got %d", other.ID, got.ParentID)
	}
	if len(rec.calls) != 1 || rec.calls[0].Action != libjournal.Action_Patch {
		t.Fatalf("unexpected complete calls: %#v", rec.calls)
	}
}

func TestMoveLocation_RejectsCycle(t *testing.T) {
	resetDB(t)
	root := seedRoot(t, "root")
	child := seedChild(t, root.ID, "child")

	ctx, cancel := newExecCtx(t, nil)
	defer cancel()

	err := testSvc.MoveLocation(ctx, root.ID, child.ID)
	if err == nil || err.Error() != "location would create a cycle" {
		t.Fatalf("want cycle error, got %v", err)
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
	if len(rec.calls) != 1 || rec.calls[0].Action != libjournal.Action_Patch {
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

	details := types.JSON(`{"a":1}`)
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

	err := testSvc.UpdateLocationDetails(ctx, 999, types.JSON(`{}`))
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
