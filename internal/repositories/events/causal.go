package events

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// EventCausal is a directed causal edge from one event to another.
// From is the cause event (the event the link originates from).
// To is the effect event (the event the link leads to).
// From and To form the composite primary key: a causal link either exists with
// both endpoints set, or it does not exist at all. From and To must be non-nil
// and distinct. Direct self-links are forbidden.
// The edge endpoints are immutable after insert; Cause may be updated via PatchEventCausalCause.
// Edges are removed when either endpoint event is deleted.
type EventCausal struct {
	// From is the cause event id (от какого события).
	From ID `gorm:"column:from_id;type:uuid;primaryKey;check:chk_event_causals_no_self,from_id <> to_id" json:"from"`
	// To is the effect event id (к какому событию).
	To ID `gorm:"column:to_id;type:uuid;primaryKey;index" json:"to"`

	Cause string `gorm:"type:text;not null" json:"cause"`
}

// String returns a compact debug representation of the causal link.
func (c EventCausal) String() string {
	return fmt.Sprintf("EventCausal{From:%s To:%s Cause:%q}", c.From, c.To, c.Cause)
}

func validateEventCausalEndpoints(from, to ID) error {
	if uuid.UUID(from) == uuid.Nil || uuid.UUID(to) == uuid.Nil {
		return ErrEventCausalIncomplete
	}
	if from == to {
		return ErrEventCausalSelfLink
	}
	return nil
}

// CreateEventCausal inserts a new directed causal edge.
// Both From and To are required and must differ; together they identify the edge.
// Endpoints are immutable afterward; Cause may be changed with PatchEventCausalCause.
func (r *EventsRepository) CreateEventCausal(ctx context.Context, edge *EventCausal) error {
	if err := validateEventCausalEndpoints(edge.From, edge.To); err != nil {
		r.error("failed to create event causal", "type", "EventsRepository", "func", "CreateEventCausal", "data", edge, "error", err)
		return err
	}

	if err := r.db.WithContext(ctx).Create(edge).Error; err != nil {
		r.error("failed to create event causal", "type", "EventsRepository", "func", "CreateEventCausal", "data", edge, "error", err)
		return err
	}
	r.info("event causal is created", "type", "EventsRepository", "func", "CreateEventCausal", "data", edge)
	return nil
}

// GetEventCausal returns the causal edge from -> to.
// Returns gorm.ErrRecordNotFound if no row exists.
func (r *EventsRepository) GetEventCausal(ctx context.Context, from, to ID) (*EventCausal, error) {
	var edge EventCausal

	if err := r.db.WithContext(ctx).
		First(&edge, "from_id = ? AND to_id = ?", from, to).Error; err != nil {
		r.error("failed to get event causal", "type", "EventsRepository", "func", "GetEventCausal", "from", from, "to", to, "error", err)
		return nil, err
	}

	r.debug("got event causal", "type", "EventsRepository", "func", "GetEventCausal", "data", &edge)
	return &edge, nil
}

// FindEventCausals returns causal edges matching filter ordered by from_id, to_id.
// With an empty filter, all edges are returned (subject to Limit when positive).
//
// EventID, when set, matches rows where the event is either endpoint and
// ignores From/To.
func (r *EventsRepository) FindEventCausals(ctx context.Context, filter FindEventCausalsFilter) ([]EventCausal, error) {
	edges := make([]EventCausal, 0)
	query := r.db.WithContext(ctx).Model(&EventCausal{})

	switch {
	case filter.EventID != nil:
		query = query.Where("from_id = ? OR to_id = ?", *filter.EventID, *filter.EventID)
	default:
		if filter.From != nil {
			query = query.Where("from_id = ?", *filter.From)
		}
		if filter.To != nil {
			query = query.Where("to_id = ?", *filter.To)
		}
	}
	if filter.Limit > 0 {
		query = query.Limit(filter.Limit)
	}

	if err := query.Order("from_id, to_id").Find(&edges).Error; err != nil {
		r.error("failed to find event causals", "type", "EventsRepository", "func", "FindEventCausals", "filter", filter, "error", err)
		return nil, err
	}

	r.debug("found event causals", "type", "EventsRepository", "func", "FindEventCausals", "count", len(edges))
	return edges, nil
}

// PatchEventCausalCause sets Cause for the edge from -> to.
// Returns gorm.ErrRecordNotFound if no row was updated.
func (r *EventsRepository) PatchEventCausalCause(ctx context.Context, from, to ID, cause string) error {
	result := r.db.WithContext(ctx).
		Model(&EventCausal{}).
		Where("from_id = ? AND to_id = ?", from, to).
		Update("cause", cause)
	if result.Error != nil {
		r.error("failed to patch event causal cause", "type", "EventsRepository", "func", "PatchEventCausalCause", "from", from, "to", to, "error", result.Error)
		return result.Error
	}
	if result.RowsAffected == 0 {
		r.error("failed to patch event causal cause", "type", "EventsRepository", "func", "PatchEventCausalCause", "from", from, "to", to, "error", gorm.ErrRecordNotFound)
		return gorm.ErrRecordNotFound
	}

	r.info("event causal cause is patched", "type", "EventsRepository", "func", "PatchEventCausalCause", "from", from, "to", to)
	return nil
}
