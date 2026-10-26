package implemented_test

import (
	"errors"
	"testing"
	"time"

	"github.com/dejitarudemon/mundarium/internal/journal"
	"github.com/dejitarudemon/mundarium/internal/journal/implemented"
	libjournal "github.com/dejitarudemon/mundarium/internal/libs/journal"
)

func TestMapGroup_AddSwapClose(t *testing.T) {
	groupID := libjournal.ID("grp-1")
	g := implemented.NewMapGroup(groupID)
	id := libjournal.ID("req-1")

	if g.Len() != 0 {
		t.Fatalf("empty Len: want 0, got %d", g.Len())
	}
	if g.IsExpired(time.Now(), time.Hour) {
		t.Fatal("empty group must not be expired")
	}

	req := libjournal.Request{
		ID:        id,
		GroupID:   groupID,
		CreatedAt: time.Now(),
		Tool:      "create_location",
		ArgsHash:  []byte{1, 2, 3},
		Status:    libjournal.Status_Running,
	}
	if err := g.Add(req); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if g.Len() != 1 {
		t.Fatalf("Len after Add: want 1, got %d", g.Len())
	}

	got, ok := g.Get(id)
	if !ok || got.Tool != "create_location" {
		t.Fatalf("Get: ok=%v got=%+v", ok, got)
	}
	got.ArgsHash[0] = 9
	again, _ := g.Get(id)
	if again.ArgsHash[0] != 1 {
		t.Fatal("Get must return a defensive copy")
	}

	var already journal.AlreadyRegistered
	if err := g.Add(req); !errors.As(err, &already) {
		t.Fatalf("duplicate Add: want AlreadyRegistered, got %v", err)
	}

	if _, ok := g.Get("missing"); ok {
		t.Fatal("Get missing: want false")
	}

	var notFound journal.NotFound
	if err := g.Swap(libjournal.Request{ID: "missing", GroupID: groupID}); !errors.As(err, &notFound) {
		t.Fatalf("Swap missing: want NotFound, got %v", err)
	}
	if notFound.GroupRequestID != groupID {
		t.Fatalf("NotFound group: want %s, got %s", groupID, notFound.GroupRequestID)
	}

	deadline := time.Now().UTC().Add(time.Hour).Truncate(time.Second)
	g.Close(deadline)
	stored, _ := g.Get(id)
	if stored.ExpiresAt == nil || !stored.ExpiresAt.Equal(deadline) {
		t.Fatalf("Close ExpiresAt: want %v, got %v", deadline, stored.ExpiresAt)
	}

	if err := g.Add(libjournal.Request{ID: "req-2", GroupID: groupID}); !errors.Is(err, journal.ErrGroupIsClosed) {
		t.Fatalf("Add after Close: want ErrGroupIsClosed, got %v", err)
	}

	updated := stored
	updated.Status = libjournal.Status_Completed
	updated.Result = libjournal.Result{Action: libjournal.Action_Create}
	updated.ExpiresAt = nil // Swap must preserve group ExpiresAt
	if err := g.Swap(updated); err != nil {
		t.Fatalf("Swap after Close: %v", err)
	}
	stored, _ = g.Get(id)
	if stored.Status != libjournal.Status_Completed || stored.Result.Action != libjournal.Action_Create {
		t.Fatalf("Swap update: %+v", stored)
	}
	if stored.ExpiresAt == nil || !stored.ExpiresAt.Equal(deadline) {
		t.Fatalf("Swap must keep ExpiresAt %v, got %v", deadline, stored.ExpiresAt)
	}

	later := deadline.Add(time.Hour)
	g.Close(later)
	stored, _ = g.Get(id)
	if stored.ExpiresAt == nil || !stored.ExpiresAt.Equal(deadline) {
		t.Fatalf("second Close must not rewrite ExpiresAt: want %v, got %v", deadline, stored.ExpiresAt)
	}
}

func TestMapGroup_IsExpired(t *testing.T) {
	g := implemented.NewMapGroup("grp")
	now := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)
	past := now.Add(-time.Minute)
	future := now.Add(time.Minute)

	a := libjournal.Request{ID: "a", CreatedAt: now, ExpiresAt: &past, Status: libjournal.Status_Running}
	b := libjournal.Request{ID: "b", CreatedAt: now, ExpiresAt: &future, Status: libjournal.Status_Running}
	if err := g.Add(a); err != nil {
		t.Fatalf("Add a: %v", err)
	}
	if err := g.Add(b); err != nil {
		t.Fatalf("Add b: %v", err)
	}
	if g.IsExpired(now, 0) {
		t.Fatal("mixed soft expiry: want false")
	}

	g.Close(past)
	if !g.IsExpired(now, 0) {
		t.Fatal("after Close with past deadline: want true")
	}
}
