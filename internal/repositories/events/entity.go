package events

import (
	"context"
	"fmt"

	"github.com/dejitarudemon/mundarium/internal/repositories"
	"github.com/dejitarudemon/mundarium/internal/repositories/entities"
	"gorm.io/gorm"
)

// EventEntity links an entity to an event with a role and optional belief/knowledge.
//
// Role may be changed via PatchEventEntityRole at any time.
// KnownAtGameTime == nil means the entity does not yet know about the event.
// Knowledge is established once via PatchEventEntityKnownAtGameTime (sets KnownAtGameTime and Belief).
// After that, Belief is updated with PatchEventEntityBelief; KnownAtGameTime is not changed.
// Belief is a conviction score from 0 to 100 when set.
type EventEntity struct {
	EntityID entities.ID `gorm:"type:uuid;primaryKey" json:"entity_id"`
	EventID  ID          `gorm:"type:uuid;primaryKey;index" json:"event_id"`

	Role EventEntityRole `gorm:"not null" json:"role"`

	Belief *int8 `gorm:"check:chk_event_entities_belief,belief IS NULL OR (belief >= 0 AND belief <= 100)" json:"belief,omitempty"`
	// KnownAtGameTime is when knowledge of the event was established.
	KnownAtGameTime *repositories.Ticks `json:"known_at_game_time,omitempty"`

	// Details holds contextual JSON about how/when the entity learned of the event, and related notes.
	Details repositories.JSON `gorm:"type:jsonb" json:"details"`
}

// String returns a compact debug representation of the entity–event link.
func (e EventEntity) String() string {
	return fmt.Sprintf(
		"EventEntity{EntityID:%s EventID:%s Role:%s Belief:%v KnownAtGameTime:%v Details:%s}",
		e.EntityID, e.EventID, e.Role, e.Belief, e.KnownAtGameTime, e.Details,
	)
}

// CreateEventEntity inserts a new entity–event link.
func (r *EventsRepository) CreateEventEntity(ctx context.Context, link *EventEntity) error {
	if err := r.db.WithContext(ctx).Create(link).Error; err != nil {
		r.error("failed to create event entity", "type", "EventsRepository", "func", "CreateEventEntity", "data", link, "error", err)
		return err
	}
	r.info("event entity is created", "type", "EventsRepository", "func", "CreateEventEntity", "data", link)
	return nil
}

// GetEventEntity returns the link for the given entity and event ids.
// Returns gorm.ErrRecordNotFound if no row exists.
func (r *EventsRepository) GetEventEntity(ctx context.Context, entityID entities.ID, eventID ID) (*EventEntity, error) {
	var link EventEntity

	if err := r.db.WithContext(ctx).
		First(&link, "entity_id = ? AND event_id = ?", entityID, eventID).Error; err != nil {
		r.error("failed to get event entity", "type", "EventsRepository", "func", "GetEventEntity", "entity_id", entityID, "event_id", eventID, "error", err)
		return nil, err
	}

	r.debug("got event entity", "type", "EventsRepository", "func", "GetEventEntity", "data", &link)
	return &link, nil
}

// FindEventEntities returns links matching filter ordered by event_id, entity_id.
// With an empty filter, all links are returned.
func (r *EventsRepository) FindEventEntities(ctx context.Context, filter FindEventEntitiesFilter) ([]EventEntity, error) {
	links := make([]EventEntity, 0)
	query := r.db.WithContext(ctx).Model(&EventEntity{})

	if filter.EventID != nil {
		query = query.Where("event_id = ?", *filter.EventID)
	}
	if filter.EntityID != nil {
		query = query.Where("entity_id = ?", *filter.EntityID)
	}
	if filter.Role != nil {
		query = query.Where("role = ?", *filter.Role)
	}
	if filter.KnownOnly {
		query = query.Where("known_at_game_time IS NOT NULL")
	}

	if err := query.Order("event_id, entity_id").Find(&links).Error; err != nil {
		r.error("failed to find event entities", "type", "EventsRepository", "func", "FindEventEntities", "filter", filter, "error", err)
		return nil, err
	}

	r.debug("found event entities", "type", "EventsRepository", "func", "FindEventEntities", "count", len(links))
	return links, nil
}

// PatchEventEntityRole sets Role for the given entity–event link.
// Returns gorm.ErrRecordNotFound if no row was updated.
func (r *EventsRepository) PatchEventEntityRole(ctx context.Context, entityID entities.ID, eventID ID, role EventEntityRole) error {
	result := r.db.WithContext(ctx).
		Model(&EventEntity{}).
		Where("entity_id = ? AND event_id = ?", entityID, eventID).
		Update("role", role)
	if result.Error != nil {
		r.error("failed to patch event entity role", "type", "EventsRepository", "func", "PatchEventEntityRole", "entity_id", entityID, "event_id", eventID, "error", result.Error)
		return result.Error
	}
	if result.RowsAffected == 0 {
		r.error("failed to patch event entity role", "type", "EventsRepository", "func", "PatchEventEntityRole", "entity_id", entityID, "event_id", eventID, "error", gorm.ErrRecordNotFound)
		return gorm.ErrRecordNotFound
	}

	r.info("event entity role is patched", "type", "EventsRepository", "func", "PatchEventEntityRole", "entity_id", entityID, "event_id", eventID)
	return nil
}

