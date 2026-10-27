package locations

import (
	"context"
	"strings"

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

// CreateSubtype inserts a new location subtype and returns the persisted record
// (including the generated ID and any DB defaults). Key is normalized to lowercase.
func (r *LocationRepository) CreateSubtype(ctx context.Context, subtype *LocationSubtype) (*LocationSubtype, error) {
	subtype.Key = strings.ToLower(subtype.Key)

	if err := r.db.WithContext(ctx).Create(subtype).Error; err != nil {
		r.error("failed to create subtype", "type", "LocationRepository", "func", "CreateSubtype", "data", subtype, "error", err)
		return nil, err
	}
	r.info("subtype is created", "type", "LocationRepository", "func", "CreateSubtype", "data", subtype)
	return subtype, nil
}

// GetSubtype returns the location subtype with the given id.
// Returns gorm.ErrRecordNotFound if no row exists.
func (r *LocationRepository) GetSubtype(ctx context.Context, id repositories.ID) (*LocationSubtype, error) {
	var subtype LocationSubtype

	if err := r.db.
		WithContext(ctx).
		First(&subtype, "id = ?", id).Error; err != nil {
		r.error("failed to get subtype", "type", "LocationRepository", "func", "GetSubtype", "id", id, "error", err)
		return nil, err
	}

	r.debug("got subtype", "type", "LocationRepository", "func", "GetSubtype", "data", &subtype)
	return &subtype, nil
}

// FindSubtypes returns subtypes matching filter. Nil/empty fields are ignored.
// Keys are normalized to lowercase and OR-matched as substrings (LIKE).
// Other filters are combined with AND. With an empty filter, all subtypes are returned.
func (r *LocationRepository) FindSubtypes(ctx context.Context, filter FindSubtypesFilter) ([]LocationSubtype, error) {
	var subtypes []LocationSubtype

	query := r.db.WithContext(ctx).Model(&LocationSubtype{})

	if filter.ID != nil {
		query = query.Where("id = ?", *filter.ID)
	}
	if keys := normalizeSubtypeKeys(filter.Keys); len(keys) > 0 {
		or := r.db.Where("key LIKE ?", "%"+keys[0]+"%")
		for _, key := range keys[1:] {
			or = or.Or("key LIKE ?", "%"+key+"%")
		}
		query = query.Where(or)
	}
	if filter.Type != nil {
		query = query.Where("type = ?", *filter.Type)
	}

	if err := query.Order("id").Find(&subtypes).Error; err != nil {
		r.error("failed to find subtypes", "type", "LocationRepository", "func", "FindSubtypes", "filter", filter, "error", err)
		return nil, err
	}

	r.debug("found subtypes", "type", "LocationRepository", "func", "FindSubtypes", "count", len(subtypes))
	return subtypes, nil
}

func normalizeSubtypeKeys(keys []string) []string {
	out := make([]string, 0, len(keys))
	for _, key := range keys {
		key = strings.ToLower(strings.TrimSpace(key))
		if key == "" {
			continue
		}
		out = append(out, key)
	}
	return out
}
