package events

import (
	"github.com/dejitarudemon/mundarium/internal/repositories"
	repoevents "github.com/dejitarudemon/mundarium/internal/repositories/events"
)

// CreateEventOutput is returned after an event has been created.
type CreateEventOutput struct {
	ID         repoevents.ID    `json:"id"`
	Summary    string           `json:"summary"`
	LocationID *repositories.ID `json:"location_id,omitempty"`
}

// GetEventOutput is returned when fetching a single event (with entities and causals).
type GetEventOutput struct {
	ID                repoevents.ID          `json:"id"`
	LocationID        *repositories.ID       `json:"location_id,omitempty"`
	Tags              []string               `json:"tags"`
	Importance        int8                   `json:"importance"`
	CreatedAtGameTime repositories.Ticks     `json:"created_at_game_time"`
	Summary           string                 `json:"summary"`
	Details           repositories.JSON      `json:"details"`
	Entities          []GetEventEntityOutput `json:"entities"`
	Causals           []GetEventCausalOutput `json:"causals"`
}

// FindEventsOutput is the list result for FindEvents.
type FindEventsOutput struct {
	Events []GetEventOutput `json:"events"`
}

// GetEventEntityOutput is an entity–event link.
type GetEventEntityOutput struct {
	EntityID        string              `json:"entity_id"`
	EventID         repoevents.ID       `json:"event_id"`
	Role            string              `json:"role"`
	Belief          *int8               `json:"belief,omitempty"`
	KnownAtGameTime *repositories.Ticks `json:"known_at_game_time,omitempty"`
	Details         repositories.JSON   `json:"details"`
}

// FindEventEntitiesOutput is the list result for FindEventEntities.
type FindEventEntitiesOutput struct {
	Entities []GetEventEntityOutput `json:"entities"`
}

// GetEventCausalOutput is a directed causal edge.
type GetEventCausalOutput struct {
	From  repoevents.ID `json:"from"`
	To    repoevents.ID `json:"to"`
	Cause string        `json:"cause"`
}

// FindEventCausalsOutput is the list result for FindEventCausals.
type FindEventCausalsOutput struct {
	Causals []GetEventCausalOutput `json:"causals"`
}
