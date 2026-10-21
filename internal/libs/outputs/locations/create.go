// Package locations defines output DTOs for location-related use cases.
package locations

import "github.com/dejitarudemon/mundarium/internal/libs/types"

// CreateLocationOutput is returned after a location has been created.
type CreateLocationOutput struct {
	ID       types.ID // generated location ID
	ParentID types.ID // parent location ID
	Name     string   // display name
}
