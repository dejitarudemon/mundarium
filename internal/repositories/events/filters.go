package events

import (
	"github.com/dejitarudemon/mundarium/internal/repositories"
	"github.com/dejitarudemon/mundarium/internal/repositories/entities"
)

// FindEventsFilter selects events. Nil/empty fields are ignored.
type FindEventsFilter struct {
	ID            *ID                 // exact event id match
	LocationID    *repositories.ID    // exact location match
	Tags          []string            // event must share at least one of these tags
	MinImportance *int8               // inclusive
	MaxImportance *int8               // inclusive
	CreatedAtFrom *repositories.Ticks // inclusive
	CreatedAtTo   *repositories.Ticks // inclusive
	// Query, when set, matches a case-insensitive substring in summary or details::text.
	Query *string
	// Limit caps the number of returned rows. Values <= 0 mean no limit.
	Limit int
}

// FindEventEntitiesFilter selects entity–event links. Nil fields are ignored.
type FindEventEntitiesFilter struct {
	EventID  *ID
	EntityID *entities.ID
	Role     *EventEntityRole
	// KnownOnly, when true, keeps only rows with KnownAtGameTime set.
	KnownOnly bool
	// Limit caps the number of returned rows. Values <= 0 mean no limit.
	Limit int
}

// FindEventCausalsFilter selects causal edges. Nil fields are ignored.
//
// EventID, when set, matches rows where the event is either endpoint and
// ignores From/To.
type FindEventCausalsFilter struct {
	EventID *ID
	From    *ID
	To      *ID
	// Limit caps the number of returned rows. Values <= 0 mean no limit.
	Limit int
}
