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
//	Type: repositories.LocationType_STRUCTURE
//	DefiningTags: []string{"rest", "sleep", "enclosed space"}
//	ExpectedTags: []string{"village", "city", "town", "safe", "minigames", "rumors", "quests"}
//	Description: "used for safe, paid recreation"
//
// Or a simple cave room:
//
//	Key: simple cave room
//	DefiningTags: []string{"cave", "enclosed space"}
//	ExpectedTags: []string{"danger", "dark", "meeting", "poor loot", "useless loot", "echo"}
//	Type: repositories.LocationType_ROOM
type LocationSubtype struct {
	ID repositories.ID `gorm:"primaryKey" json:"id"`

	Key string `gorm:"not null;index:location_subtype_key_type" json:"key"` // machine key, e.g. tavern, castle

	DefiningTags pq.StringArray `gorm:"type:text[];not null,default:'{}'" json:"defining_tags"` // tags that define required location behavior
	ExpectedTags pq.StringArray `gorm:"type:text[];not null,default:'{}'" json:"expected_tags"` // tags for expected location behavior

	Type repositories.LocationType `gorm:"not null;index:location_subtype_key_type" json:"type"` // location type this subtype belongs to, e.g. tavern -> STRUCTURE; used when choosing a subtype while creating a location

	Description string `gorm:"type:text" json:"description"` // human-readable description of the subtype
}
