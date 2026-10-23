package implemented_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dejitarudemon/mundarium/internal/journal"
	"github.com/dejitarudemon/mundarium/internal/journal/implemented"
	libjournal "github.com/dejitarudemon/mundarium/internal/libs/journal"
	"github.com/google/uuid"
)

var idSeq atomic.Uint64

func newID() libjournal.RequestID {
	return libjournal.RequestID(fmt.Sprintf("req-%d", idSeq.Add(1)))
}

func newGroup() libjournal.RequestID {
	return libjournal.RequestID(fmt.Sprintf("grp-%d", idSeq.Add(1)))
}

func TestMapJournal_RegisterAndFind(t *testing.T) {
	ctx := context.Background()
	mj := implemented.NewMapJournal()
	id := newID()
	group := newGroup()

	argsHash := []byte{0x01, 0x02, 0x03}
	complete, fail, err := mj.Register(ctx, id, group, "create_location", argsHash)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if complete == nil || fail == nil {
		t.Fatal("expected Complete and Fail callbacks")
	}

	got, err := mj.Find(ctx, id, group)
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if got.ID != id || got.GroupID != group || got.Tool != "create_location" || got.Status != libjournal.Status_Running {
		t.Fatalf("unexpected request: %+v", got)
	}
	if got.CreatedAt.IsZero() {
		t.Fatal("Register must set CreatedAt")
	}
	if string(got.ArgsHash) != string(argsHash) {
		t.Fatalf("want args hash %v, got %v", argsHash, got.ArgsHash)
	}
	got.ArgsHash[0] = 0xff
	got2, err := mj.Find(ctx, id, group)
	if err != nil {
		t.Fatalf("Find after mutate: %v", err)
	}
	if got2.ArgsHash[0] != 0x01 {
		t.Fatal("Find must return a copy of ArgsHash")
	}
	if got.Result != (libjournal.Result{}) || got.Error != nil {
		t.Fatalf("running request must have empty outcome: %+v", got)
	}
}

func TestMapJournal_Register_AlreadyRegistered(t *testing.T) {
	ctx := context.Background()
	mj := implemented.NewMapJournal()
	id := newID()
	group := newGroup()

	if _, _, err := mj.Register(ctx, id, group, "get_location", nil); err != nil {
		t.Fatalf("first Register: %v", err)
	}

	_, _, err := mj.Register(ctx, id, group, "create_location", nil)
	var already journal.AlreadyRegistered
	if !errors.As(err, &already) {
		t.Fatalf("want AlreadyRegistered, got %v", err)
	}
	if already.Request.ID != id || already.Request.GroupID != group {
		t.Fatalf("want request %s/%s, got %s/%s", id, group, already.Request.ID, already.Request.GroupID)
	}
	if already.Request.Tool != "get_location" {
		t.Fatalf("want original tool get_location, got %q", already.Request.Tool)
	}
	if already.TracebackID() == uuid.Nil {
		t.Fatal("expected non-nil traceback id")
	}
	msg := already.Error()
	if !strings.Contains(msg, string(id)) || !strings.Contains(msg, string(group)) {
		t.Fatalf("AlreadyRegistered message missing ids: %q", msg)
	}
}

func TestMapJournal_Register_SameIDDifferentGroups(t *testing.T) {
	ctx := context.Background()
	mj := implemented.NewMapJournal()
	id := newID()
	g1 := newGroup()
	g2 := newGroup()

	if _, _, err := mj.Register(ctx, id, g1, "create_location", nil); err != nil {
		t.Fatalf("Register g1: %v", err)
	}
	if _, _, err := mj.Register(ctx, id, g2, "create_location", nil); err != nil {
		t.Fatalf("Register g2: %v", err)
	}
}

