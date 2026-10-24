package locations

import (
	"testing"

	filterslocations "github.com/dejitarudemon/mundarium/internal/libs/filters/locations"
	liblocations "github.com/dejitarudemon/mundarium/internal/libs/locations"
	"github.com/dejitarudemon/mundarium/internal/libs/types"
)

func TestFindLocationsMatch(t *testing.T) {
	sql, args := findLocationsMatch(filterslocations.FindLocationsFilter{})
	if sql != "TRUE" || len(args) != 0 {
		t.Fatalf("empty filter: sql=%q args=%v", sql, args)
	}

	locType := types.LocationType_REGION
	subtype := types.ID(3)
	query := "gate"
	sql, args = findLocationsMatch(filterslocations.FindLocationsFilter{
		Type:      &locType,
		SubtypeID: &subtype,
		Query:     &query,
	})
	if sql != "TRUE AND type = ? AND subtype_id = ? AND (name ILIKE '%' || ? || '%' OR description ILIKE '%' || ? || '%')" {
		t.Fatalf("unexpected sql: %s", sql)
	}
	if len(args) != 4 || args[0] != locType || args[1] != subtype || args[2] != query || args[3] != query {
		t.Fatalf("unexpected args: %#v", args)
	}
}

func TestGroupFindLocations(t *testing.T) {
	rows := []findLocationsRow{
		{Location: liblocations.Location{ID: 1, Name: "World"}, RelDepth: 0},
		{Location: liblocations.Location{ID: 2, Name: "North"}, RelDepth: 1},
		{Location: liblocations.Location{ID: 3, Name: "Gate"}, RelDepth: 2},
	}
	got := groupFindLocations(rows)
	if got.Scope.Name != "World" || len(got.Children[1]) != 1 || len(got.Children[2]) != 1 {
		t.Fatalf("unexpected grouping: %#v", got)
	}
	if got.Children[1][0].Name != "North" || got.Children[2][0].Name != "Gate" {
		t.Fatalf("unexpected children: %#v", got.Children)
	}
}
