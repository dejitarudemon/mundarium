// Package locations defines input DTOs for location-related use cases.
package locations

import "github.com/dejitarudemon/mundarium/internal/libs/types"

// CreateLocationInput carries data required to create a new location.
type CreateLocationInput struct {
	ParentID    types.ID   // parent location ID (required, non-zero)
	Type        string     // location type name, e.g. "region", "structure"
	SubtypeID   *types.ID  // optional subtype ID; nil means no subtype
	Name        string     // display name
	Description string     // short description
	Details     types.JSON // additional structured description
}
