package locations

import "github.com/dejitarudemon/mundarium/internal/libs/types"

type CreateLocationInput struct {
	ParentID    types.ID
	Type        string
	SubtypeID   *types.ID
	Name        string
	Description string
	Details     types.JSON
}
