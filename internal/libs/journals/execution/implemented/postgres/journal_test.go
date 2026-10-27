package postgres

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	journal "github.com/dejitarudemon/mundarium/internal/libs/journals/execution"
)

var idSeq atomic.Uint64

func newID() journal.ID {
	return journal.ID(fmt.Sprintf("req-%d", idSeq.Add(1)))
}

func newGroup() journal.ID {
	return journal.ID(fmt.Sprintf("grp-%d", idSeq.Add(1)))
}

func TestNewPostgresJournal_NilConnection(t *testing.T) {
	_, err := NewPostgresJournal(nil)
	if err == nil || err.Error() != "connection is nil" {
		t.Fatalf("want connection is nil, got %v", err)
	}
}

func TestPostgresJournal_RegisterAndFind(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	j := newJournal(t)
	id, group := newID(), newGroup()
	hash := []byte{1, 2, 3}

	complete, fail, err := j.Register(ctx, id, group, "create_location", hash)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if complete == nil || fail == nil {
		t.Fatal("want complete and fail callbacks")
	}

	got, err := j.Find(ctx, id, group)
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if got.ID != id || got.GroupID != group || got.Tool != "create_location" || got.Status != journal.Status_Running {
		t.Fatalf("unexpected request: %+v", got)
	}
	if string(got.ArgsHash) != string(hash) {
		t.Fatalf("args hash: want %v, got %v", hash, got.ArgsHash)
	}
}

func TestPostgresJournal_Register_AlreadyRegistered(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	j := newJournal(t)
	id, group := newID(), newGroup()

	if _, _, err := j.Register(ctx, id, group, "create_location", nil); err != nil {
		t.Fatalf("Register: %v", err)
	}

	_, _, err := j.Register(ctx, id, group, "create_location", nil)
	var already journal.AlreadyRegistered
	if !errors.As(err, &already) {
		t.Fatalf("want AlreadyRegistered, got %v", err)
	}
	if already.Request.ID != id || already.Request.GroupID != group {
		t.Fatalf("unexpected already request: %+v", already.Request)
	}
}

func TestPostgresJournal_CompleteAndFail(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	j := newJournal(t)

	t.Run("complete", func(t *testing.T) {
		id, group := newID(), newGroup()
		complete, _, err := j.Register(ctx, id, group, "create_location", nil)
		if err != nil {
			t.Fatalf("Register: %v", err)
		}
		result := journal.Result{Action: journal.Action_Create, Entity: "LOCATION", Data: "ok"}
		if err := complete(ctx, id, group, result, nil); err != nil {
			t.Fatalf("Complete: %v", err)
		}
		got, err := j.Find(ctx, id, group)
		if err != nil {
			t.Fatalf("Find: %v", err)
		}
		if got.Status != journal.Status_Completed || got.Result.Entity != "LOCATION" {
			t.Fatalf("unexpected completed: %+v", got)
		}
	})

	t.Run("fail", func(t *testing.T) {
		id, group := newID(), newGroup()
		_, fail, err := j.Register(ctx, id, group, "create_location", nil)
		if err != nil {
			t.Fatalf("Register: %v", err)
		}
		if err := fail(ctx, id, group, errors.New("boom"), nil); err != nil {
			t.Fatalf("Fail: %v", err)
		}
		got, err := j.Find(ctx, id, group)
		if err != nil {
			t.Fatalf("Find: %v", err)
		}
		if got.Status != journal.Status_Failed || got.Error.Error() != "boom" {
			t.Fatalf("unexpected failed: %+v", got)
		}
	})
}

func TestPostgresJournal_StatusConflicts(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	j := newJournal(t)
	id, group := newID(), newGroup()

	complete, fail, err := j.Register(ctx, id, group, "create_location", nil)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if err := complete(ctx, id, group, journal.Result{Action: journal.Action_Create}, nil); err != nil {
		t.Fatalf("Complete: %v", err)
	}

	err = complete(ctx, id, group, journal.Result{}, nil)
	var conflict journal.StatusConflict
	if !errors.As(err, &conflict) || conflict.Got != journal.Status_Completed {
		t.Fatalf("want StatusConflict completed, got %v", err)
	}

	err = fail(ctx, id, group, errors.New("x"), nil)
	if !errors.As(err, &conflict) || conflict.Got != journal.Status_Failed {
		t.Fatalf("want StatusConflict failed, got %v", err)
	}
}

func TestPostgresJournal_CloseAndGroupClosed(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	j := newJournal(t)
	group := newGroup()
	a, b := newID(), newID()

	if err := j.Close(ctx, group, time.Now()); err != nil {
		t.Fatalf("Close missing: %v", err)
	}

	if _, _, err := j.Register(ctx, a, group, "create_location", nil); err != nil {
		t.Fatalf("Register a: %v", err)
	}
	if _, _, err := j.Register(ctx, b, group, "find_locations", nil); err != nil {
		t.Fatalf("Register b: %v", err)
	}

	deadline := time.Now().UTC().Add(time.Hour).Truncate(time.Second)
	if err := j.Close(ctx, group, deadline); err != nil {
		t.Fatalf("Close: %v", err)
	}

	got, err := j.Find(ctx, a, group)
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if got.ExpiresAt == nil || !got.ExpiresAt.Equal(deadline) {
		t.Fatalf("want ExpiresAt %v, got %v", deadline, got.ExpiresAt)
	}

	if _, _, err := j.Register(ctx, newID(), group, "create_location", nil); !errors.Is(err, journal.ErrGroupIsClosed) {
		t.Fatalf("want ErrGroupIsClosed, got %v", err)
	}
}

func TestPostgresJournal_IsGroupFreeAndDeleteExpired(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	j := newJournal(t)
	group := newGroup()
	id := newID()

	free, err := j.IsGroupFree(ctx, group)
	if err != nil || !free {
		t.Fatalf("missing group: want free, got %v %v", free, err)
	}

	if _, _, err := j.Register(ctx, id, group, "create_location", nil); err != nil {
		t.Fatalf("Register: %v", err)
	}
	free, err = j.IsGroupFree(ctx, group)
	if err != nil || free {
		t.Fatalf("non-empty: want busy, got %v %v", free, err)
	}

	past := time.Now().UTC().Add(-time.Hour)
	if err := j.Close(ctx, group, past); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := j.DeleteAllExpired(ctx, 0); err != nil {
		t.Fatalf("DeleteAllExpired: %v", err)
	}
	free, err = j.IsGroupFree(ctx, group)
	if err != nil || !free {
		t.Fatalf("after delete: want free, got %v %v", free, err)
	}
}

func TestPostgresJournal_FailNotFoundDoesNotUseEmergency(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	j := newJournal(t)

	id, group := newID(), newGroup()
	err := j.Fail(ctx, id, group, errors.New("offline"), nil)
	if _, ok := errors.AsType[journal.NotFound](err); !ok {
		t.Fatalf("want NotFound, got %v", err)
	}

	_, err = j.Find(ctx, id, group)
	if _, ok := errors.AsType[journal.NotFound](err); !ok {
		t.Fatalf("want NotFound after Fail-not-found, got %v", err)
	}
}

func TestPostgresJournal_ImplementsJournal(t *testing.T) {
	var _ journal.Journal = newJournal(t)
}
