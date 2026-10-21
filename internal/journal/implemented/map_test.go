package implemented_test

import (
	"context"
	"errors"
	"testing"

	"github.com/dejitarudemon/mundarium/internal/journal"
	"github.com/dejitarudemon/mundarium/internal/journal/implemented"
	libjournal "github.com/dejitarudemon/mundarium/internal/libs/journal"
	"github.com/google/uuid"
)

func newID() libjournal.RequestID {
	return libjournal.RequestID(uuid.New())
}

func TestMapJournal_RegisterAndFind(t *testing.T) {
	ctx := context.Background()
	mj := implemented.NewMapJournal()
	id := newID()

	complete, fail, err := mj.Register(ctx, id, libjournal.Action_Create)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if complete == nil || fail == nil {
		t.Fatal("expected Complete and Fail callbacks")
	}

	got, err := mj.Find(ctx, id)
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if got.ID != id || got.Action != libjournal.Action_Create || got.Status != libjournal.Status_Running {
		t.Fatalf("unexpected request: %+v", got)
	}
	if got.Result != nil || got.Error != nil {
		t.Fatalf("running request must have empty outcome: %+v", got)
	}
}

func TestMapJournal_Register_AlreadyRegistered(t *testing.T) {
	ctx := context.Background()
	mj := implemented.NewMapJournal()
	id := newID()

	if _, _, err := mj.Register(ctx, id, libjournal.Action_Get); err != nil {
		t.Fatalf("first Register: %v", err)
	}

	_, _, err := mj.Register(ctx, id, libjournal.Action_Create)
	var already journal.AlreadyRegistered
	if !errors.As(err, &already) {
		t.Fatalf("want AlreadyRegistered, got %v", err)
	}
	if already.Request.ID != id {
		t.Fatalf("want request id %s, got %s", id, already.Request.ID)
	}
	if already.TracebackID() == uuid.Nil {
		t.Fatal("expected non-nil traceback id")
	}
}

func TestMapJournal_Complete(t *testing.T) {
	ctx := context.Background()
	mj := implemented.NewMapJournal()
	id := newID()

	complete, _, err := mj.Register(ctx, id, libjournal.Action_Create)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	result := journal.Result{
		Action: libjournal.Action_Create,
		Entity: "LOCATION",
		Data:   "ok",
	}
	if err := complete(ctx, id, result); err != nil {
		t.Fatalf("Complete: %v", err)
	}

	got, err := mj.Find(ctx, id)
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if got.Status != libjournal.Status_Completed {
		t.Fatalf("want completed, got %v", got.Status)
	}
	if got.Result == nil || got.Result.Entity != "LOCATION" || got.Result.Data != "ok" {
		t.Fatalf("unexpected result: %#v", got.Result)
	}
}

func TestMapJournal_Complete_NotFound(t *testing.T) {
	ctx := context.Background()
	mj := implemented.NewMapJournal()

	err := mj.Complete(ctx, newID(), journal.Result{Action: libjournal.Action_Get})
	var notFound journal.NotFound
	if !errors.As(err, &notFound) {
		t.Fatalf("want NotFound, got %v", err)
	}
}

func TestMapJournal_Complete_AfterFail(t *testing.T) {
	ctx := context.Background()
	mj := implemented.NewMapJournal()
	id := newID()

	_, fail, err := mj.Register(ctx, id, libjournal.Action_Patch)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if err := fail(ctx, id, errors.New("boom")); err != nil {
		t.Fatalf("Fail: %v", err)
	}

	err = mj.Complete(ctx, id, journal.Result{Action: libjournal.Action_Patch})
	var conflict journal.StatusConflict
	if !errors.As(err, &conflict) {
		t.Fatalf("want StatusConflict, got %v", err)
	}
	if conflict.Got != libjournal.Status_Completed || conflict.Actual != libjournal.Status_Failed {
		t.Fatalf("unexpected conflict: %+v", conflict)
	}
}

func TestMapJournal_Fail(t *testing.T) {
	ctx := context.Background()
	mj := implemented.NewMapJournal()
	id := newID()

	_, fail, err := mj.Register(ctx, id, libjournal.Action_Update)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	cause := errors.New("failed op")
	if err := fail(ctx, id, cause); err != nil {
		t.Fatalf("Fail: %v", err)
	}

	got, err := mj.Find(ctx, id)
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if got.Status != libjournal.Status_Failed {
		t.Fatalf("want failed, got %v", got.Status)
	}
	if got.Error == nil || got.Error.Error() != "failed op" {
		t.Fatalf("unexpected error: %v", got.Error)
	}
	if got.Result != nil {
		t.Fatalf("failed request must not keep result: %#v", got.Result)
	}
}

func TestMapJournal_Fail_NotFound(t *testing.T) {
	ctx := context.Background()
	mj := implemented.NewMapJournal()

	err := mj.Fail(ctx, newID(), errors.New("x"))
	var notFound journal.NotFound
	if !errors.As(err, &notFound) {
		t.Fatalf("want NotFound, got %v", err)
	}
}

func TestMapJournal_Fail_AlreadyFailed(t *testing.T) {
	ctx := context.Background()
	mj := implemented.NewMapJournal()
	id := newID()

	_, fail, err := mj.Register(ctx, id, libjournal.Action_Delete)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if err := fail(ctx, id, errors.New("first")); err != nil {
		t.Fatalf("first Fail: %v", err)
	}

	err = fail(ctx, id, errors.New("second"))
	var conflict journal.StatusConflict
	if !errors.As(err, &conflict) {
		t.Fatalf("want StatusConflict, got %v", err)
	}
	if conflict.Got != libjournal.Status_Failed || conflict.Actual != libjournal.Status_Failed {
		t.Fatalf("unexpected conflict: %+v", conflict)
	}
}

func TestMapJournal_Find_NotFound(t *testing.T) {
	ctx := context.Background()
	mj := implemented.NewMapJournal()

	_, err := mj.Find(ctx, newID())
	var notFound journal.NotFound
	if !errors.As(err, &notFound) {
		t.Fatalf("want NotFound, got %v", err)
	}
}

func TestMapJournal_Delete(t *testing.T) {
	ctx := context.Background()
	mj := implemented.NewMapJournal()
	id := newID()

	if _, _, err := mj.Register(ctx, id, libjournal.Action_Create); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if err := mj.Delete(ctx, id); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err := mj.Find(ctx, id)
	var notFound journal.NotFound
	if !errors.As(err, &notFound) {
		t.Fatalf("want NotFound after delete, got %v", err)
	}

	// Missing id is ignored.
	if err := mj.Delete(ctx, id); err != nil {
		t.Fatalf("Delete missing: %v", err)
	}
}

func TestMapJournal_ImplementsJournal(t *testing.T) {
	var _ journal.Journal = implemented.NewMapJournal()
}
