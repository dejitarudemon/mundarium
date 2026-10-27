package locations

import "github.com/dejitarudemon/mundarium/internal/repositories"

// CreateLocationInput carries data required to create a new location.
type CreateLocationInput struct {
	ParentID    *repositories.ID  `json:"parent_id"`   // parent location ID; nil creates a root
	Type        string            `json:"type"`        // location type name, e.g. "region", "structure"
	SubtypeID   *repositories.ID  `json:"subtype"`     // optional subtype ID; nil means no subtype
	Name        string            `json:"name"`        // display name
	Description string            `json:"description"` // short description
	Details     repositories.JSON `json:"details"`     // additional structured description
}

type FindLocationsInput struct {
	ScopeID   repositories.ID  `json:"scope_id"` // root of the search area
	Query     *string          `json:"query"`    // optional name/description substring
	Type      *string          `json:"type"`     // optional location type name
	SubtypeID *repositories.ID `json:"subtype"`  // optional subtype ID
	Depth     *int             `json:"depth"`    // 0..maxDepth; nil defaults to 1
	Limit     *int             `json:"limit"`    // 1..100; nil defaults to 20
}

// CreateSubtypeInput carries data required to create a new location subtype.
type CreateSubtypeInput struct {
	Key          string   `json:"key"`           // subtype slug; normalized to lowercase on create
	Type         string   `json:"type"`          // location type name, e.g. "structure"
	DefiningTags []string `json:"defining_tags"` // tags that define this subtype
	ExpectedTags []string `json:"expected_tags"` // tags commonly associated with this subtype
	Description  string   `json:"description"`   // short description
}

// FindSubtypesInput carries optional filters for FindSubtypes.
type FindSubtypesInput struct {
	ID   *repositories.ID `json:"id"`   // optional exact subtype id
	Keys []string         `json:"keys"` // optional key substrings (OR-matched)
	Type *string          `json:"type"` // optional location type name
}

// CreateConnectionInput carries data required to create a new location connection.
type CreateConnectionInput struct {
	FromID        repositories.ID   `json:"from_id"`
	ToID          repositories.ID   `json:"to_id"`
	Bidirectional bool              `json:"bidirectional"`
	State         repositories.JSON `json:"state"`
	Details       repositories.JSON `json:"details"`
	Description   string            `json:"description"`
	ExpiresAt     *int64            `json:"expires_at"`
}

// FindConnectionsInput carries optional filters for FindConnections.
type FindConnectionsInput struct {
	FromID                      *repositories.ID `json:"from_id"`
	ToID                        *repositories.ID `json:"to_id"`
	LocationID                  *repositories.ID `json:"location_id"` // either endpoint
	EitherDirection             bool             `json:"either_direction"`
	IncludeBidirectionalReverse bool             `json:"include_bidirectional_reverse"`
}

// UpdateConnectionInput fully replaces mutable connection fields (not FromID).
// A nil ExpiresAt clears expiry.
type UpdateConnectionInput struct {
	ID            repositories.ID   `json:"id"`
	ToID          repositories.ID   `json:"to_id"`
	Bidirectional bool              `json:"bidirectional"`
	State         repositories.JSON `json:"state"`
	Details       repositories.JSON `json:"details"`
	Description   string            `json:"description"`
	ExpiresAt     *int64            `json:"expires_at"`
}