func TestMapJournal_Complete(t *testing.T) {
	ctx := context.Background()
	mj := implemented.NewMapJournal()
	id := newID()
	group := newGroup()

	complete, _, err := mj.Register(ctx, id, group, "create_location", nil)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	deadline := time.Now().UTC().Add(time.Hour).Truncate(time.Second)
	if err := mj.Close(ctx, group, deadline); err != nil {
		t.Fatalf("Close: %v", err)
	}

	result := libjournal.Result{
		Action: libjournal.Action_Create,
		Entity: "LOCATION",
		Data:   "ok",
	}
	if err := complete(ctx, id, group, result, nil); err != nil {
		t.Fatalf("Complete: %v", err)
	}

	got, err := mj.Find(ctx, id, group)
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if got.Status != libjournal.Status_Completed {
		t.Fatalf("want completed, got %v", got.Status)
	}
	if got.Tool != "create_location" {
		t.Fatalf("want tool preserved, got %q", got.Tool)
	}
	if got.Result.Entity != "LOCATION" || got.Result.Data != "ok" {
		t.Fatalf("unexpected result: %#v", got.Result)
	}
	if got.ExpiresAt == nil || !got.ExpiresAt.Equal(deadline) {
		t.Fatalf("Complete must preserve ExpiresAt, got %v", got.ExpiresAt)
	}
	if got.CreatedAt.IsZero() {
		t.Fatal("Complete must preserve CreatedAt")
	}
}

func TestMapJournal_Complete_NotFound(t *testing.T) {
	ctx := context.Background()
	mj := implemented.NewMapJournal()
	id := newID()
	group := newGroup()

	err := mj.Complete(ctx, id, group, libjournal.Result{Action: libjournal.Action_Get}, nil)
	var notFound journal.NotFound
	if !errors.As(err, &notFound) {
		t.Fatalf("want NotFound, got %v", err)
	}
	if notFound.RequestID != id || notFound.GroupRequestID != group {
		t.Fatalf("unexpected not found: %+v", notFound)
	}
}

func TestMapJournal_Complete_AfterFail(t *testing.T) {
	ctx := context.Background()
	mj := implemented.NewMapJournal()
	id := newID()
	group := newGroup()

	_, fail, err := mj.Register(ctx, id, group, "patch_location", nil)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if err := fail(ctx, id, group, errors.New("boom"), nil); err != nil {
		t.Fatalf("Fail: %v", err)
	}

	err = mj.Complete(ctx, id, group, libjournal.Result{Action: libjournal.Action_Patch}, nil)
	var conflict journal.StatusConflict
	if !errors.As(err, &conflict) {
		t.Fatalf("want StatusConflict, got %v", err)
	}
	if conflict.Got != libjournal.Status_Completed || conflict.Actual != libjournal.Status_Failed {
		t.Fatalf("unexpected conflict: %+v", conflict)
	}
}

func TestMapJournal_Complete_AlreadyCompleted(t *testing.T) {
	ctx := context.Background()
	mj := implemented.NewMapJournal()
	id := newID()
	group := newGroup()

	complete, _, err := mj.Register(ctx, id, group, "create_location", nil)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	first := libjournal.Result{Action: libjournal.Action_Create, Entity: "LOCATION", Data: "first"}
	if err := complete(ctx, id, group, first, nil); err != nil {
		t.Fatalf("first Complete: %v", err)
	}

	err = complete(ctx, id, group, libjournal.Result{Action: libjournal.Action_Create, Entity: "LOCATION", Data: "second"}, nil)
	var conflict journal.StatusConflict
	if !errors.As(err, &conflict) {
		t.Fatalf("want StatusConflict, got %v", err)
	}

	got, err := mj.Find(ctx, id, group)
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if got.Result.Data != "first" {
		t.Fatalf("completed request must keep first result: %#v", got.Result)
	}
}

func TestMapJournal_Fail(t *testing.T) {
	ctx := context.Background()
	mj := implemented.NewMapJournal()
	id := newID()
	group := newGroup()

	_, fail, err := mj.Register(ctx, id, group, "update_location", nil)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	cause := errors.New("failed op")
	if err := fail(ctx, id, group, cause, nil); err != nil {
		t.Fatalf("Fail: %v", err)
	}

	got, err := mj.Find(ctx, id, group)
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if got.Status != libjournal.Status_Failed {
		t.Fatalf("want failed, got %v", got.Status)
	}
	if got.Error == nil || got.Error.Error() != "failed op" {
		t.Fatalf("unexpected error: %v", got.Error)
	}
}

