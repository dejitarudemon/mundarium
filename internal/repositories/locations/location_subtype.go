package locations

import (
	"github.com/dejitarudemon/mundarium/internal/repositories"
	"github.com/lib/pq"
)

// LocationSubtype describes a reusable template for a kind of location.
//
// For example, a tavern:
//
//	Key: tavern
//	Type: LocationType_STRUCTURE
//	DefiningTags: []string{"rest", "sleep", "enclosed space"}
//	ExpectedTags: []string{"village", "city", "town", "safe", "minigames", "rumors", "quests"}
//	Description: "used for safe, paid recreation"
type LocationSubtype struct {
	ID repositories.ID `gorm:"primaryKey" json:"id"`

	// Key is a lowercase slug; CreateSubtype normalizes case before insert.
	Key string `gorm:"not null;index:location_subtype_key_type" json:"key"`

	DefiningTags pq.StringArray `gorm:"type:text[];not null,default:'{}'" json:"defining_tags"`
	ExpectedTags pq.StringArray `gorm:"type:text[];not null,default:'{}'" json:"expected_tags"`

	Type LocationType `gorm:"not null;index:location_subtype_key_type" json:"type"`

	Description string `gorm:"type:text" json:"description"`
}
