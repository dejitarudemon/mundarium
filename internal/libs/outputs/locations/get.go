package locations

import (
	"github.com/dejitarudemon/mundarium/internal/repositories"
)

// GetLocationOutput is returned when fetching a single location.
type GetLocationOutput struct {
	ID          repositories.ID   `json:"id"`
	ParentID    *repositories.ID  `json:"parent_id"` // parent location ID; nil for a root
	Type        string            `json:"type"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Details     repositories.JSON `json:"details"`
}