func TestMapJournal_Fail_NotFound(t *testing.T) {
	ctx := context.Background()
	mj := implemented.NewMapJournal()
	id := newID()
	group := newGroup()

	err := mj.Fail(ctx, id, group, errors.New("x"), nil)
	var notFound journal.NotFound
	if !errors.As(err, &notFound) {
		t.Fatalf("want NotFound, got %v", err)
	}
	if notFound.RequestID != id || notFound.GroupRequestID != group {
		t.Fatalf("unexpected not found: %+v", notFound)
	}
}

func TestMapJournal_Fail_AlreadyFailed(t *testing.T) {
	ctx := context.Background()
	mj := implemented.NewMapJournal()
	id := newID()
	group := newGroup()

	_, fail, err := mj.Register(ctx, id, group, "delete_location", nil)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if err := fail(ctx, id, group, errors.New("first"), nil); err != nil {
		t.Fatalf("first Fail: %v", err)
	}

	err = fail(ctx, id, group, errors.New("second"), nil)
	var conflict journal.StatusConflict
	if !errors.As(err, &conflict) {
		t.Fatalf("want StatusConflict, got %v", err)
	}

	got, err := mj.Find(ctx, id, group)
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if got.Error == nil || got.Error.Error() != "first" {
		t.Fatalf("failed request must keep first error: %v", got.Error)
	}
}

func TestMapJournal_Fail_AfterComplete(t *testing.T) {
	ctx := context.Background()
	mj := implemented.NewMapJournal()
	id := newID()
	group := newGroup()

	complete, fail, err := mj.Register(ctx, id, group, "update_location", nil)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if err := complete(ctx, id, group, libjournal.Result{Action: libjournal.Action_Update}, nil); err != nil {
		t.Fatalf("Complete: %v", err)
	}

	err = fail(ctx, id, group, errors.New("late"), nil)
	var conflict journal.StatusConflict
	if !errors.As(err, &conflict) {
		t.Fatalf("want StatusConflict, got %v", err)
	}
	if conflict.Got != libjournal.Status_Failed || conflict.Actual != libjournal.Status_Completed {
		t.Fatalf("unexpected conflict: %+v", conflict)
	}
}

func TestMapJournal_Find_NotFound(t *testing.T) {
	ctx := context.Background()
	mj := implemented.NewMapJournal()
	id := newID()
	group := newGroup()

	_, err := mj.Find(ctx, id, group)
	var notFound journal.NotFound
	if !errors.As(err, &notFound) {
		t.Fatalf("want NotFound, got %v", err)
	}
	if notFound.RequestID != id || notFound.GroupRequestID != group {
		t.Fatalf("unexpected not found: %+v", notFound)
	}
}

func TestMapJournal_Close(t *testing.T) {
	ctx := context.Background()
	mj := implemented.NewMapJournal()
	group := newGroup()
	a := newID()
	b := newID()

	if err := mj.Close(ctx, group, time.Now()); err != nil {
		t.Fatalf("Close missing group: %v", err)
	}

	if _, _, err := mj.Register(ctx, a, group, "create_location", nil); err != nil {
		t.Fatalf("Register a: %v", err)
	}
	if _, _, err := mj.Register(ctx, b, group, "find_locations", nil); err != nil {
		t.Fatalf("Register b: %v", err)
	}

	deadline := time.Now().UTC().Add(time.Hour).Truncate(time.Second)
	if err := mj.Close(ctx, group, deadline); err != nil {
		t.Fatalf("Close: %v", err)
	}
	got, err := mj.Find(ctx, a, group)
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if got.ExpiresAt == nil || !got.ExpiresAt.Equal(deadline) {
		t.Fatalf("want ExpiresAt %v, got %v", deadline, got.ExpiresAt)
	}
	got, err = mj.Find(ctx, b, group)
	if err != nil {
		t.Fatalf("Find b: %v", err)
	}
	if got.ExpiresAt == nil || !got.ExpiresAt.Equal(deadline) {
		t.Fatalf("want ExpiresAt %v on b, got %v", deadline, got.ExpiresAt)
	}

	if _, _, err := mj.Register(ctx, newID(), group, "create_location", nil); !errors.Is(err, journal.ErrGroupIsClosed) {
		t.Fatalf("Register after Close: want ErrGroupIsClosed, got %v", err)
	}
}

