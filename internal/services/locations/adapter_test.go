package locations

import (
	"github.com/dejitarudemon/mundarium/internal/repositories"
	"testing"

	input "github.com/dejitarudemon/mundarium/internal/libs/inputs/locations"
	repolocations "github.com/dejitarudemon/mundarium/internal/repositories/locations"
)

func idPtr(id repositories.ID) *repositories.ID {
	return &id
}

func TestToNewLocation(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		subtypeID := repositories.ID(7)
		loc, err := toNewLocation(input.CreateLocationInput{
			ParentID:    idPtr(3),
			Type:        "structure",
			SubtypeID:   &subtypeID,
			Name:        "Tavern",
			Description: "warm",
			Details:     repositories.JSON(`{"x":1}`),
		})
		if err != nil {
			t.Fatalf("toNewLocation: %v", err)
		}
		if loc.ParentID == nil || *loc.ParentID != 3 || loc.Type != repolocations.LocationType_STRUCTURE || loc.Name != "Tavern" {
			t.Fatalf("unexpected location: %+v", loc)
		}
		if loc.SubtypeID == nil || *loc.SubtypeID != 7 {
			t.Fatalf("unexpected subtype: %#v", loc.SubtypeID)
		}
	})

	t.Run("root parent", func(t *testing.T) {
		loc, err := toNewLocation(input.CreateLocationInput{Type: "world", Name: "x"})
		if err != nil {
			t.Fatalf("toNewLocation: %v", err)
		}
		if loc.ParentID != nil {
			t.Fatalf("want nil parent for root, got %v", loc.ParentID)
		}
	})

	t.Run("zero parent", func(t *testing.T) {
		_, err := toNewLocation(input.CreateLocationInput{ParentID: idPtr(0), Type: "region", Name: "x"})
		if err == nil || err.Error() != "parent id is zero" {
			t.Fatalf("want parent id is zero, got %v", err)
		}
	})

	t.Run("bad type", func(t *testing.T) {
		_, err := toNewLocation(input.CreateLocationInput{ParentID: idPtr(1), Type: "nope", Name: "x"})
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("zero subtype", func(t *testing.T) {
		zero := repositories.ID(0)
		_, err := toNewLocation(input.CreateLocationInput{
			ParentID:  idPtr(1),
			Type:      "region",
			SubtypeID: &zero,
			Name:      "x",
		})
		if err == nil || err.Error() != "subtype id is zero" {
			t.Fatalf("want subtype id is zero, got %v", err)
		}
	})
}

func TestToCreatedAndGotLocation(t *testing.T) {
	loc := repolocations.Location{
		ID:          10,
		ParentID:    nil,
		Type:        repolocations.LocationType_WORLD,
		Name:        "World",
		Description: "big",
		Details:     repositories.JSON(`{}`),
	}

	created, err := toCreatedLocation(loc)
	if err != nil {
		t.Fatalf("toCreatedLocation: %v", err)
	}
	if created.ID != 10 || created.ParentID != nil || created.Name != "World" {
		t.Fatalf("unexpected created: %+v", created)
	}

	got, err := toGotLocation(loc)
	if err != nil {
		t.Fatalf("toGotLocation: %v", err)
	}
	if got.Type != "world" || got.Description != "big" || got.ParentID != nil {
		t.Fatalf("unexpected got: %+v", got)
	}
}

func TestCheckLocation(t *testing.T) {
	if err := checkLocation(repolocations.Location{ID: 0, ParentID: idPtr(1)}); err == nil || err.Error() != "location id is zero" {
		t.Fatalf("want location id is zero, got %v", err)
	}
	if err := checkLocation(repolocations.Location{ID: 1, ParentID: idPtr(0)}); err == nil || err.Error() != "parent id is zero" {
		t.Fatalf("want parent id is zero, got %v", err)
	}
	if err := checkLocation(repolocations.Location{ID: 1, ParentID: nil}); err != nil {
		t.Fatalf("root location should be valid: %v", err)
	}
}

func TestToGotChildLocations(t *testing.T) {
	children := map[int][]repolocations.Location{
		1: {{ID: 2, ParentID: idPtr(1), Type: repolocations.LocationType_REGION, Name: "A"}},
		2: {{ID: 3, ParentID: idPtr(2), Type: repolocations.LocationType_SITE, Name: "A1"}},
	}

	out, err := toGotChildLocations(children)
	if err != nil {
		t.Fatalf("toGotChildLocations: %v", err)
	}
	if len(out[1]) != 1 || out[1][0].Name != "A" || len(out[2]) != 1 || out[2][0].Name != "A1" {
		t.Fatalf("unexpected output: %#v", out)
	}

	_, err = toGotChildLocations(map[int][]repolocations.Location{
		1: {{ID: 0, ParentID: idPtr(1), Name: "bad"}},
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
}

func TestToFindLocationsFilter(t *testing.T) {
	depth := 2
	locType := "region"
	query := "north"

	const maxDepth = 5

	got, limit, err := toFindLocationsFilter(input.FindLocationsInput{
		ScopeID: 7,
		Query:   &query,
		Type:    &locType,
		Depth:   &depth,
	}, maxDepth)
	if err != nil {
		t.Fatalf("toFindLocationsFilter: %v", err)
	}
	if got.ScopeID != 7 || got.Depth != 2 || limit != defaultFindLocationsLimit {
		t.Fatalf("unexpected filter/limit: %+v %d", got, limit)
	}
	if got.Query == nil || *got.Query != "north" {
		t.Fatalf("unexpected query: %#v", got.Query)
	}
	if got.Type == nil || *got.Type != repolocations.LocationType_REGION {
		t.Fatalf("unexpected type: %#v", got.Type)
	}

	_, _, err = toFindLocationsFilter(input.FindLocationsInput{ScopeID: 0}, maxDepth)
	if err == nil || err.Error() != "scope id is zero" {
		t.Fatalf("want scope id is zero, got %v", err)
	}

	defaults, limit, err := toFindLocationsFilter(input.FindLocationsInput{ScopeID: 1}, maxDepth)
	if err != nil {
		t.Fatalf("defaults: %v", err)
	}
	if defaults.Depth != 1 || limit != 20 {
		t.Fatalf("want depth=1 limit=20, got %+v %d", defaults, limit)
	}

	badDepth := 6
	_, _, err = toFindLocationsFilter(input.FindLocationsInput{ScopeID: 1, Depth: &badDepth}, maxDepth)
	if err == nil || err.Error() != "depth out of range" {
		t.Fatalf("want depth out of range, got %v", err)
	}

	badLimit := 0
	_, _, err = toFindLocationsFilter(input.FindLocationsInput{ScopeID: 1, Limit: &badLimit}, maxDepth)
	if err == nil || err.Error() != "limit out of range" {
		t.Fatalf("want limit out of range, got %v", err)
	}
}
