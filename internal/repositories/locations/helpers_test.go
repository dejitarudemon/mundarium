package locations_test

import (
	"context"
	"testing"

	"github.com/dejitarudemon/mundarium/internal/repositories"
	"github.com/dejitarudemon/mundarium/internal/repositories/locations"
	"github.com/lib/pq"
)

func createLocation(t *testing.T, parentID *repositories.ID, name string) *locations.Location {
	t.Helper()

	loc, err := testRepo.CreateLocation(context.Background(), &locations.Location{
		ParentID: parentID,
		Type:     locations.LocationType_REGION,
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

func createSubtype(t *testing.T, key string, locType locations.LocationType, description string) *locations.LocationSubtype {
	t.Helper()

	subtype := &locations.LocationSubtype{
		Key:          key,
		Type:         locType,
		DefiningTags: pq.StringArray{"tag-a"},
		ExpectedTags: pq.StringArray{"tag-b"},
		Description:  description,
	}
	created, err := testRepo.CreateSubtype(context.Background(), subtype)
	if err != nil {
		t.Fatalf("CreateSubtype(%q): %v", key, err)
	}
	return created
}

func createConnection(t *testing.T, fromID, toID repositories.ID, description string) *locations.Connection {
	t.Helper()

	conn := &locations.Connection{
		FromID:      fromID,
		ToID:        toID,
		Description: description,
	}
	if err := testRepo.CreateConnection(context.Background(), conn); err != nil {
		t.Fatalf("CreateConnection: %v", err)
	}
	return conn
}

func childCount(children map[int][]locations.Location) int {
	n := 0
	for _, locs := range children {
		n += len(locs)
	}
	return n
}
