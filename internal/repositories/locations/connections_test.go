package locations_test

import (
	"context"
	"errors"
	"testing"

	"github.com/dejitarudemon/mundarium/internal/libs/locations"
	"github.com/dejitarudemon/mundarium/internal/libs/types"
	"gorm.io/gorm"
)

func createConnection(t *testing.T, fromID, toID types.ID, description string) *locations.Connection {
	t.Helper()

	conn := &locations.Connection{
		FromID:      fromID,
		ToID:        toID,
		Description: description,
	}
	if err := testRepo.CreateConnection(context.Background(), conn); err != nil {
		t.Fatalf("CreateConnection: %v", err)
	}
	return conn
}

func TestCreateAndGetConnection(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	from := createRoot(t, "from")
	to := createRoot(t, "to")
	created := createConnection(t, from.ID, to.ID, "path")

	got, err := testRepo.GetConnection(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetConnection: %v", err)
	}
	if got.ID != created.ID || got.FromID != from.ID || got.ToID != to.ID || got.Description != "path" {
		t.Fatalf("unexpected connection: %+v", got)
	}
	if got.Bidirectional {
		t.Fatalf("want Bidirectional=false by default")
	}
}

func TestGetConnection_NotFound(t *testing.T) {
	resetDB(t)

	_, err := testRepo.GetConnection(context.Background(), 999)
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected ErrRecordNotFound, got %v", err)
	}
}

func TestCreateConnection_AllowsSameEndpoints(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	loc := createRoot(t, "loop")
	conn := &locations.Connection{FromID: loc.ID, ToID: loc.ID, Description: "self"}
	if err := testRepo.CreateConnection(ctx, conn); err != nil {
		t.Fatalf("CreateConnection same endpoints: %v", err)
	}
}

func TestFindInderectConnections(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	a := createRoot(t, "A")
	b := createRoot(t, "B")
	c := createRoot(t, "C")

	ab1 := createConnection(t, a.ID, b.ID, "ab-1")
	ab2 := createConnection(t, a.ID, b.ID, "ab-2")
	_ = createConnection(t, a.ID, c.ID, "ac")
	_ = createConnection(t, b.ID, a.ID, "ba")

	got, err := testRepo.FindInderectConnections(ctx, a.ID, b.ID)
	if err != nil {
		t.Fatalf("FindInderectConnections: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 A->B connections, got %#v", got)
	}

	ids := map[types.ID]struct{}{got[0].ID: {}, got[1].ID: {}}
	if _, ok := ids[ab1.ID]; !ok {
		t.Fatalf("missing ab-1 in %#v", got)
	}
	if _, ok := ids[ab2.ID]; !ok {
		t.Fatalf("missing ab-2 in %#v", got)
	}

	empty, err := testRepo.FindInderectConnections(ctx, c.ID, a.ID)
	if err != nil {
		t.Fatalf("FindInderectConnections empty: %v", err)
	}
	if len(empty) != 0 {
		t.Fatalf("want empty, got %#v", empty)
	}
}

func TestPutConnection(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	a := createRoot(t, "A")
	b := createRoot(t, "B")
	c := createRoot(t, "C")
	conn := createConnection(t, a.ID, b.ID, "old")

	expires := int64(1_700_000_000)
	conn.FromID = a.ID
	conn.ToID = c.ID
	conn.Bidirectional = true
	conn.Description = "new"
	conn.ExpiresAt = &expires
	conn.Details = types.JSON(`{"k":"v"}`)
	conn.State = types.JSON(`{"open":true}`)

	if err := testRepo.PutConnection(ctx, conn); err != nil {
		t.Fatalf("PutConnection: %v", err)
	}

	got, err := testRepo.GetConnection(ctx, conn.ID)
	if err != nil {
		t.Fatalf("GetConnection: %v", err)
	}
	if got.ToID != c.ID || !got.Bidirectional || got.Description != "new" {
		t.Fatalf("unexpected updated connection: %+v", got)
	}
	if got.ExpiresAt == nil || *got.ExpiresAt != expires {
		t.Fatalf("unexpected ExpiresAt: %#v", got.ExpiresAt)
	}
}

func TestPutConnection_RejectsSameEndpoints(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	a := createRoot(t, "A")
	b := createRoot(t, "B")
	conn := createConnection(t, a.ID, b.ID, "link")
	conn.ToID = a.ID

	err := testRepo.PutConnection(ctx, conn)
	if err == nil || err.Error() != "connection between the same locations" {
		t.Fatalf("want same-location error, got %v", err)
	}
}

func TestPutConnection_NotFound(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	a := createRoot(t, "A")
	b := createRoot(t, "B")

	err := testRepo.PutConnection(ctx, &locations.Connection{
		ID:     999,
		FromID: a.ID,
		ToID:   b.ID,
	})
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected ErrRecordNotFound, got %v", err)
	}
}

func TestDeleteConnection(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	a := createRoot(t, "A")
	b := createRoot(t, "B")
	conn := createConnection(t, a.ID, b.ID, "temp")

	if err := testRepo.DeleteConnection(ctx, conn.ID); err != nil {
		t.Fatalf("DeleteConnection: %v", err)
	}

	_, err := testRepo.GetConnection(ctx, conn.ID)
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected ErrRecordNotFound after delete, got %v", err)
	}
}

func TestCreateConnection_RequiresExistingLocations(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	a := createRoot(t, "A")
	err := testRepo.CreateConnection(ctx, &locations.Connection{
		FromID: a.ID,
		ToID:   999,
	})
	if err == nil {
		t.Fatal("expected FK error for missing ToID")
	}
}
