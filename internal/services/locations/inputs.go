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
