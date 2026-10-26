package locations_test

import (
	"context"
	"errors"
	"github.com/dejitarudemon/mundarium/internal/repositories"
	"testing"

	"github.com/dejitarudemon/mundarium/internal/libs/locations"
	"gorm.io/gorm"
)

func idPtr(id repositories.ID) *repositories.ID {
	return &id
}

func createLocation(t *testing.T, parentID *repositories.ID, name string) *locations.Location {
	t.Helper()

	loc, err := testRepo.CreateLocation(context.Background(), &locations.Location{
		ParentID: parentID,
		Type:     repositories.LocationType_REGION,
		Name:     name,
	})
	if err != nil {
		t.Fatalf("CreateLocation(%q): %v", name, err)
	}

	return loc
}

func createRoot(t *testing.T, name string) *locations.Location {
	t.Helper()
	return createLocation(t, nil, name)
}

func TestCreateAndGetLocation(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	root := createRoot(t, "world")
	got, err := testRepo.GetLocation(ctx, root.ID)
	if err != nil {
		t.Fatalf("GetLocation: %v", err)
	}
	if got.ID != root.ID || got.ParentID != nil || got.Name != "world" {
		t.Fatalf("unexpected location: %+v", got)
	}
}

func TestGetLocation_NotFound(t *testing.T) {
	resetDB(t)

	_, err := testRepo.GetLocation(context.Background(), 999)
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected ErrRecordNotFound, got %v", err)
	}
}

func TestFindChildLocations_ByDepth(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	root := createRoot(t, "root")
	childA := createLocation(t, idPtr(root.ID), "A")
	childB := createLocation(t, idPtr(root.ID), "B")
	grand := createLocation(t, idPtr(childA.ID), "A1")

	depth0, err := testRepo.FindChildLocations(ctx, root.ID, 0, false)
	if err != nil {
		t.Fatalf("depth 0: %v", err)
	}
	if len(depth0) != 0 {
		t.Fatalf("depth 0: want empty map, got %#v", depth0)
	}

	depth1, err := testRepo.FindChildLocations(ctx, root.ID, 1, false)
	if err != nil {
		t.Fatalf("depth 1: %v", err)
	}
	if len(depth1[1]) != 2 {
		t.Fatalf("depth 1: want 2 children, got %#v", depth1)
	}
	if _, ok := depth1[2]; ok {
		t.Fatalf("depth 1: unexpected level 2: %#v", depth1)
	}

	depth2, err := testRepo.FindChildLocations(ctx, root.ID, 2, false)
	if err != nil {
		t.Fatalf("depth 2: %v", err)
	}
	if len(depth2[1]) != 2 || len(depth2[2]) != 1 {
		t.Fatalf("depth 2: want levels 1=2 and 2=1, got %#v", depth2)
	}
	if depth2[2][0].ID != grand.ID {
		t.Fatalf("depth 2 grandchild: want %d, got %d", grand.ID, depth2[2][0].ID)
	}

	_ = childB
}

func TestFindChildLocations_SkipsCycles(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	// Force B <-> C via parent_id (child lookup uses parent_id, so each is a child of the other).
	b := createRoot(t, "B")
	c := createLocation(t, idPtr(b.ID), "C")
	if err := testDB.Model(&locations.Location{}).Where("id = ?", b.ID).Update("parent_id", c.ID).Error; err != nil {
		t.Fatalf("force cycle B.parent=C: %v", err)
	}

	got, err := testRepo.FindChildLocations(ctx, b.ID, 5, false)
	if err != nil {
		t.Fatalf("FindChildLocations: %v", err)
	}
	if len(got[1]) != 1 || got[1][0].ID != c.ID {
		t.Fatalf("level 1: want C, got %#v", got)
	}
	if _, ok := got[2]; ok {
		t.Fatalf("level 2 should be empty because B is already seen, got %#v", got)
	}
}

