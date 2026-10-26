package locations

import "github.com/dejitarudemon/mundarium/internal/repositories"

// CreateLocationOutput is returned after a location has been created.
type CreateLocationOutput struct {
	ID       repositories.ID  `json:"id"`
	ParentID *repositories.ID `json:"parent_id"` // parent location ID; nil for a root
	Name     string           `json:"name"`
}

// FindChildLocationsOutput maps hierarchy depth to locations at that depth.
// Depth keys follow repository semantics: 1 = direct children, 2 = grandchildren, and so on.
type FindChildLocationsOutput map[int][]GetLocationOutput

// FindLocationsOutput is the scoped search result for FindLocations.
type FindLocationsOutput struct {
	Scope    GetLocationOutput           `json:"scope"`    // search root; always present when the call succeeds
	Children map[int][]GetLocationOutput `json:"children"` // matching descendants by relative depth (1 = direct children)
	HasMore  int                         `json:"has_more"` // number of matching descendants omitted due to Limit
}

// GetLocationOutput is returned when fetching a single location.
type GetLocationOutput struct {
	ID          repositories.ID   `json:"id"`
	ParentID    *repositories.ID  `json:"parent_id"` // parent location ID; nil for a root
	Type        string            `json:"type"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Details     repositories.JSON `json:"details"`
}

// CreateSubtypeOutput is returned after a location subtype has been created.
type CreateSubtypeOutput struct {
	ID  repositories.ID `json:"id"`
	Key string          `json:"key"`
}

// GetSubtypeOutput is returned when fetching a single location subtype.
type GetSubtypeOutput struct {
	ID           repositories.ID `json:"id"`
	Key          string          `json:"key"`
	Type         string          `json:"type"`
	DefiningTags []string        `json:"defining_tags"`
	ExpectedTags []string        `json:"expected_tags"`
	Description  string          `json:"description"`
}

// FindSubtypesOutput is the list result for FindSubtypes.
type FindSubtypesOutput struct {
	Subtypes []GetSubtypeOutput `json:"subtypes"`
}
