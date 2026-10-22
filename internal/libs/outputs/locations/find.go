package locations

// FindChildLocationsOutput maps hierarchy depth to locations at that depth.
// Depth keys follow repository semantics: 1 = direct children, 2 = grandchildren, and so on.
type FindChildLocationsOutput map[int][]GetLocationOutput

// FindLocationsOutput is the scoped search result for FindLocations.
type FindLocationsOutput struct {
	Scope    GetLocationOutput           `json:"scope"`     // search root; always present when the call succeeds
	Children map[int][]GetLocationOutput `json:"children"` // matching descendants by relative depth (1 = direct children)
	HasMore  int                         `json:"has_more"`  // number of matching descendants omitted due to Limit
}