func TestPatchLocationParent_AllowsRoot(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	parent := createRoot(t, "parent")
	loc := createLocation(t, idPtr(parent.ID), "alone")
	if err := testRepo.PatchLocationParent(ctx, loc.ID, nil); err != nil {
		t.Fatalf("null parent root: %v", err)
	}

	got, err := testRepo.GetLocation(ctx, loc.ID)
	if err != nil {
		t.Fatalf("GetLocation: %v", err)
	}
	if got.ParentID != nil {
		t.Fatalf("want root ParentID=nil, got parent=%v id=%d", got.ParentID, got.ID)
	}
}

func TestPatchLocationParent_RejectsSelfParent(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	loc := createRoot(t, "alone")
	err := testRepo.PatchLocationParent(ctx, loc.ID, idPtr(loc.ID))
	if err == nil || err.Error() != "location would create a cycle" {
		t.Fatalf("want cycle error for self-parent, got %v", err)
	}
}

func TestPatchLocationParent_AllowsChain(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	// A > B > C  (A child of B child of C/root)
	c := createRoot(t, "C")
	b := createLocation(t, idPtr(c.ID), "B")
	a := createLocation(t, idPtr(b.ID), "A")

	if err := testRepo.PatchLocationParent(ctx, a.ID, idPtr(c.ID)); err != nil {
		t.Fatalf("reparent A under C: %v", err)
	}

	got, err := testRepo.GetLocation(ctx, a.ID)
	if err != nil {
		t.Fatalf("GetLocation: %v", err)
	}
	if got.ParentID == nil || *got.ParentID != c.ID {
		t.Fatalf("want parent %d, got %v", c.ID, got.ParentID)
	}
}

func TestPatchLocationParent_RejectsTwoNodeCycle(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	// A > B > A attempt: A root, B under A, then A.parent = B
	a := createRoot(t, "A")
	b := createLocation(t, idPtr(a.ID), "B")

	err := testRepo.PatchLocationParent(ctx, a.ID, idPtr(b.ID))
	if err == nil || err.Error() != "location would create a cycle" {
		t.Fatalf("want cycle error, got %v", err)
	}
}

func TestPatchLocationParent_RejectsCycleIntoDescendantBranch(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	// Setup: C root, B under C, A under B. Then PatchLocationParent(B, A) should fail
	// because A is descendant of B.
	c := createRoot(t, "C")
	b := createLocation(t, idPtr(c.ID), "B")
	a := createLocation(t, idPtr(b.ID), "A")

	err := testRepo.PatchLocationParent(ctx, b.ID, idPtr(a.ID))
	if err == nil || err.Error() != "location would create a cycle" {
		t.Fatalf("want cycle error when B.parent=A, got %v", err)
	}

	// Also: after making C.parent = B is invalid while B still under C.
	err = testRepo.PatchLocationParent(ctx, c.ID, idPtr(b.ID))
	if err == nil || err.Error() != "location would create a cycle" {
		t.Fatalf("want cycle error when C.parent=B, got %v", err)
	}
}

func TestPutLocation_RejectsCycle(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	a := createRoot(t, "A")
	b := createLocation(t, idPtr(a.ID), "B")

	b.ParentID = idPtr(a.ID)
	a.ParentID = idPtr(b.ID)
	err := testRepo.PutLocation(ctx, a)
	if err == nil || err.Error() != "location would create a cycle" {
		t.Fatalf("want cycle error, got %v", err)
	}
}

func TestPatchLocationName(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	loc := createRoot(t, "old")
	if err := testRepo.PatchLocationName(ctx, loc.ID, "new"); err != nil {
		t.Fatalf("PatchLocationName: %v", err)
	}

	got, err := testRepo.GetLocation(ctx, loc.ID)
	if err != nil {
		t.Fatalf("GetLocation: %v", err)
	}
	if got.Name != "new" {
		t.Fatalf("want name %q, got %q", "new", got.Name)
	}
}
