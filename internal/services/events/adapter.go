package events

import (
	"errors"
	"strings"

	"github.com/dejitarudemon/mundarium/internal/repositories/entities"
	repoevents "github.com/dejitarudemon/mundarium/internal/repositories/events"
	"github.com/google/uuid"
	"github.com/lib/pq"
)

func isNilEventID(id repoevents.ID) bool {
	return uuid.UUID(id) == uuid.Nil
}

func parseEntityID(s string) (entities.ID, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return entities.ID{}, errors.New("entity id is empty")
	}
	u, err := uuid.Parse(s)
	if err != nil {
		return entities.ID{}, errors.New("entity id is invalid")
	}
	return entities.ID(u), nil
}

func parseRole(s string) (repoevents.EventEntityRole, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return repoevents.EventEntityRoleUnknown, errors.New("role is empty")
	}
	return repoevents.NewEventEntityRole(s)
}

// toImportance converts a tool-facing importance int into the domain int8 range 0..100.
// Values outside that range yield ErrEventImportanceOutOfRange (mapped to InvalidRequest).
func toImportance(v int) (int8, error) {
	if v < 0 || v > 100 {
		return 0, repoevents.ErrEventImportanceOutOfRange
	}
	return int8(v), nil
}

// toNewEvent maps CreateEventInput to a domain Event ready for insert.
func toNewEvent(input CreateEventInput) (*repoevents.Event, error) {
	if strings.TrimSpace(input.Summary) == "" {
		return nil, errors.New("summary is empty")
	}
	importance, err := toImportance(input.Importance)
	if err != nil {
		return nil, err
	}
	if input.LocationID != nil && *input.LocationID == 0 {
		return nil, repoevents.ErrEventLocationRequired
	}

	entitiesLinks := make([]repoevents.EventEntity, 0, len(input.Entities))
	for _, in := range input.Entities {
		entityID, err := parseEntityID(in.EntityID)
		if err != nil {
			return nil, err
		}
		role, err := parseRole(in.Role)
		if err != nil {
			return nil, err
		}
		entitiesLinks = append(entitiesLinks, repoevents.EventEntity{
			EntityID: entityID,
			Role:     role,
			Details:  in.Details,
		})
	}

	// Nested causals are attached after insert (incoming: From -> new event as To).
	if _, err := toIncomingCreateCausals(input.Causals); err != nil {
		return nil, err
	}

	return &repoevents.Event{
		LocationID:        input.LocationID,
		Tags:              pq.StringArray(input.Tags),
		Importance:        importance,
		CreatedAtGameTime: input.CreatedAtGameTime,
		Summary:           input.Summary,
		Details:           input.Details,
		Entities:          entitiesLinks,
	}, nil
}

// toIncomingCreateCausals validates nested create_event causals (From + Cause).
// To is filled with the new event id by CreateEvent after insert.
func toIncomingCreateCausals(input []CreateEventCausalInput) ([]repoevents.EventCausal, error) {
	edges := make([]repoevents.EventCausal, 0, len(input))
	for _, in := range input {
		if isNilEventID(in.From) {
			return nil, repoevents.ErrEventCausalIncomplete
		}
		if strings.TrimSpace(in.Cause) == "" {
			return nil, errors.New("cause is empty")
		}
		edges = append(edges, repoevents.EventCausal{
			From:  in.From,
			Cause: in.Cause,
		})
	}
	return edges, nil
}

func toCreatedEvent(event repoevents.Event) (CreateEventOutput, error) {
	if isNilEventID(event.ID) {
		return CreateEventOutput{}, errors.New("event id is nil")
	}
	return CreateEventOutput{
		ID:         event.ID,
		Summary:    event.Summary,
		LocationID: event.LocationID,
	}, nil
}

