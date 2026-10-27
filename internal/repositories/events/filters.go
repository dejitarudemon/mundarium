package events

import (
	"github.com/dejitarudemon/mundarium/internal/repositories"
	"github.com/dejitarudemon/mundarium/internal/repositories/entities"
)

// FindEventsFilter selects events. Nil/empty fields are ignored.
type FindEventsFilter struct {
	LocationID    *repositories.ID    // exact location match
	Tags          []string            // event must share at least one of these tags
	MinImportance *int8               // inclusive
	MaxImportance *int8               // inclusive
	CreatedAtFrom *repositories.Ticks // inclusive
	CreatedAtTo   *repositories.Ticks // inclusive
}

// FindEventEntitiesFilter selects entity–event links. Nil fields are ignored.
type FindEventEntitiesFilter struct {
	EventID  *ID
	EntityID *entities.ID
	Role     *EventEntityRole
	// KnownOnly, when true, keeps only rows with KnownAtGameTime set.
	KnownOnly bool
}

// FindEventCausalsFilter selects causal edges. Nil fields are ignored.
//
// EventID, when set, matches rows where the event is either endpoint and
// ignores From/To.
type FindEventCausalsFilter struct {
	EventID *ID
	From    *ID
	To      *ID
}
