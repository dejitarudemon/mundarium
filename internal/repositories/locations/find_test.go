package locations_test

import (
	"context"
	"errors"
	"github.com/dejitarudemon/mundarium/internal/repositories/locations"
	"testing"

	"gorm.io/gorm"
)

func typePtr(t locations.LocationType) *locations.LocationType { return &t }

func childCount(children map[int][]locations.Location) int {
	n := 0
	for _, locs := range children {
		n += len(locs)
	}
	return n
}

func TestFindLocations_DepthAndQuery(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	root := createRoot(t, "World")
	north := createLocation(t, idPtr(root.ID), "North")
	gate, err := testRepo.CreateLocation(ctx, &locations.Location{
		ParentID:    idPtr(north.ID),
		Type:        locations.LocationType_SITE,
		Name:        "North Gate",
		Description: "stone arch",
	})
	if err != nil {
		t.Fatalf("CreateLocation gate: %v", err)
	}
	_ = createLocation(t, idPtr(root.ID), "South")

	depth0, err := testRepo.FindLocations(ctx, locations.FindLocationsFilter{
		ScopeID: root.ID,
		Depth:   0,
	})
	if err != nil {
		t.Fatalf("depth 0: %v", err)
	}
	if depth0.Scope.ID != root.ID || childCount(depth0.Children) != 0 {
		t.Fatalf("depth 0: want only scope, got %#v", depth0)
	}

	depth1, err := testRepo.FindLocations(ctx, locations.FindLocationsFilter{
		ScopeID: root.ID,
		Depth:   1,
	})
	if err != nil {
		t.Fatalf("depth 1: %v", err)
	}
	if depth1.Scope.ID != root.ID || len(depth1.Children[1]) != 2 {
		t.Fatalf("depth 1: want scope+2 children, got %#v", depth1)
	}

	query := "Gate"
	found, err := testRepo.FindLocations(ctx, locations.FindLocationsFilter{
		ScopeID: root.ID,
		Depth:   2,
		Query:   &query,
	})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if found.Scope.ID != root.ID || len(found.Children[2]) != 1 || found.Children[2][0].ID != gate.ID {
		t.Fatalf("query: want scope+North Gate at depth 2, got %#v", found)
	}
}

func TestFindLocations_TypeAndSubtype(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	root := createRoot(t, "town")
	subtype := createSubtype(t, "tavern", locations.LocationType_STRUCTURE, "inn")
	subtypeID := subtype.ID

	tavern, err := testRepo.CreateLocation(ctx, &locations.Location{
		ParentID:  idPtr(root.ID),
		Type:      locations.LocationType_STRUCTURE,
		SubtypeID: &subtypeID,
		Name:      "The Boar",
	})
	if err != nil {
		t.Fatalf("CreateLocation tavern: %v", err)
	}
	_ = createLocation(t, idPtr(root.ID), "Square")

	byType, err := testRepo.FindLocations(ctx, locations.FindLocationsFilter{
		ScopeID: root.ID,
		Depth:   1,
		Type:    typePtr(locations.LocationType_STRUCTURE),
	})
	if err != nil {
		t.Fatalf("by type: %v", err)
	}
	if byType.Scope.ID != root.ID || len(byType.Children[1]) != 1 || byType.Children[1][0].ID != tavern.ID {
		t.Fatalf("by type: want scope+tavern, got %#v", byType)
	}

	bySubtype, err := testRepo.FindLocations(ctx, locations.FindLocationsFilter{
		ScopeID:   root.ID,
		Depth:     1,
		SubtypeID: &subtypeID,
	})
	if err != nil {
		t.Fatalf("by subtype: %v", err)
	}
	if bySubtype.Scope.ID != root.ID || len(bySubtype.Children[1]) != 1 || bySubtype.Children[1][0].ID != tavern.ID {
		t.Fatalf("by subtype: want scope+tavern, got %#v", bySubtype)
	}
}

func TestFindLocations_NotFoundAndDepthRange(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	_, err := testRepo.FindLocations(ctx, locations.FindLocationsFilter{ScopeID: 999, Depth: 1})
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("want ErrRecordNotFound, got %v", err)
	}

	root := createRoot(t, "world")
	_, err = testRepo.FindLocations(ctx, locations.FindLocationsFilter{ScopeID: root.ID, Depth: 6})
	if err == nil || err.Error() != "depth out of range" {
		t.Fatalf("want depth out of range, got %v", err)
	}
	_, err = testRepo.FindLocations(ctx, locations.FindLocationsFilter{ScopeID: root.ID, Depth: -1})
	if err == nil || err.Error() != "depth out of range" {
		t.Fatalf("want depth out of range, got %v", err)
	}
}