func toGotEvent(event repoevents.Event) (GetEventOutput, error) {
	if isNilEventID(event.ID) {
		return GetEventOutput{}, errors.New("event id is nil")
	}

	entitiesOut := make([]GetEventEntityOutput, 0, len(event.Entities))
	for _, link := range event.Entities {
		item, err := toGotEventEntity(link)
		if err != nil {
			return GetEventOutput{}, err
		}
		entitiesOut = append(entitiesOut, item)
	}

	causalsOut := make([]GetEventCausalOutput, 0, len(event.Causals))
	for _, edge := range event.Causals {
		item, err := toGotEventCausal(edge)
		if err != nil {
			return GetEventOutput{}, err
		}
		causalsOut = append(causalsOut, item)
	}

	tags := []string(event.Tags)
	if tags == nil {
		tags = []string{}
	}

	return GetEventOutput{
		ID:                event.ID,
		LocationID:        event.LocationID,
		Tags:              tags,
		Importance:        event.Importance,
		CreatedAtGameTime: event.CreatedAtGameTime,
		Summary:           event.Summary,
		Details:           event.Details,
		Entities:          entitiesOut,
		Causals:           causalsOut,
	}, nil
}

func toFoundEvents(events []repoevents.Event) (FindEventsOutput, error) {
	out := make([]GetEventOutput, 0, len(events))
	for _, event := range events {
		item, err := toGotEvent(event)
		if err != nil {
			return FindEventsOutput{}, err
		}
		out = append(out, item)
	}
	return FindEventsOutput{Events: out}, nil
}

const (
	defaultFindEventsLimit = 20
	maxFindEventsLimit     = 100
)

func toFindEventsFilter(in FindEventsInput) (repoevents.FindEventsFilter, error) {
	if in.ID != nil && isNilEventID(*in.ID) {
		return repoevents.FindEventsFilter{}, errors.New("event id is nil")
	}
	if in.LocationID != nil && *in.LocationID == 0 {
		return repoevents.FindEventsFilter{}, repoevents.ErrEventLocationRequired
	}
	var minImportance *int8
	if in.MinImportance != nil {
		v, err := toImportance(*in.MinImportance)
		if err != nil {
			return repoevents.FindEventsFilter{}, err
		}
		minImportance = &v
	}
	var maxImportance *int8
	if in.MaxImportance != nil {
		v, err := toImportance(*in.MaxImportance)
		if err != nil {
			return repoevents.FindEventsFilter{}, err
		}
		maxImportance = &v
	}

	limit, err := resolveFindLimit(in.Limit)
	if err != nil {
		return repoevents.FindEventsFilter{}, err
	}

	var query *string
	if in.Query != nil {
		q := strings.TrimSpace(*in.Query)
		if q != "" {
			query = &q
		}
	}

	return repoevents.FindEventsFilter{
		ID:            in.ID,
		LocationID:    in.LocationID,
		Tags:          in.Tags,
		MinImportance: minImportance,
		MaxImportance: maxImportance,
		CreatedAtFrom: in.CreatedAtFrom,
		CreatedAtTo:   in.CreatedAtTo,
		Query:         query,
		Limit:         limit,
	}, nil
}

func toNewEventEntity(input CreateEventEntityInput) (*repoevents.EventEntity, error) {
	entityID, err := parseEntityID(input.EntityID)
	if err != nil {
		return nil, err
	}
	if isNilEventID(input.EventID) {
		return nil, errors.New("event id is nil")
	}
	role, err := parseRole(input.Role)
	if err != nil {
		return nil, err
	}
	return &repoevents.EventEntity{
		EntityID: entityID,
		EventID:  input.EventID,
		Role:     role,
		Details:  input.Details,
	}, nil
}

func toGotEventEntity(link repoevents.EventEntity) (GetEventEntityOutput, error) {
	if uuid.UUID(link.EntityID) == uuid.Nil {
		return GetEventEntityOutput{}, errors.New("entity id is nil")
	}
	if isNilEventID(link.EventID) {
		return GetEventEntityOutput{}, errors.New("event id is nil")
	}
	return GetEventEntityOutput{
		EntityID:        link.EntityID.String(),
		EventID:         link.EventID,
		Role:            link.Role.String(),
		Belief:          link.Belief,
		KnownAtGameTime: link.KnownAtGameTime,
		Details:         link.Details,
	}, nil
}

