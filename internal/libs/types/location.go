package types

import (
	"fmt"
)

// LocationType is the hierarchical kind of a location in the world model.
type LocationType uint16

// Known location types, ordered from broadest scope to most specific.
const (
	LocationType_UNKNOWN    LocationType = iota // unrecognized or unset type
	LocationType_PLANE                          // cosmological plane
	LocationType_DEMIPLANE                      // demiplane
	LocationType_PLANELAYER                     // layer within a plane
	LocationType_WORLD                          // world / planet
	LocationType_CONTINENT                      // continent
	LocationType_REGION                         // geographic or political region
	LocationType_SETTLEMENT                     // settlement (city, town, village)
	LocationType_DISTRICT                       // district within a settlement
	LocationType_SITE                           // notable site
	LocationType_STRUCTURE                      // building or structure
	LocationType_ROOM                           // room inside a structure
	LocationType_AREA                           // generic area
)

var (
	fromMapTypes = map[string]LocationType{
		"plane":      LocationType_PLANE,
		"demiplane":  LocationType_DEMIPLANE,
		"planelayer": LocationType_PLANELAYER,
		"world":      LocationType_WORLD,
		"continent":  LocationType_CONTINENT,
		"region":     LocationType_REGION,
		"settlement": LocationType_SETTLEMENT,
		"district":   LocationType_DISTRICT,
		"site":       LocationType_SITE,
		"structure":  LocationType_STRUCTURE,
		"room":       LocationType_ROOM,
		"area":       LocationType_AREA,
		"unkown":     LocationType_UNKNOWN,
	}

	// AvaliableTypes lists all known LocationType values except LocationType_UNKNOWN.
	AvaliableTypes = []LocationType{
		LocationType_PLANE,
		LocationType_DEMIPLANE,
		LocationType_PLANELAYER,
		LocationType_WORLD,
		LocationType_CONTINENT,
		LocationType_REGION,
		LocationType_SETTLEMENT,
		LocationType_DISTRICT,
		LocationType_SITE,
		LocationType_STRUCTURE,
		LocationType_ROOM,
		LocationType_AREA,
	}

	// AvaliableTypesString lists canonical string names for AvaliableTypes.
	AvaliableTypesString = []string{
		"plane",
		"demiplane",
		"planelayer",
		"world",
		"continent",
		"region",
		"settlement",
		"district",
		"site",
		"structure",
		"room",
		"area",
	}
)

// NewLocationType parses s into a LocationType.
// Unknown values map to LocationType_UNKNOWN.
func NewLocationType(s string) (LocationType, error) {
	if t, ok := fromMapTypes[s]; ok {
		return t, nil
	}

	return LocationType_UNKNOWN, fmt.Errorf("unexepcted location type: %v", s)
}

// String returns the canonical name of the location type.
func (t LocationType) String() string {
	switch t {
	case LocationType_PLANE:
		return "plane"
	case LocationType_DEMIPLANE:
		return "demiplane"
	case LocationType_PLANELAYER:
		return "planelayer"
	case LocationType_WORLD:
		return "world"
	case LocationType_CONTINENT:
		return "continent"
	case LocationType_REGION:
		return "region"
	case LocationType_SETTLEMENT:
		return "settlement"
	case LocationType_DISTRICT:
		return "district"
	case LocationType_SITE:
		return "site"
	case LocationType_STRUCTURE:
		return "structure"
	case LocationType_ROOM:
		return "room"
	case LocationType_AREA:
		return "area"
	case LocationType_UNKNOWN:
		return "unknown"
	}

	return fmt.Sprintf("unexpected type (%d)", t)
}
