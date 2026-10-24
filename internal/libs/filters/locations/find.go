// Package locations defines repository filters for location queries.
package locations

import (
	"github.com/dejitarudemon/mundarium/internal/libs/locations"
	"github.com/dejitarudemon/mundarium/internal/libs/types"
)

// FindLocationsFilter selects locations under ScopeID with optional filters.
//
// Depth semantics (relative to ScopeID):
//   - 0: only the scope location
//   - 1: scope + direct children
//   - n: scope + descendants up to relative depth n (capped by the repository max depth)
type FindLocationsFilter struct {
	ScopeID   types.ID
	Query     *string
	Type      *types.LocationType
	SubtypeID *types.ID
	Depth     int
}

// FindLocationsResult is the scope plus matching descendants grouped by relative depth.
// Depth keys: 1 = direct children, 2 = grandchildren, and so on.
type FindLocationsResult struct {
	Scope    locations.Location
	Children map[int][]locations.Location
}
