package events

import (
	"context"
	"fmt"

	"github.com/dejitarudemon/mundarium/internal/repositories"
	"github.com/lib/pq"
	"gorm.io/gorm"
)

// Event is a fact or occurrence in game time that can be referenced by entities.
// Usage should follow the Novikov self-consistency principle.
type Event struct {
	ID ID `gorm:"type:uuid;primaryKey" json:"id"`

	// LocationID is where the event took place (locations.Location.ID).
	// Optional; deleting the event does not delete the location.
	LocationID *repositories.ID `gorm:"index" json:"location_id,omitempty"`

	Tags pq.StringArray `gorm:"type:text[];not null,default:'{}'" json:"tags"` // search tags

	// Importance is fact importance from 0 to 100 inclusive.
	Importance int8 `gorm:"not null;check:chk_events_importance,importance >= 0 AND importance <= 100" json:"importance"`
	// CreatedAtGameTime is the game time when the event occurred.
	CreatedAtGameTime repositories.Ticks `gorm:"not null;index" json:"created_at_game_time"`

	Summary string            `gorm:"type:text;not null" json:"summary"` // short event context
	Details repositories.JSON `gorm:"type:jsonb" json:"details"`         // event details

	// Entities are entity links for this event (EventEntity.EventID).
	Entities []EventEntity `gorm:"foreignKey:EventID;references:ID" json:"entities,omitempty"`
	// Causals are outgoing causal edges where this event is From (the cause);
	// each Causal.To is the effect event.
	Causals []EventCausal `gorm:"foreignKey:From;references:ID" json:"causals,omitempty"`
}

// BeforeCreate always assigns a new event ID and wires nested Causals.From to it.
func (e *Event) BeforeCreate(_ *gorm.DB) error {
	e.ID = New()
	for i := range e.Causals {
		e.Causals[i].From = e.ID
		if err := validateEventCausalEndpoints(e.Causals[i].From, e.Causals[i].To); err != nil {
			return err
		}
	}
	return nil
}

// String returns a compact debug representation of the event.
func (e Event) String() string {
	return fmt.Sprintf(
		"Event{ID:%s LocationID:%v Importance:%d CreatedAtGameTime:%d Summary:%q Tags:%v}",
		e.ID, e.LocationID, e.Importance, e.CreatedAtGameTime, e.Summary, e.Tags,
	)
}

// CreateEvent inserts a new event and returns the persisted record
// (including the ID assigned in BeforeCreate).
// Optional Entities and Causals on event are persisted with the insert when present.
// Nested Causals must include To; From is set to this event in BeforeCreate.
// Causal endpoints are immutable after creation; Cause may be patched.
func (r *EventsRepository) CreateEvent(ctx context.Context, event *Event) (*Event, error) {
	if err := r.db.WithContext(ctx).Create(event).Error; err != nil {
		r.error("failed to create event", "type", "EventsRepository", "func", "CreateEvent", "data", event, "error", err)
		return nil, err
	}
	r.info("event is created", "type", "EventsRepository", "func", "CreateEvent", "data", event)
	return event, nil
}

// GetEvent returns the event with the given id, preloading Entities and Causals.
// Returns gorm.ErrRecordNotFound if no row exists.
func (r *EventsRepository) GetEvent(ctx context.Context, id ID) (*Event, error) {
	var event Event

	if err := r.db.
		WithContext(ctx).
		Preload("Entities").
		Preload("Causals").
		First(&event, "id = ?", id).Error; err != nil {
		r.error("failed to get event", "type", "EventsRepository", "func", "GetEvent", "id", id, "error", err)
		return nil, err
	}

	r.debug("got event", "type", "EventsRepository", "func", "GetEvent", "data", &event)
	return &event, nil
}

// FindEvents returns events matching filter ordered by created_at_game_time, id.
// With an empty filter, all events are returned (subject to Limit when positive).
func (r *EventsRepository) FindEvents(ctx context.Context, filter FindEventsFilter) ([]Event, error) {
	events := make([]Event, 0)
	query := r.db.WithContext(ctx).Model(&Event{})

	if filter.ID != nil {
		query = query.Where("id = ?", *filter.ID)
	}
	if filter.LocationID != nil {
		query = query.Where("location_id = ?", *filter.LocationID)
	}
	if len(filter.Tags) > 0 {
		query = query.Where("tags && ?", pq.Array(filter.Tags))
	}
	if filter.MinImportance != nil {
		query = query.Where("importance >= ?", *filter.MinImportance)
	}
	if filter.MaxImportance != nil {
		query = query.Where("importance <= ?", *filter.MaxImportance)
	}
	if filter.CreatedAtFrom != nil {
		query = query.Where("created_at_game_time >= ?", *filter.CreatedAtFrom)
	}
	if filter.CreatedAtTo != nil {
		query = query.Where("created_at_game_time <= ?", *filter.CreatedAtTo)
	}
	if filter.Query != nil && *filter.Query != "" {
		query = query.Where(
			"summary ILIKE '%' || ? || '%' OR details::text ILIKE '%' || ? || '%'",
			*filter.Query, *filter.Query,
		)
	}
	if filter.Limit > 0 {
		query = query.Limit(filter.Limit)
	}

	if err := query.Order("created_at_game_time, id").Find(&events).Error; err != nil {
		r.error("failed to find events", "type", "EventsRepository", "func", "FindEvents", "filter", filter, "error", err)
		return nil, err
	}

	r.debug("found events", "type", "EventsRepository", "func", "FindEvents", "count", len(events))
	return events, nil
}

