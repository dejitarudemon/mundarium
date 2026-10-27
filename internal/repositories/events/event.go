package events

import (
	"fmt"

	"github.com/dejitarudemon/mundarium/internal/repositories"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"gorm.io/gorm"
)

// Event is a fact or occurrence in game time that can be referenced by entities.
type Event struct {
	ID ID `gorm:"type:uuid;primaryKey" json:"id"`

	Tags pq.StringArray `gorm:"type:text[];not null,default:'{}'" json:"tags"` // search tags

	// Importance is fact importance from 0 to 100 inclusive.
	Importance int8 `gorm:"not null;check:chk_events_importance,importance >= 0 AND importance <= 100" json:"importance"`
	// CreatedAtGameTime is the game time when the event occurred.
	CreatedAtGameTime repositories.Ticks `gorm:"not null;index" json:"created_at_game_time"`

	Summary string            `gorm:"type:text;not null" json:"summary"` // short event context
	Details repositories.JSON `gorm:"type:jsonb" json:"details"`         // event details
}

// BeforeCreate assigns a random ID when none was set by the caller.
func (e *Event) BeforeCreate(_ *gorm.DB) error {
	if uuid.UUID(e.ID) == uuid.Nil {
		e.ID = New()
	}
	return nil
}

// String returns a compact debug representation of the event.
func (e Event) String() string {
	return fmt.Sprintf(
		"Event{ID:%s Importance:%d CreatedAtGameTime:%d Summary:%q Tags:%v}",
		e.ID, e.Importance, e.CreatedAtGameTime, e.Summary, e.Tags,
	)
}
