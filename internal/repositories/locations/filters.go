package locations

import "github.com/dejitarudemon/mundarium/internal/repositories"

// FindLocationsFilter selects locations under ScopeID with optional filters.
//
// Depth semantics (relative to ScopeID):
//   - 0: only the scope location
//   - 1: scope + direct children
//   - n: scope + descendants up to relative depth n (capped by the repository max depth)
type FindLocationsFilter struct {
	ScopeID   repositories.ID
	Query     *string
	Type      *LocationType
	SubtypeID *repositories.ID
	Depth     int
}

// FindLocationsResult is the scope plus matching descendants grouped by relative depth.
// Depth keys: 1 = direct children, 2 = grandchildren, and so on.
type FindLocationsResult struct {
	Scope    Location
	Children map[int][]Location
}

// FindSubtypesFilter selects location subtypes by optional id, key substrings, and type.
// Nil/empty fields are ignored. Keys are OR-matched with lowercase substring LIKE.
type FindSubtypesFilter struct {
	ID   *repositories.ID
	Keys []string
	Type *LocationType
}

// FindConnectionsFilter selects connections by optional endpoints.
// Nil fields are ignored. An empty filter returns all connections.
//
// LocationID, when set, matches rows where that location is From or To and
// takes precedence over FromID/ToID flags.
//
// When both FromID and ToID are set:
//   - default: exact directed pair
//   - EitherDirection: also match the reverse directed pair
//   - IncludeBidirectionalReverse: also match reverse rows marked bidirectional
type FindConnectionsFilter struct {
	FromID *repositories.ID
	ToID   *repositories.ID
	// LocationID matches connections involving this location as either endpoint.
	LocationID *repositories.ID
	// EitherDirection also returns ToID -> FromID when both endpoints are set.
	EitherDirection bool
	// IncludeBidirectionalReverse also returns ToID -> FromID when that row is bidirectional.
	IncludeBidirectionalReverse bool
}
