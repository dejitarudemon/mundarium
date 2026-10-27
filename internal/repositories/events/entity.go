package events

import (
	"fmt"

	"github.com/dejitarudemon/mundarium/internal/repositories"
	"github.com/dejitarudemon/mundarium/internal/repositories/entities"
)

// EventEntity links an entity to an event with a role and optional belief/knowledge.
//
// Role describes the entity's relation to the event.
// KnownAtGameTime == nil means the entity does not know about the event in the
// current model; a non-nil value records when knowledge was established.
// Belief == nil means no subjective truth assessment is set; a non-nil Belief is
// a conviction score from 0 to 100. Knowing about an event and assessing its
// truthfulness are independent: Belief may be nil even when KnownAtGameTime is set.
type EventEntity struct {
	EntityID entities.ID `gorm:"type:uuid;primaryKey" json:"entity_id"`
	EventID  ID          `gorm:"type:uuid;primaryKey;index" json:"event_id"`

	Role EventEntityRole `gorm:"not null" json:"role"`

	Belief *int8 `gorm:"check:chk_event_entities_belief,belief IS NULL OR (belief >= 0 AND belief <= 100)" json:"belief,omitempty"`
	// KnownAtGameTime is when knowledge of the event was established.
	KnownAtGameTime *repositories.Ticks `json:"known_at_game_time,omitempty"`
}

// String returns a compact debug representation of the entity–event link.
func (e EventEntity) String() string {
	return fmt.Sprintf(
		"EventEntity{EntityID:%s EventID:%s Role:%s Belief:%v KnownAtGameTime:%v}",
		e.EntityID, e.EventID, e.Role, e.Belief, e.KnownAtGameTime,
	)
}
