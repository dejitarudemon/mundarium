package locations

// FindChildLocationsOutput maps hierarchy depth to locations at that depth.
// Depth keys follow repository semantics: 1 = direct children, 2 = grandchildren, and so on.
type FindChildLocationsOutput map[int][]GetLocationOutput