// PatchEventEntityKnownAtGameTime establishes knowledge: sets KnownAtGameTime and Belief together.
// Both values are required. Fails with ErrEventEntityAlreadyKnown if knowledge was already set.
// Returns gorm.ErrRecordNotFound if no row exists.
func (r *EventsRepository) PatchEventEntityKnownAtGameTime(ctx context.Context, entityID entities.ID, eventID ID, knownAt repositories.Ticks, belief int8) error {
	var existing EventEntity
	if err := r.db.WithContext(ctx).
		First(&existing, "entity_id = ? AND event_id = ?", entityID, eventID).Error; err != nil {
		r.error("failed to patch event entity known_at", "type", "EventsRepository", "func", "PatchEventEntityKnownAtGameTime", "entity_id", entityID, "event_id", eventID, "error", err)
		return err
	}
	if existing.KnownAtGameTime != nil {
		r.error("failed to patch event entity known_at", "type", "EventsRepository", "func", "PatchEventEntityKnownAtGameTime", "entity_id", entityID, "event_id", eventID, "error", ErrEventEntityAlreadyKnown)
		return ErrEventEntityAlreadyKnown
	}

	result := r.db.WithContext(ctx).
		Model(&EventEntity{}).
		Where("entity_id = ? AND event_id = ?", entityID, eventID).
		Select("KnownAtGameTime", "Belief").
		Updates(EventEntity{
			KnownAtGameTime: &knownAt,
			Belief:          &belief,
		})
	if result.Error != nil {
		r.error("failed to patch event entity known_at", "type", "EventsRepository", "func", "PatchEventEntityKnownAtGameTime", "entity_id", entityID, "event_id", eventID, "error", result.Error)
		return result.Error
	}
	if result.RowsAffected == 0 {
		r.error("failed to patch event entity known_at", "type", "EventsRepository", "func", "PatchEventEntityKnownAtGameTime", "entity_id", entityID, "event_id", eventID, "error", gorm.ErrRecordNotFound)
		return gorm.ErrRecordNotFound
	}

	r.info("event entity known_at is patched", "type", "EventsRepository", "func", "PatchEventEntityKnownAtGameTime", "entity_id", entityID, "event_id", eventID)
	return nil
}

// PatchEventEntityBelief sets Belief for a link that already has KnownAtGameTime.
// Returns ErrEventEntityNotKnown if KnownAtGameTime is still nil.
// Returns gorm.ErrRecordNotFound if no row exists.
func (r *EventsRepository) PatchEventEntityBelief(ctx context.Context, entityID entities.ID, eventID ID, belief int8) error {
	var existing EventEntity
	if err := r.db.WithContext(ctx).
		First(&existing, "entity_id = ? AND event_id = ?", entityID, eventID).Error; err != nil {
		r.error("failed to patch event entity belief", "type", "EventsRepository", "func", "PatchEventEntityBelief", "entity_id", entityID, "event_id", eventID, "error", err)
		return err
	}
	if existing.KnownAtGameTime == nil {
		r.error("failed to patch event entity belief", "type", "EventsRepository", "func", "PatchEventEntityBelief", "entity_id", entityID, "event_id", eventID, "error", ErrEventEntityNotKnown)
		return ErrEventEntityNotKnown
	}

	result := r.db.WithContext(ctx).
		Model(&EventEntity{}).
		Where("entity_id = ? AND event_id = ?", entityID, eventID).
		Update("belief", belief)
	if result.Error != nil {
		r.error("failed to patch event entity belief", "type", "EventsRepository", "func", "PatchEventEntityBelief", "entity_id", entityID, "event_id", eventID, "error", result.Error)
		return result.Error
	}
	if result.RowsAffected == 0 {
		r.error("failed to patch event entity belief", "type", "EventsRepository", "func", "PatchEventEntityBelief", "entity_id", entityID, "event_id", eventID, "error", gorm.ErrRecordNotFound)
		return gorm.ErrRecordNotFound
	}

	r.info("event entity belief is patched", "type", "EventsRepository", "func", "PatchEventEntityBelief", "entity_id", entityID, "event_id", eventID)
	return nil
}

// PatchEventEntityDetails sets Details for the given entity–event link.
// Returns gorm.ErrRecordNotFound if no row was updated.
func (r *EventsRepository) PatchEventEntityDetails(ctx context.Context, entityID entities.ID, eventID ID, details repositories.JSON) error {
	result := r.db.WithContext(ctx).
		Model(&EventEntity{}).
		Where("entity_id = ? AND event_id = ?", entityID, eventID).
		Update("details", details)
	if result.Error != nil {
		r.error("failed to patch event entity details", "type", "EventsRepository", "func", "PatchEventEntityDetails", "entity_id", entityID, "event_id", eventID, "error", result.Error)
		return result.Error
	}
	if result.RowsAffected == 0 {
		r.error("failed to patch event entity details", "type", "EventsRepository", "func", "PatchEventEntityDetails", "entity_id", entityID, "event_id", eventID, "error", gorm.ErrRecordNotFound)
		return gorm.ErrRecordNotFound
	}

	r.info("event entity details are patched", "type", "EventsRepository", "func", "PatchEventEntityDetails", "entity_id", entityID, "event_id", eventID)
	return nil
}
