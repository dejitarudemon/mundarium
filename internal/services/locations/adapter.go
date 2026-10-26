package locations

import (
	"errors"
	"strings"

	"github.com/lib/pq"

	repolocations "github.com/dejitarudemon/mundarium/internal/repositories/locations"
)

// toNewLocation maps CreateLocationInput to a domain Location ready for insert.
func toNewLocation(input CreateLocationInput) (*repolocations.Location, error) {
	if input.ParentID != nil && *input.ParentID == 0 {
		return nil, errors.New("parent id is zero")
	}

	t, err := repolocations.NewLocationType(input.Type)
	if err != nil {
		return nil, err
	}

	if input.SubtypeID != nil && *input.SubtypeID == 0 {
		return nil, errors.New("subtype id is zero")
	}

	return &repolocations.Location{
		ParentID:    input.ParentID,
		Type:        t,
		SubtypeID:   input.SubtypeID,
		Name:        input.Name,
		Description: input.Description,
		Details:     input.Details,
	}, nil
}

// toCreatedLocation maps a persisted Location to CreateLocationOutput.
func toCreatedLocation(location repolocations.Location) (CreateLocationOutput, error) {
	if err := checkLocation(location); err != nil {
		return CreateLocationOutput{}, err
	}

	return CreateLocationOutput{
		ID:       location.ID,
		ParentID: location.ParentID,
		Name:     location.Name,
	}, nil
}

// toGotLocation maps a persisted Location to GetLocationOutput.
func toGotLocation(location repolocations.Location) (GetLocationOutput, error) {
	if err := checkLocation(location); err != nil {
		return GetLocationOutput{}, err
	}

	return GetLocationOutput{
		ID:          location.ID,
		ParentID:    location.ParentID,
		Type:        location.Type.String(),
		Name:        location.Name,
		Description: location.Description,
		Details:     location.Details,
	}, nil
}

// toGotChildLocations maps descendants grouped by depth to FindChildLocationsOutput.
func toGotChildLocations(children map[int][]repolocations.Location) (FindChildLocationsOutput, error) {
	result := make(FindChildLocationsOutput, len(children))

	for depth, locs := range children {
		level := make([]GetLocationOutput, 0, len(locs))
		for _, loc := range locs {
			item, err := toGotLocation(loc)
			if err != nil {
				return nil, err
			}
			level = append(level, item)
		}
		result[depth] = level
	}

	return result, nil
}

const (
	defaultFindLocationsDepth = 1
	defaultFindLocationsLimit = 20
	maxFindLocationsLimit     = 100
)

// toFindLocationsFilter maps FindLocationsInput to a repository filter and page limit.
// maxDepth is the inclusive upper bound for Depth (from LocationRepository.MaxDepth).
func toFindLocationsFilter(in FindLocationsInput, maxDepth int) (repolocations.FindLocationsFilter, int, error) {
	if in.ScopeID == 0 {
		return repolocations.FindLocationsFilter{}, 0, errors.New("scope id is zero")
	}
	if in.SubtypeID != nil && *in.SubtypeID == 0 {
		return repolocations.FindLocationsFilter{}, 0, errors.New("subtype id is zero")
	}

	depth := defaultFindLocationsDepth
	if in.Depth != nil {
		depth = *in.Depth
	}
	if depth < 0 || depth > maxDepth {
		return repolocations.FindLocationsFilter{}, 0, errors.New("depth out of range")
	}

	limit := defaultFindLocationsLimit
	if in.Limit != nil {
		limit = *in.Limit
	}
	if limit < 1 || limit > maxFindLocationsLimit {
		return repolocations.FindLocationsFilter{}, 0, errors.New("limit out of range")
	}

	filter := repolocations.FindLocationsFilter{
		ScopeID:   in.ScopeID,
		Query:     in.Query,
		SubtypeID: in.SubtypeID,
		Depth:     depth,
	}

	if in.Type != nil && *in.Type != "" {
		t, err := repolocations.NewLocationType(*in.Type)
		if err != nil {
			return repolocations.FindLocationsFilter{}, 0, err
		}
		filter.Type = &t
	}

	return filter, limit, nil
}

