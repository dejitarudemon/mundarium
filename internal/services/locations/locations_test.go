package locations_test

import (
	"errors"
	"testing"

	journal "github.com/dejitarudemon/mundarium/internal/libs/journals/execution"
	"github.com/dejitarudemon/mundarium/internal/repositories"
	svclocations "github.com/dejitarudemon/mundarium/internal/services/locations"
	"gorm.io/gorm"
)

func TestCreateLocation(t *testing.T) {
	resetDB(t)
	root := seedRoot(t, "world")
	rec := &completeRecorder{}
	ctx, cancel := newExecCtx(t, rec)
	defer cancel()

	out, err := testSvc.CreateLocation(ctx, svclocations.CreateLocationInput{
		ParentID:    new(root.ID),
		Type:        "region",
		Name:        "North",
		Description: "cold",
		Details:     repositories.JSON(`{"climate":"arctic"}`),
	})
	if err != nil {
		t.Fatalf("CreateLocation: %v", err)
	}
	if out.ID == 0 || out.ParentID == nil || *out.ParentID != root.ID || out.Name != "North" {
		t.Fatalf("unexpected output: %+v", out)
	}
	if len(rec.calls) != 1 || rec.calls[0].Action != journal.Action_Create || rec.calls[0].Entity != "LOCATION" {
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
	rec := &completeRecorder{}
	ctx, cancel := newExecCtx(t, rec)
	defer cancel()

	t.Run("zero parent", func(t *testing.T) {
		_, err := testSvc.CreateLocation(ctx, svclocations.CreateLocationInput{
			ParentID: new(repositories.ID(0)),
			Type:     "region",
			Name:     "x",
		})
		if err == nil || err.Error() != "parent id is zero" {
			t.Fatalf("want parent id is zero, got %v", err)
		}
		if len(rec.failCalls) == 0 {
			t.Fatal("want Fail called on validation error")
		}
	})

	t.Run("unknown type", func(t *testing.T) {
		before := len(rec.failCalls)
		_, err := testSvc.CreateLocation(ctx, svclocations.CreateLocationInput{
			ParentID: new(root.ID),
			Type:     "not-a-type",
			Name:     "x",
		})
		if err == nil {
			t.Fatal("expected type error")
		}
		if len(rec.failCalls) <= before {
			t.Fatal("want Fail called on validation error")
		}
	})

	t.Run("zero subtype id", func(t *testing.T) {
		before := len(rec.failCalls)
		_, err := testSvc.CreateLocation(ctx, svclocations.CreateLocationInput{
			ParentID:  new(root.ID),
			Type:      "region",
			SubtypeID: new(repositories.ID(0)),
			Name:      "x",
		})
		if err == nil || err.Error() != "subtype id is zero" {
			t.Fatalf("want subtype id is zero, got %v", err)
		}
		if len(rec.failCalls) <= before {
			t.Fatal("want Fail called on validation error")
		}
	})
}

func TestCreateLocation_CompleteError(t *testing.T) {
	resetDB(t)
	root := seedRoot(t, "world")
	rec := &completeRecorder{err: errors.New("callback failed")}
	ctx, cancel := newExecCtx(t, rec)
	defer cancel()

	_, err := testSvc.CreateLocation(ctx, svclocations.CreateLocationInput{
		ParentID: new(root.ID),
		Type:     "region",
		Name:     "X",
	})
	if err == nil || err.Error() != "callback failed" {
		t.Fatalf("want callback failed, got %v", err)
	}
	if len(rec.failCalls) != 1 || rec.failCalls[0].Error() != "callback failed" {
		t.Fatalf("want Fail called once with callback failed, got %#v", rec.failCalls)
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
	if out.ID != child.ID || out.ParentID == nil || *out.ParentID != root.ID || out.Name != "child" || out.Type != "region" {
		t.Fatalf("unexpected output: %+v", out)
	}
	if len(rec.calls) != 1 || rec.calls[0].Action != journal.Action_Get {
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
