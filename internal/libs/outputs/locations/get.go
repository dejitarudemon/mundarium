package locations

import "github.com/dejitarudemon/mundarium/internal/libs/types"

// GetLocationOutput is returned when fetching a single location.
type GetLocationOutput struct {
	ID          types.ID   // location ID
	ParentID    types.ID   // parent location ID
	Type        string     // canonical location type name
	Name        string     // display name
	Description string     // short description
	Details     types.JSON // additional structured description
}
