package locations

import (
	"github.com/dejitarudemon/mundarium/internal/repositories"
)

// Location is a place in the world hierarchy (plane, region, room, and so on).
// A root location has a nil ParentID.
type Location struct {
	ID       repositories.ID  `gorm:"primaryKey" json:"id"`
	ParentID *repositories.ID `gorm:"index" json:"parent_id"` // parent location ID; nil for a root
	Parent   *Location        `gorm:"foreignKey:ParentID" json:"parent"`

	Type      LocationType     `gorm:"not null" json:"type"`
	SubtypeID *repositories.ID `gorm:"index" json:"subtype_id"`
	Subtype   *LocationSubtype `gorm:"foreignKey:SubtypeID" json:"subtype"`

	Name        string `gorm:"not null" json:"name"`
	Description string `gorm:"text" json:"description"`

	Details repositories.JSON `json:"Details"`
}
