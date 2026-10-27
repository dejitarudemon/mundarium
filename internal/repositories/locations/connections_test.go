package locations_test

import (
	"context"
	"errors"
	"testing"

	"github.com/dejitarudemon/mundarium/internal/repositories"
	"github.com/dejitarudemon/mundarium/internal/repositories/locations"
	"gorm.io/gorm"
)

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

	ids := map[repositories.ID]struct{}{got[0].ID: {}, got[1].ID: {}}
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
	conn.Details = repositories.JSON(`{"k":"v"}`)
	conn.State = repositories.JSON(`{"open":true}`)

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
	if !errors.Is(err, locations.ErrConnectionSameEndpoints) {
		t.Fatalf("want ErrConnectionSameEndpoints, got %v", err)
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

func TestDeleteConnection_NotFound(t *testing.T) {
	resetDB(t)

	err := testRepo.DeleteConnection(context.Background(), 999)
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected ErrRecordNotFound, got %v", err)
	}
}

func TestFindConnections(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	a := createRoot(t, "A")
	b := createRoot(t, "B")
	c := createRoot(t, "C")

	ab := createConnection(t, a.ID, b.ID, "ab")
	ba := createConnection(t, b.ID, a.ID, "ba")
	ba.Bidirectional = true
	ba.FromID = b.ID
	ba.ToID = a.ID
	ba.Description = "ba"
	if err := testRepo.PutConnection(ctx, ba); err != nil {
		t.Fatalf("PutConnection ba: %v", err)
	}
	ac := createConnection(t, a.ID, c.ID, "ac")

	t.Run("by from", func(t *testing.T) {
		got, err := testRepo.FindConnections(ctx, locations.FindConnectionsFilter{FromID: &a.ID})
		if err != nil {
			t.Fatalf("FindConnections: %v", err)
		}
		if len(got) != 2 {
			t.Fatalf("want 2 from A, got %#v", got)
		}
		ids := map[repositories.ID]struct{}{got[0].ID: {}, got[1].ID: {}}
		if _, ok := ids[ab.ID]; !ok {
			t.Fatalf("missing ab in %#v", got)
		}
		if _, ok := ids[ac.ID]; !ok {
			t.Fatalf("missing ac in %#v", got)
		}
	})

	t.Run("directed pair", func(t *testing.T) {
		got, err := testRepo.FindConnections(ctx, locations.FindConnectionsFilter{FromID: &a.ID, ToID: &b.ID})
		if err != nil {
			t.Fatalf("FindConnections: %v", err)
		}
		if len(got) != 1 || got[0].ID != ab.ID {
			t.Fatalf("want only ab, got %#v", got)
		}
	})

	t.Run("either direction", func(t *testing.T) {
		got, err := testRepo.FindConnections(ctx, locations.FindConnectionsFilter{
			FromID:          &a.ID,
			ToID:            &b.ID,
			EitherDirection: true,
		})
		if err != nil {
			t.Fatalf("FindConnections: %v", err)
		}
		if len(got) != 2 {
			t.Fatalf("want ab+ba, got %#v", got)
		}
	})

	t.Run("bidirectional reverse", func(t *testing.T) {
		got, err := testRepo.FindConnections(ctx, locations.FindConnectionsFilter{
			FromID:                      &a.ID,
			ToID:                        &b.ID,
			IncludeBidirectionalReverse: true,
		})
		if err != nil {
			t.Fatalf("FindConnections: %v", err)
		}
		if len(got) != 2 {
			t.Fatalf("want ab + bidirectional ba, got %#v", got)
		}
	})

	t.Run("involving location", func(t *testing.T) {
		got, err := testRepo.FindConnections(ctx, locations.FindConnectionsFilter{LocationID: &b.ID})
		if err != nil {
			t.Fatalf("FindConnections: %v", err)
		}
		if len(got) != 2 {
			t.Fatalf("want connections involving B, got %#v", got)
		}
		ids := map[repositories.ID]struct{}{got[0].ID: {}, got[1].ID: {}}
		if _, ok := ids[ab.ID]; !ok {
			t.Fatalf("missing ab in %#v", got)
		}
		if _, ok := ids[ba.ID]; !ok {
			t.Fatalf("missing ba in %#v", got)
		}
	})
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
