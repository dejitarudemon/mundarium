package locations_test

import (
	"context"
	"errors"
	"testing"

	"github.com/dejitarudemon/mundarium/internal/repositories/locations"
	"github.com/lib/pq"
	"gorm.io/gorm"
)

func TestCreateAndGetSubtype(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	created := createSubtype(t, "tavern", locations.LocationType_STRUCTURE, "inn")

	got, err := testRepo.GetSubtype(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetSubtype: %v", err)
	}
	if got.ID != created.ID || got.Key != "tavern" || got.Type != locations.LocationType_STRUCTURE {
		t.Fatalf("unexpected subtype: %+v", got)
	}
	if len(got.DefiningTags) != 1 || got.DefiningTags[0] != "tag-a" {
		t.Fatalf("unexpected DefiningTags: %#v", got.DefiningTags)
	}
	if got.Description != "inn" {
		t.Fatalf("want description %q, got %q", "inn", got.Description)
	}
}

func TestCreateSubtype_NormalizesKeyLowercase(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	created, err := testRepo.CreateSubtype(ctx, &locations.LocationSubtype{
		Key:          "TaVeRn",
		Type:         locations.LocationType_STRUCTURE,
		DefiningTags: pq.StringArray{},
		ExpectedTags: pq.StringArray{},
		Description:  "inn",
	})
	if err != nil {
		t.Fatalf("CreateSubtype: %v", err)
	}
	if created.Key != "tavern" {
		t.Fatalf("want key %q, got %q", "tavern", created.Key)
	}

	got, err := testRepo.GetSubtype(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetSubtype: %v", err)
	}
	if got.Key != "tavern" {
		t.Fatalf("want persisted key %q, got %q", "tavern", got.Key)
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

	tavern := createSubtype(t, "tavern", locations.LocationType_STRUCTURE, "inn")
	castle := createSubtype(t, "castle", locations.LocationType_STRUCTURE, "fort")
	cave := createSubtype(t, "cave", locations.LocationType_ROOM, "dark")
	_ = createSubtype(t, "dark_tavern", locations.LocationType_STRUCTURE, "shadow inn")

	all, err := testRepo.FindSubtypes(ctx, locations.FindSubtypesFilter{})
	if err != nil {
		t.Fatalf("FindSubtypes all: %v", err)
	}
	if len(all) != 4 {
		t.Fatalf("want 4 subtypes, got %#v", all)
	}

	byID, err := testRepo.FindSubtypes(ctx, locations.FindSubtypesFilter{ID: new(tavern.ID)})
	if err != nil {
		t.Fatalf("FindSubtypes by id: %v", err)
	}
	if len(byID) != 1 || byID[0].ID != tavern.ID {
		t.Fatalf("want only tavern by id, got %#v", byID)
	}

	byKeys, err := testRepo.FindSubtypes(ctx, locations.FindSubtypesFilter{
		Keys: []string{"TAV", "cas"},
	})
	if err != nil {
		t.Fatalf("FindSubtypes by keys: %v", err)
	}
	if len(byKeys) != 3 {
		t.Fatalf("want 3 by keys LIKE tav|cas, got %#v", byKeys)
	}

	structure := locations.LocationType_STRUCTURE
	byType, err := testRepo.FindSubtypes(ctx, locations.FindSubtypesFilter{Type: &structure})
	if err != nil {
		t.Fatalf("FindSubtypes by type: %v", err)
	}
	if len(byType) != 3 {
		t.Fatalf("want 3 structures, got %#v", byType)
	}

	byBoth, err := testRepo.FindSubtypes(ctx, locations.FindSubtypesFilter{
		Keys: []string{"CAS"},
		Type: &structure,
	})
	if err != nil {
		t.Fatalf("FindSubtypes by keys+type: %v", err)
	}
	if len(byBoth) != 1 || byBoth[0].ID != castle.ID {
		t.Fatalf("want only castle, got %#v", byBoth)
	}

	empty, err := testRepo.FindSubtypes(ctx, locations.FindSubtypesFilter{Keys: []string{"nope", "  "}})
	if err != nil {
		t.Fatalf("FindSubtypes miss: %v", err)
	}
	if len(empty) != 0 {
		t.Fatalf("want empty list, got %#v", empty)
	}

	_ = cave
}

func TestGetLocation_PreloadsSubtype(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	subtype := createSubtype(t, "tavern", locations.LocationType_STRUCTURE, "inn")
	root := createRoot(t, "town")
	id := subtype.ID
	loc, err := testRepo.CreateLocation(ctx, &locations.Location{
		ParentID:  new(root.ID),
		Type:      locations.LocationType_STRUCTURE,
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
