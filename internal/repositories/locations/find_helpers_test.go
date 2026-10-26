package locations

import (
	"testing"

	"github.com/dejitarudemon/mundarium/internal/repositories"
)

func TestFindLocationsMatch(t *testing.T) {
	sql, args := findLocationsMatch(FindLocationsFilter{})
	if sql != "TRUE" || len(args) != 0 {
		t.Fatalf("empty filter: sql=%q args=%v", sql, args)
	}

	locType := LocationType_REGION
	subtype := repositories.ID(3)
	query := "gate"
	sql, args = findLocationsMatch(FindLocationsFilter{
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
		{Location: Location{ID: 1, Name: "root"}, RelDepth: 0},
		{Location: Location{ID: 2, Name: "child"}, RelDepth: 1},
		{Location: Location{ID: 3, Name: "grand"}, RelDepth: 2},
	}
	got := groupFindLocations(rows)
	if got.Scope.ID != 1 || got.Scope.Name != "root" {
		t.Fatalf("scope: %+v", got.Scope)
	}
	if len(got.Children[1]) != 1 || got.Children[1][0].ID != 2 {
		t.Fatalf("depth1: %+v", got.Children[1])
	}
	if len(got.Children[2]) != 1 || got.Children[2][0].ID != 3 {
		t.Fatalf("depth2: %+v", got.Children[2])
	}
}
