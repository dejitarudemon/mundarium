package events

import (
	"github.com/dejitarudemon/mundarium/internal/repositories"
	repoevents "github.com/dejitarudemon/mundarium/internal/repositories/events"
)

// CreateEventInput carries data required to create a new event.
// Optional Entities are persisted with the insert.
// Nested Causals are incoming: each From is an existing cause event; this event becomes To.
// Causal cycles across distinct events are allowed; a self-link is rejected by the repository.
// Cause endpoints may refer to events that occur later in game time.
type CreateEventInput struct {
	LocationID        *repositories.ID         `json:"location_id"`
	Tags              []string                 `json:"tags"`
	Importance        int8                     `json:"importance"`
	CreatedAtGameTime repositories.Ticks       `json:"created_at_game_time"`
	Summary           string                   `json:"summary"`
	Details           repositories.JSON        `json:"details"`
	Entities          []CreateEventEntityInput `json:"entities"`
	Causals           []CreateEventCausalInput `json:"causals"`
}

// CreateEventEntityInput links a participant to an event at create or add time.
// EntityID is a UUID string (entities.ID). Knowledge fields are set later via patches.
type CreateEventEntityInput struct {
	EntityID string            `json:"entity_id"`
	EventID  repoevents.ID     `json:"event_id"` // required when adding after create; ignored on nested create
	Role     string            `json:"role"`
	Details  repositories.JSON `json:"details"`
}

// CreateEventCausalInput creates a directed causal edge.
// From is the cause event; To is the effect event.
// On nested create_event, only From and Cause are used; To becomes the new event.
// On CreateEventCausal, both From and To are required.
type CreateEventCausalInput struct {
	From  repoevents.ID `json:"from"` // cause event (от какого)
	To    repoevents.ID `json:"to"`   // effect event (к какому); ignored on nested create
	Cause string        `json:"cause"`
}

// FindEventsInput carries optional filters for FindEvents.
type FindEventsInput struct {
	ID            *repoevents.ID      `json:"id"`
	LocationID    *repositories.ID    `json:"location_id"`
	Tags          []string            `json:"tags"`
	MinImportance *int8               `json:"min_importance"`
	MaxImportance *int8               `json:"max_importance"`
	CreatedAtFrom *repositories.Ticks `json:"created_at_from"`
	CreatedAtTo   *repositories.Ticks `json:"created_at_to"`
	Query         *string             `json:"query"` // substring match against summary or details
	Limit         *int                `json:"limit"` // 1..100; nil defaults to 20
}

// FindEventEntitiesInput carries optional filters for FindEventEntities.
type FindEventEntitiesInput struct {
	EventID   *repoevents.ID `json:"event_id"`
	EntityID  *string        `json:"entity_id"`
	Role      *string        `json:"role"`
	KnownOnly bool           `json:"known_only"`
	Limit     *int           `json:"limit"` // 1..100; nil defaults to 20
}

// FindEventCausalsInput carries optional filters for FindEventCausals.
type FindEventCausalsInput struct {
	EventID *repoevents.ID `json:"event_id"`
	From    *repoevents.ID `json:"from"`
	To      *repoevents.ID `json:"to"`
	Limit   *int           `json:"limit"` // 1..100; nil defaults to 20
}
