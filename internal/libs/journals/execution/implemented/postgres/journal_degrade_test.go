package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	journal "github.com/dejitarudemon/mundarium/internal/libs/journals/execution"
)

func TestPostgresJournal_FailOnClosedConnection_UsesEmergency(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	conn := openIsolatedConn(t)
	j, err := NewPostgresJournal(conn)
	if err != nil {
		t.Fatalf("NewPostgresJournal: %v", err)
	}

	id, group := newID(), newGroup()
	if _, _, err := j.Register(ctx, id, group, "create_location", nil); err != nil {
		t.Fatalf("Register: %v", err)
	}

	closeConn(t, conn)

	err = j.Fail(ctx, id, group, errors.New("tool failed"), nil)
	var internal journal.InternalErr
	if !errors.As(err, &internal) {
		t.Fatalf("want InternalErr after PG outage, got %v", err)
	}
	if !j.emergency.find(id, group) {
		t.Fatal("want emergency marker after Fail with dead PG")
	}

	_, err = j.Find(ctx, id, group)
	if !errors.Is(err, journal.ErrAlreadyFailed) {
		t.Fatalf("want ErrAlreadyFailed from emergency, got %v", err)
	}
}

func TestPostgresJournal_FailOnSaveError_UsesEmergency(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	j := newJournal(t)
	id, group := newID(), newGroup()

	if _, _, err := j.Register(ctx, id, group, "create_location", nil); err != nil {
		t.Fatalf("Register: %v", err)
	}

	db := testConn.DB()
	if err := db.Exec(`
CREATE OR REPLACE FUNCTION execution_journal_block_update() RETURNS trigger AS $$
BEGIN
  RAISE EXCEPTION 'simulated pg save failure';
END;
$$ LANGUAGE plpgsql;
`).Error; err != nil {
		t.Fatalf("create function: %v", err)
	}
	if err := db.Exec(`
CREATE TRIGGER execution_journal_block_update
BEFORE UPDATE ON execution_journal_requests
FOR EACH ROW EXECUTE FUNCTION execution_journal_block_update();
`).Error; err != nil {
		t.Fatalf("create trigger: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Exec(`DROP TRIGGER IF EXISTS execution_journal_block_update ON execution_journal_requests`).Error
		_ = db.Exec(`DROP FUNCTION IF EXISTS execution_journal_block_update()`).Error
	})

	err := j.Fail(ctx, id, group, errors.New("tool failed"), nil)
	var internal journal.InternalErr
	if !errors.As(err, &internal) {
		t.Fatalf("want InternalErr after save failure, got %v", err)
	}
	if !j.emergency.find(id, group) {
		t.Fatal("want emergency marker after save failure")
	}

	_, err = j.Find(ctx, id, group)
	if !errors.Is(err, journal.ErrAlreadyFailed) {
		t.Fatalf("want ErrAlreadyFailed, got %v", err)
	}

	// Postgres row stays Running — Fail never persisted.
	var status journal.Status
	if err := db.Model(&journal.Request{}).
		Select("status").
		Where("id = ? AND group_id = ?", id, group).
		Scan(&status).Error; err != nil {
		t.Fatalf("scan status: %v", err)
	}
	if status != journal.Status_Running {
		t.Fatalf("want Running in PG, got %v", status)
	}
}

func TestPostgresJournal_FindOnClosedConnection_InternalErr(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	conn := openIsolatedConn(t)
	j, err := NewPostgresJournal(conn)
	if err != nil {
		t.Fatalf("NewPostgresJournal: %v", err)
	}

	id, group := newID(), newGroup()
	if _, _, err := j.Register(ctx, id, group, "create_location", nil); err != nil {
		t.Fatalf("Register: %v", err)
	}

	closeConn(t, conn)

	_, err = j.Find(ctx, id, group)
	var internal journal.InternalErr
	if !errors.As(err, &internal) {
		t.Fatalf("want InternalErr when PG is down and no emergency, got %v", err)
	}
}

func TestPostgresJournal_DeleteAllExpired_ClearsEmergency(t *testing.T) {
	resetDB(t)
	ctx := context.Background()
	j := newJournal(t)
	id, group := newID(), newGroup()

	if _, _, err := j.Register(ctx, id, group, "create_location", nil); err != nil {
		t.Fatalf("Register: %v", err)
	}
	j.emergency.add(id, group)

	past := time.Now().UTC().Add(-time.Hour)
	if err := j.Close(ctx, group, past); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := j.DeleteAllExpired(ctx, 0); err != nil {
		t.Fatalf("DeleteAllExpired: %v", err)
	}

	if j.emergency.find(id, group) {
		t.Fatal("want emergency cleared after DeleteAllExpired")
	}
	_, err := j.Find(ctx, id, group)
	if _, ok := errors.AsType[journal.NotFound](err); !ok {
		t.Fatalf("want NotFound after group deleted, got %v", err)
	}
}