func TestMapJournal_IsGroupFree(t *testing.T) {
	ctx := context.Background()
	mj := implemented.NewMapJournal()
	group := newGroup()
	id := newID()

	free, err := mj.IsGroupFree(ctx, group)
	if err != nil || !free {
		t.Fatalf("missing group: want true nil, got %v %v", free, err)
	}

	if _, _, err := mj.Register(ctx, id, group, "create_location", nil); err != nil {
		t.Fatalf("Register: %v", err)
	}
	free, err = mj.IsGroupFree(ctx, group)
	if err != nil || free {
		t.Fatalf("non-empty group: want false nil, got %v %v", free, err)
	}

	past := time.Now().UTC().Add(-time.Hour)
	if err := mj.Close(ctx, group, past); err != nil {
		t.Fatalf("Close: %v", err)
	}
	free, err = mj.IsGroupFree(ctx, group)
	if err != nil || free {
		t.Fatalf("expired requests still occupy group: want false nil, got %v %v", free, err)
	}

	if err := mj.DeleteAllExpired(ctx, 0); err != nil {
		t.Fatalf("DeleteAllExpired: %v", err)
	}
	free, err = mj.IsGroupFree(ctx, group)
	if err != nil || !free {
		t.Fatalf("after DeleteAllExpired: want true nil, got %v %v", free, err)
	}
}

func TestMapJournal_DeleteAllExpired_Soft(t *testing.T) {
	ctx := context.Background()
	mj := implemented.NewMapJournal()

	keepGroup := newGroup()
	dropGroup := newGroup()
	keepID := newID()
	dropID := newID()

	if _, _, err := mj.Register(ctx, keepID, keepGroup, "create_location", nil); err != nil {
		t.Fatalf("Register keep: %v", err)
	}
	if _, _, err := mj.Register(ctx, dropID, dropGroup, "create_location", nil); err != nil {
		t.Fatalf("Register drop: %v", err)
	}

	if err := mj.Close(ctx, keepGroup, time.Now().UTC().Add(time.Hour)); err != nil {
		t.Fatalf("Close keep: %v", err)
	}
	if err := mj.Close(ctx, dropGroup, time.Now().UTC().Add(-time.Hour)); err != nil {
		t.Fatalf("Close drop: %v", err)
	}

	if err := mj.DeleteAllExpired(ctx, 0); err != nil {
		t.Fatalf("DeleteAllExpired: %v", err)
	}

	if _, err := mj.Find(ctx, keepID, keepGroup); err != nil {
		t.Fatalf("keep group must remain: %v", err)
	}
	if _, err := mj.Find(ctx, dropID, dropGroup); err == nil {
		t.Fatal("soft-expired group must be removed")
	}
}

func TestMapJournal_DeleteAllExpired_HardTTL(t *testing.T) {
	ctx := context.Background()
	mj := implemented.NewMapJournal()

	keepGroup := newGroup()
	dropGroup := newGroup()
	keepID := newID()
	dropID := newID()

	if _, _, err := mj.Register(ctx, dropID, dropGroup, "create_location", nil); err != nil {
		t.Fatalf("Register drop: %v", err)
	}
	time.Sleep(5 * time.Millisecond)
	if _, _, err := mj.Register(ctx, keepID, keepGroup, "create_location", nil); err != nil {
		t.Fatalf("Register keep: %v", err)
	}

	if err := mj.DeleteAllExpired(ctx, 3*time.Millisecond); err != nil {
		t.Fatalf("DeleteAllExpired: %v", err)
	}

	if _, err := mj.Find(ctx, keepID, keepGroup); err != nil {
		t.Fatalf("fresh group must remain: %v", err)
	}
	if _, err := mj.Find(ctx, dropID, dropGroup); err == nil {
		t.Fatal("hardTTL-expired group must be removed")
	}
}

func TestMapJournal_DeleteAllExpired_ContextCanceled(t *testing.T) {
	mj := implemented.NewMapJournal()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := mj.DeleteAllExpired(ctx, time.Hour)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("want context.Canceled, got %v", err)
	}
}

func TestMapJournal_ImplementsJournal(t *testing.T) {
	var _ journal.Journal = implemented.NewMapJournal()
}