// toFoundLocations maps a FindLocationsResult to FindLocationsOutput.
// Limit applies only to matching descendants (scope is always included).
// HasMore is the number of matching descendants omitted due to limit.
func toFoundLocations(found repolocations.FindLocationsResult, limit int) (FindLocationsOutput, error) {
	scope, err := toGotLocation(found.Scope)
	if err != nil {
		return FindLocationsOutput{}, err
	}

	type entry struct {
		depth int
		loc   repolocations.Location
	}
	flat := make([]entry, 0)
	maxDepth := 0
	for depth := range found.Children {
		if depth > maxDepth {
			maxDepth = depth
		}
	}
	for depth := 1; depth <= maxDepth; depth++ {
		for _, loc := range found.Children[depth] {
			flat = append(flat, entry{depth: depth, loc: loc})
		}
	}

	omitted := 0
	if len(flat) > limit {
		omitted = len(flat) - limit
		flat = flat[:limit]
	}

	children := make(map[int][]GetLocationOutput)
	for _, e := range flat {
		item, err := toGotLocation(e.loc)
		if err != nil {
			return FindLocationsOutput{}, err
		}
		children[e.depth] = append(children[e.depth], item)
	}

	return FindLocationsOutput{
		Scope:    scope,
		Children: children,
		HasMore:  omitted,
	}, nil
}

// checkLocation validates required identity fields on a Location.
func checkLocation(location repolocations.Location) error {
	if location.ID == 0 {
		return errors.New("location id is zero")
	}
	if location.ParentID != nil && *location.ParentID == 0 {
		return errors.New("parent id is zero")
	}

	return nil
}

// toNewSubtype maps CreateSubtypeInput to a domain LocationSubtype ready for insert.
func toNewSubtype(input CreateSubtypeInput) (*repolocations.LocationSubtype, error) {
	key := strings.TrimSpace(input.Key)
	if key == "" {
		return nil, errors.New("key is empty")
	}

	t, err := repolocations.NewLocationType(input.Type)
	if err != nil {
		return nil, err
	}

	return &repolocations.LocationSubtype{
		Key:          key,
		Type:         t,
		DefiningTags: stringArrayOrEmpty(input.DefiningTags),
		ExpectedTags: stringArrayOrEmpty(input.ExpectedTags),
		Description:  input.Description,
	}, nil
}

// toCreatedSubtype maps a persisted LocationSubtype to CreateSubtypeOutput.
func toCreatedSubtype(subtype repolocations.LocationSubtype) (CreateSubtypeOutput, error) {
	if err := checkSubtype(subtype); err != nil {
		return CreateSubtypeOutput{}, err
	}

	return CreateSubtypeOutput{
		ID:  subtype.ID,
		Key: subtype.Key,
	}, nil
}

// toGotSubtype maps a persisted LocationSubtype to GetSubtypeOutput.
func toGotSubtype(subtype repolocations.LocationSubtype) (GetSubtypeOutput, error) {
	if err := checkSubtype(subtype); err != nil {
		return GetSubtypeOutput{}, err
	}

	return GetSubtypeOutput{
		ID:           subtype.ID,
		Key:          subtype.Key,
		Type:         subtype.Type.String(),
		DefiningTags: tagsOrEmpty(subtype.DefiningTags),
		ExpectedTags: tagsOrEmpty(subtype.ExpectedTags),
		Description:  subtype.Description,
	}, nil
}

// toFindSubtypesFilter maps FindSubtypesInput to a repository filter.
func toFindSubtypesFilter(in FindSubtypesInput) (repolocations.FindSubtypesFilter, error) {
	if in.ID != nil && *in.ID == 0 {
		return repolocations.FindSubtypesFilter{}, errors.New("subtype id is zero")
	}

	filter := repolocations.FindSubtypesFilter{
		ID:   in.ID,
		Keys: in.Keys,
	}

	if in.Type != nil && *in.Type != "" {
		t, err := repolocations.NewLocationType(*in.Type)
		if err != nil {
			return repolocations.FindSubtypesFilter{}, err
		}
		filter.Type = &t
	}

	return filter, nil
}

// toFoundSubtypes maps persisted subtypes to FindSubtypesOutput.
func toFoundSubtypes(subtypes []repolocations.LocationSubtype) (FindSubtypesOutput, error) {
	out := make([]GetSubtypeOutput, 0, len(subtypes))
	for _, subtype := range subtypes {
		item, err := toGotSubtype(subtype)
		if err != nil {
			return FindSubtypesOutput{}, err
		}
		out = append(out, item)
	}
	return FindSubtypesOutput{Subtypes: out}, nil
}

// checkSubtype validates required identity fields on a LocationSubtype.
func checkSubtype(subtype repolocations.LocationSubtype) error {
	if subtype.ID == 0 {
		return errors.New("subtype id is zero")
	}
	if strings.TrimSpace(subtype.Key) == "" {
		return errors.New("key is empty")
	}
	return nil
}

func stringArrayOrEmpty(v []string) pq.StringArray {
	if v == nil {
		return pq.StringArray{}
	}
	return pq.StringArray(v)
}

func tagsOrEmpty(v pq.StringArray) []string {
	if v == nil {
		return []string{}
	}
	return []string(v)
}
