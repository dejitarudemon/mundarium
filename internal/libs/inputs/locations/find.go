package locations

import (
	"github.com/dejitarudemon/mundarium/internal/repositories"
)

// FindLocationsInput carries filters for searching locations under a scope.
type FindLocationsInput struct {
	ScopeID   repositories.ID  `json:"scope_id"` // root of the search area
	Query     *string          `json:"query"`    // optional name/description substring
	Type      *string          `json:"type"`     // optional location type name
	SubtypeID *repositories.ID `json:"subtype"`  // optional subtype ID
	Depth     *int             `json:"depth"`    // 0..maxDepth; nil defaults to 1
	Limit     *int             `json:"limit"`    // 1..100; nil defaults to 20
}
