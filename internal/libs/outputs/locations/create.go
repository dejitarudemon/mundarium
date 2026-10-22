// Package locations defines output DTOs for location-related use cases.
package locations

import "github.com/dejitarudemon/mundarium/internal/libs/types"

// CreateLocationOutput is returned after a location has been created.
type CreateLocationOutput struct {
	ID       types.ID  `json:"id"`
	ParentID *types.ID `json:"parent_id"` // parent location ID; nil for a root
	Name     string    `json:"name"`
}