// PatchEventImportance sets Importance for the event identified by id.
// Importance must be in 0..100 inclusive; otherwise ErrEventImportanceOutOfRange is returned.
// Returns gorm.ErrRecordNotFound if no row was updated.
func (r *EventsRepository) PatchEventImportance(ctx context.Context, id ID, importance int8) error {
	if importance < 0 || importance > 100 {
		r.error("failed to patch event importance", "type", "EventsRepository", "func", "PatchEventImportance", "id", id, "error", ErrEventImportanceOutOfRange)
		return ErrEventImportanceOutOfRange
	}

	result := r.db.WithContext(ctx).Model(&Event{}).Where("id = ?", id).Update("importance", importance)
	if result.Error != nil {
		r.error("failed to patch event importance", "type", "EventsRepository", "func", "PatchEventImportance", "id", id, "error", result.Error)
		return result.Error
	}
	if result.RowsAffected == 0 {
		r.error("failed to patch event importance", "type", "EventsRepository", "func", "PatchEventImportance", "id", id, "error", gorm.ErrRecordNotFound)
		return gorm.ErrRecordNotFound
	}

	r.info("event importance is patched", "type", "EventsRepository", "func", "PatchEventImportance", "id", id)
	return nil
}

// PatchEventDetails sets Details for the event identified by id.
// Returns gorm.ErrRecordNotFound if no row was updated.
func (r *EventsRepository) PatchEventDetails(ctx context.Context, id ID, details repositories.JSON) error {
	result := r.db.WithContext(ctx).Model(&Event{}).Where("id = ?", id).Update("details", details)
	if result.Error != nil {
		r.error("failed to patch event details", "type", "EventsRepository", "func", "PatchEventDetails", "id", id, "error", result.Error)
		return result.Error
	}
	if result.RowsAffected == 0 {
		r.error("failed to patch event details", "type", "EventsRepository", "func", "PatchEventDetails", "id", id, "error", gorm.ErrRecordNotFound)
		return gorm.ErrRecordNotFound
	}

	r.info("event details are patched", "type", "EventsRepository", "func", "PatchEventDetails", "id", id)
	return nil
}

// PatchEventSummary sets Summary for the event identified by id.
// Returns gorm.ErrRecordNotFound if no row was updated.
func (r *EventsRepository) PatchEventSummary(ctx context.Context, id ID, summary string) error {
	result := r.db.WithContext(ctx).Model(&Event{}).Where("id = ?", id).Update("summary", summary)
	if result.Error != nil {
		r.error("failed to patch event summary", "type", "EventsRepository", "func", "PatchEventSummary", "id", id, "error", result.Error)
		return result.Error
	}
	if result.RowsAffected == 0 {
		r.error("failed to patch event summary", "type", "EventsRepository", "func", "PatchEventSummary", "id", id, "error", gorm.ErrRecordNotFound)
		return gorm.ErrRecordNotFound
	}

	r.info("event summary is patched", "type", "EventsRepository", "func", "PatchEventSummary", "id", id)
	return nil
}

// PatchEventLocation sets LocationID for the event identified by id.
// locationID is required (non-zero); otherwise ErrEventLocationRequired is returned.
// Hierarchy constraints (e.g. descendant-only replacement) belong at the service layer.
// Returns gorm.ErrRecordNotFound if no row was updated.
func (r *EventsRepository) PatchEventLocation(ctx context.Context, id ID, locationID repositories.ID) error {
	if locationID == 0 {
		r.error("failed to patch event location", "type", "EventsRepository", "func", "PatchEventLocation", "id", id, "error", ErrEventLocationRequired)
		return ErrEventLocationRequired
	}

	result := r.db.WithContext(ctx).Model(&Event{}).Where("id = ?", id).Update("location_id", locationID)
	if result.Error != nil {
		r.error("failed to patch event location", "type", "EventsRepository", "func", "PatchEventLocation", "id", id, "location_id", locationID, "error", result.Error)
		return result.Error
	}
	if result.RowsAffected == 0 {
		r.error("failed to patch event location", "type", "EventsRepository", "func", "PatchEventLocation", "id", id, "error", gorm.ErrRecordNotFound)
		return gorm.ErrRecordNotFound
	}

	r.info("event location is patched", "type", "EventsRepository", "func", "PatchEventLocation", "id", id, "location_id", locationID)
	return nil
}

// DeleteEvent deletes the event and cascades to related EventEntity links and
// EventCausal edges where the event is either endpoint.
// The referenced location (LocationID) is not deleted.
// Returns gorm.ErrRecordNotFound if the event does not exist.
func (r *EventsRepository) DeleteEvent(ctx context.Context, id ID) error {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var n int64
		if err := tx.Model(&Event{}).Where("id = ?", id).Count(&n).Error; err != nil {
			return err
		}
		if n == 0 {
			return gorm.ErrRecordNotFound
		}

		if err := tx.Delete(&EventCausal{}, "from_id = ? OR to_id = ?", id, id).Error; err != nil {
			return err
		}
		if err := tx.Delete(&EventEntity{}, "event_id = ?", id).Error; err != nil {
			return err
		}
		return tx.Delete(&Event{}, "id = ?", id).Error
	})
	if err != nil {
		r.error("failed to delete event", "type", "EventsRepository", "func", "DeleteEvent", "id", id, "error", err)
		return err
	}

	r.info("event is deleted", "type", "EventsRepository", "func", "DeleteEvent", "id", id)
	return nil
}
