package locations

import (
	"errors"

	input "github.com/dejitarudemon/mundarium/internal/libs/inputs/locations"
	"github.com/dejitarudemon/mundarium/internal/libs/locations"
	output "github.com/dejitarudemon/mundarium/internal/libs/outputs/locations"
	"github.com/dejitarudemon/mundarium/internal/libs/types"
)

// toNewLocation maps CreateLocationInput to a domain Location ready for insert.
func toNewLocation(input input.CreateLocationInput) (*locations.Location, error) {
	if input.ParentID == 0 {
		return nil, errors.New("parent id is zero")
	}

	t, err := types.NewLocationType(input.Type)
	if err != nil {
		return nil, err
	}

	if input.SubtypeID != nil && *input.SubtypeID == 0 {
		return nil, errors.New("subtype id is zero")
	}

	return &locations.Location{
		ParentID:    input.ParentID,
		Type:        t,
		SubtypeID:   input.SubtypeID,
		Name:        input.Name,
		Description: input.Description,
		Details:     input.Details,
	}, nil
}

// toCreatedLocation maps a persisted Location to CreateLocationOutput.
func toCreatedLocation(location locations.Location) (output.CreateLocationOutput, error) {
	if err := checkLocation(location); err != nil {
		return output.CreateLocationOutput{}, err
	}

	return output.CreateLocationOutput{
		ID:       location.ID,
		ParentID: location.ParentID,
		Name:     location.Name,
	}, nil
}

// toGotLocation maps a persisted Location to GetLocationOutput.
func toGotLocation(location locations.Location) (output.GetLocationOutput, error) {
	if err := checkLocation(location); err != nil {
		return output.GetLocationOutput{}, err
	}

	return output.GetLocationOutput{
		ID:          location.ID,
		ParentID:    location.ParentID,
		Type:        location.Type.String(),
		Name:        location.Name,
		Description: location.Description,
		Details:     location.Details,
	}, nil
}

// toGotChildLocations maps descendants grouped by depth to FindChildLocationsOutput.
func toGotChildLocations(children map[int][]locations.Location) (output.FindChildLocationsOutput, error) {
	result := make(output.FindChildLocationsOutput, len(children))

	for depth, locs := range children {
		level := make([]output.GetLocationOutput, 0, len(locs))
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

// checkLocation validates required identity fields on a Location.
func checkLocation(location locations.Location) error {
	if location.ID == 0 {
		return errors.New("location id is zero")
	}
	if location.ParentID == 0 {
		return errors.New("parent id is zero")
	}

	return nil
}
