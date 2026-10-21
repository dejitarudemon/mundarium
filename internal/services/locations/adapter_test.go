package locations

import (
	"testing"

	input "github.com/dejitarudemon/mundarium/internal/libs/inputs/locations"
	"github.com/dejitarudemon/mundarium/internal/libs/locations"
	"github.com/dejitarudemon/mundarium/internal/libs/types"
)

func TestToNewLocation(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		subtypeID := types.ID(7)
		loc, err := toNewLocation(input.CreateLocationInput{
			ParentID:    3,
			Type:        "structure",
			SubtypeID:   &subtypeID,
			Name:        "Tavern",
			Description: "warm",
			Details:     types.JSON(`{"x":1}`),
		})
		if err != nil {
			t.Fatalf("toNewLocation: %v", err)
		}
		if loc.ParentID != 3 || loc.Type != types.LocationType_STRUCTURE || loc.Name != "Tavern" {
			t.Fatalf("unexpected location: %+v", loc)
		}
		if loc.SubtypeID == nil || *loc.SubtypeID != 7 {
			t.Fatalf("unexpected subtype: %#v", loc.SubtypeID)
		}
	})

	t.Run("zero parent", func(t *testing.T) {
		_, err := toNewLocation(input.CreateLocationInput{Type: "region", Name: "x"})
		if err == nil || err.Error() != "parent id is zero" {
			t.Fatalf("want parent id is zero, got %v", err)
		}
	})

	t.Run("bad type", func(t *testing.T) {
		_, err := toNewLocation(input.CreateLocationInput{ParentID: 1, Type: "nope", Name: "x"})
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("zero subtype", func(t *testing.T) {
		zero := types.ID(0)
		_, err := toNewLocation(input.CreateLocationInput{
			ParentID:  1,
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
	loc := locations.Location{
		ID:          10,
		ParentID:    10,
		Type:        types.LocationType_WORLD,
		Name:        "World",
		Description: "big",
		Details:     types.JSON(`{}`),
	}

	created, err := toCreatedLocation(loc)
	if err != nil {
		t.Fatalf("toCreatedLocation: %v", err)
	}
	if created.ID != 10 || created.ParentID != 10 || created.Name != "World" {
		t.Fatalf("unexpected created: %+v", created)
	}

	got, err := toGotLocation(loc)
	if err != nil {
		t.Fatalf("toGotLocation: %v", err)
	}
	if got.Type != "world" || got.Description != "big" {
		t.Fatalf("unexpected got: %+v", got)
	}
}

func TestCheckLocation(t *testing.T) {
	if err := checkLocation(locations.Location{ID: 0, ParentID: 1}); err == nil || err.Error() != "location id is zero" {
		t.Fatalf("want location id is zero, got %v", err)
	}
	if err := checkLocation(locations.Location{ID: 1, ParentID: 0}); err == nil || err.Error() != "parent id is zero" {
		t.Fatalf("want parent id is zero, got %v", err)
	}
}

func TestToGotChildLocations(t *testing.T) {
	children := map[int][]locations.Location{
		1: {{ID: 2, ParentID: 1, Type: types.LocationType_REGION, Name: "A"}},
		2: {{ID: 3, ParentID: 2, Type: types.LocationType_SITE, Name: "A1"}},
	}

	out, err := toGotChildLocations(children)
	if err != nil {
		t.Fatalf("toGotChildLocations: %v", err)
	}
	if len(out[1]) != 1 || out[1][0].Name != "A" || len(out[2]) != 1 || out[2][0].Name != "A1" {
		t.Fatalf("unexpected output: %#v", out)
	}

	_, err = toGotChildLocations(map[int][]locations.Location{
		1: {{ID: 0, ParentID: 1, Name: "bad"}},
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
}
