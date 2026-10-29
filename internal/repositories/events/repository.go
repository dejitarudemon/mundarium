// Package events provides data-access helpers for event domain models.
package events

import (
	"errors"

	"github.com/dejitarudemon/mundarium/internal/libs/logger"
	"gorm.io/gorm"
)

// ErrEventImportanceOutOfRange is returned when PatchEventImportance receives
// a value outside 0..100 inclusive.
var ErrEventImportanceOutOfRange = errors.New("event importance must be between 0 and 100")

// ErrEventCausalSelfLink is returned when a causal edge would make an event
// a cause of itself (From == To).
var ErrEventCausalSelfLink = errors.New("event cannot be a cause of itself")

// ErrEventCausalIncomplete is returned when a causal edge is missing From or To.
// A cause either exists with both endpoints, or it does not exist at all.
var ErrEventCausalIncomplete = errors.New("event causal requires both from and to")

// ErrEventEntityAlreadyKnown is returned when PatchEventEntityKnownAtGameTime
// is called on a link that already has KnownAtGameTime set.
var ErrEventEntityAlreadyKnown = errors.New("event entity already has known_at_game_time")

// ErrEventEntityNotKnown is returned when PatchEventEntityBelief is called
// before KnownAtGameTime has been established.
var ErrEventEntityNotKnown = errors.New("event entity has no known_at_game_time")

// ErrEventLocationRequired is returned when PatchEventLocation receives a zero location id.
var ErrEventLocationRequired = errors.New("event location id is required")

// EventsRepository persists and queries Event, EventEntity, and EventCausal records.
type EventsRepository struct {
	db     *gorm.DB
	logger logger.Logger
}

// NewEventsRepository creates an EventsRepository backed by db.
// Returns an error if db is nil. logger may be nil.
func NewEventsRepository(db *gorm.DB, logger logger.Logger) (*EventsRepository, error) {
	if db == nil {
		return nil, errors.New("expected *gorm.DB, got nil")
	}
	return &EventsRepository{db: db, logger: logger}, nil
}
