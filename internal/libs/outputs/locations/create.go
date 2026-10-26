// Package locations defines output DTOs for location-related use cases.
package locations

import (
	"github.com/dejitarudemon/mundarium/internal/repositories"
)

// CreateLocationOutput is returned after a location has been created.
type CreateLocationOutput struct {
	ID       repositories.ID  `json:"id"`
	ParentID *repositories.ID `json:"parent_id"` // parent location ID; nil for a root
	Name     string           `json:"name"`
}
