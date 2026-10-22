package locations_test

import (
	"context"
	"errors"
	"testing"

	"github.com/dejitarudemon/mundarium/internal/libs/locations"
	"github.com/dejitarudemon/mundarium/internal/libs/types"
	"github.com/lib/pq"
	"gorm.io/gorm"
)

func createSubtype(t *testing.T, key string, locType types.LocationType, description string) *locations.LocationSubtype {
	t.Helper()

	subtype := &locations.LocationSubtype{
		Key:          key,
		Type:         locType,
		DefiningTags: pq.StringArray{"tag-a"},
		ExpectedTags: pq.StringArray{"tag-b"},
		Description:  description,
	}
	if err := testRepo.CreateSubtype(context.Background(), subtype); err != nil {
		t.Fatalf("CreateSubtype(%q): %v", key, err)
	}
	return subtype
}

func TestCreateAndGetSubtype(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	created := createSubtype(t, "tavern", types.LocationType_STRUCTURE, "inn")

	got, err := testRepo.GetSubtype(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetSubtype: %v", err)
	}
	if got.ID != created.ID || got.Key != "tavern" || got.Type != types.LocationType_STRUCTURE {
		t.Fatalf("unexpected subtype: %+v", got)
	}
	if len(got.DefiningTags) != 1 || got.DefiningTags[0] != "tag-a" {
		t.Fatalf("unexpected DefiningTags: %#v", got.DefiningTags)
	}
	if got.Description != "inn" {
		t.Fatalf("want description %q, got %q", "inn", got.Description)
	}
}

func TestGetSubtype_NotFound(t *testing.T) {
	resetDB(t)

	_, err := testRepo.GetSubtype(context.Background(), 999)
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected ErrRecordNotFound, got %v", err)
	}
}

func TestFindSubtypes_Filters(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	tavern := createSubtype(t, "tavern", types.LocationType_STRUCTURE, "inn")
	castle := createSubtype(t, "castle", types.LocationType_STRUCTURE, "fort")
	cave := createSubtype(t, "cave", types.LocationType_ROOM, "dark")

	all, err := testRepo.FindSubtypes(ctx, nil, nil)
	if err != nil {
		t.Fatalf("FindSubtypes all: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("want 3 subtypes, got %#v", all)
	}

	byKey, err := testRepo.FindSubtypes(ctx, []string{"tavern", "cave"}, nil)
	if err != nil {
		t.Fatalf("FindSubtypes by key: %v", err)
	}
	if len(byKey) != 2 {
		t.Fatalf("want 2 by key, got %#v", byKey)
	}

	byType, err := testRepo.FindSubtypes(ctx, nil, []types.LocationType{types.LocationType_STRUCTURE})
	if err != nil {
		t.Fatalf("FindSubtypes by type: %v", err)
	}
	if len(byType) != 2 {
		t.Fatalf("want 2 structures, got %#v", byType)
	}

	byBoth, err := testRepo.FindSubtypes(ctx, []string{"castle", "cave"}, []types.LocationType{types.LocationType_STRUCTURE})
	if err != nil {
		t.Fatalf("FindSubtypes by both: %v", err)
	}
	if len(byBoth) != 1 || byBoth[0].ID != castle.ID {
		t.Fatalf("want only castle, got %#v", byBoth)
	}

	_ = tavern
	_ = cave
}

func TestGetLocation_PreloadsSubtype(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	subtype := createSubtype(t, "tavern", types.LocationType_STRUCTURE, "inn")
	root := createRoot(t, "town")
	id := subtype.ID
	loc, err := testRepo.CreateLocation(ctx, &locations.Location{
		ParentID:  idPtr(root.ID),
		Type:      types.LocationType_STRUCTURE,
		SubtypeID: &id,
		Name:      "The Boar",
	})
	if err != nil {
		t.Fatalf("CreateLocation: %v", err)
	}

	got, err := testRepo.GetLocation(ctx, loc.ID)
	if err != nil {
		t.Fatalf("GetLocation: %v", err)
	}
	if got.Subtype == nil || got.Subtype.Key != "tavern" {
		t.Fatalf("want preloaded subtype tavern, got %+v", got.Subtype)
	}
}