func toFoundEventEntities(links []repoevents.EventEntity) (FindEventEntitiesOutput, error) {
	out := make([]GetEventEntityOutput, 0, len(links))
	for _, link := range links {
		item, err := toGotEventEntity(link)
		if err != nil {
			return FindEventEntitiesOutput{}, err
		}
		out = append(out, item)
	}
	return FindEventEntitiesOutput{Entities: out}, nil
}

func resolveFindLimit(limit *int) (int, error) {
	n := defaultFindEventsLimit
	if limit != nil {
		n = *limit
	}
	if n < 1 || n > maxFindEventsLimit {
		return 0, errors.New("limit out of range")
	}
	return n, nil
}

func toFindEventEntitiesFilter(in FindEventEntitiesInput) (repoevents.FindEventEntitiesFilter, error) {
	filter := repoevents.FindEventEntitiesFilter{
		EventID:   in.EventID,
		KnownOnly: in.KnownOnly,
	}
	if in.EventID != nil && isNilEventID(*in.EventID) {
		return repoevents.FindEventEntitiesFilter{}, errors.New("event id is nil")
	}
	if in.EntityID != nil {
		id, err := parseEntityID(*in.EntityID)
		if err != nil {
			return repoevents.FindEventEntitiesFilter{}, err
		}
		filter.EntityID = &id
	}
	if in.Role != nil {
		role, err := parseRole(*in.Role)
		if err != nil {
			return repoevents.FindEventEntitiesFilter{}, err
		}
		filter.Role = &role
	}
	n, err := resolveFindLimit(in.Limit)
	if err != nil {
		return repoevents.FindEventEntitiesFilter{}, err
	}
	filter.Limit = n
	return filter, nil
}

func toNewEventCausal(input CreateEventCausalInput) (*repoevents.EventCausal, error) {
	if isNilEventID(input.From) || isNilEventID(input.To) {
		return nil, repoevents.ErrEventCausalIncomplete
	}
	if input.From == input.To {
		return nil, repoevents.ErrEventCausalSelfLink
	}
	if strings.TrimSpace(input.Cause) == "" {
		return nil, errors.New("cause is empty")
	}
	return &repoevents.EventCausal{
		From:  input.From,
		To:    input.To,
		Cause: input.Cause,
	}, nil
}

func toGotEventCausal(edge repoevents.EventCausal) (GetEventCausalOutput, error) {
	if isNilEventID(edge.From) || isNilEventID(edge.To) {
		return GetEventCausalOutput{}, repoevents.ErrEventCausalIncomplete
	}
	return GetEventCausalOutput{
		From:  edge.From,
		To:    edge.To,
		Cause: edge.Cause,
	}, nil
}

func toFoundEventCausals(edges []repoevents.EventCausal) (FindEventCausalsOutput, error) {
	out := make([]GetEventCausalOutput, 0, len(edges))
	for _, edge := range edges {
		item, err := toGotEventCausal(edge)
		if err != nil {
			return FindEventCausalsOutput{}, err
		}
		out = append(out, item)
	}
	return FindEventCausalsOutput{Causals: out}, nil
}

func toFindEventCausalsFilter(in FindEventCausalsInput) (repoevents.FindEventCausalsFilter, error) {
	if in.EventID != nil && isNilEventID(*in.EventID) {
		return repoevents.FindEventCausalsFilter{}, errors.New("event id is nil")
	}
	if in.From != nil && isNilEventID(*in.From) {
		return repoevents.FindEventCausalsFilter{}, errors.New("from id is nil")
	}
	if in.To != nil && isNilEventID(*in.To) {
		return repoevents.FindEventCausalsFilter{}, errors.New("to id is nil")
	}
	limit, err := resolveFindLimit(in.Limit)
	if err != nil {
		return repoevents.FindEventCausalsFilter{}, err
	}
	return repoevents.FindEventCausalsFilter{
		EventID: in.EventID,
		From:    in.From,
		To:      in.To,
		Limit:   limit,
	}, nil
}
