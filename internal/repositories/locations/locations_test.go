package locations_test

import (
	"context"
	"errors"
	"testing"

	"gorm.io/gorm"
)

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
