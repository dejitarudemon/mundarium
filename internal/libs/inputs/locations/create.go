// Package locations defines input DTOs for location-related use cases.
package locations

import (
	"github.com/dejitarudemon/mundarium/internal/repositories"
)

// CreateLocationInput carries data required to create a new location.
type CreateLocationInput struct {
	ParentID    *repositories.ID  `json:"parent_id"`   // parent location ID; nil creates a root
	Type        string            `json:"type"`        // location type name, e.g. "region", "structure"
	SubtypeID   *repositories.ID  `json:"subtype"`     // optional subtype ID; nil means no subtype
	Name        string            `json:"name"`        // display name
	Description string            `json:"description"` // short description
	Details     repositories.JSON `json:"details"`     // additional structured description
}
