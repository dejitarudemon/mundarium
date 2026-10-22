// Package locations defines domain models for places in the world and links between them.
package locations

import (
	"github.com/dejitarudemon/mundarium/internal/libs/types"
)

// Location is a place in the world hierarchy (plane, region, room, and so on).
// A root location has a nil ParentID.
type Location struct {
	ID       types.ID  `gorm:"primaryKey" json:"id"`
	ParentID *types.ID `gorm:"index" json:"parent_id"` // parent location ID; nil for a root

	Type      types.LocationType `gorm:"not null" json:"type"`                // location type; see types.LocationType
	SubtypeID *types.ID          `gorm:"index" json:"subtype_id"`             // optional ID of the location subtype
	Subtype   *LocationSubtype   `gorm:"foreignKey:SubtypeID" json:"subtype"` // resolved location subtype

	Name        string `gorm:"not null" json:"name"`    // display name
	Description string `gorm:"text" json:"description"` // short description of the location

	Details types.JSON `json:"Details"` // additional structured description
}
