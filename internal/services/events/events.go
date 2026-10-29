package events

import (
	"errors"

	"github.com/dejitarudemon/mundarium/internal/libs/execution"
	journal "github.com/dejitarudemon/mundarium/internal/libs/journals/execution"
	"github.com/dejitarudemon/mundarium/internal/repositories"
	repoevents "github.com/dejitarudemon/mundarium/internal/repositories/events"
	"gorm.io/gorm"
)

const eventEntity = "EVENT"

var (
	errNilEventID = errors.New("event id is nil")
)

// CreateEvent creates an event (optionally with entities and incoming causals)
// inside a transaction and completes the execution context.
// Nested causals use From as the cause event; this event becomes To (effect).
// Nested causals may form cycles with other events; self-links are rejected.
// Cause endpoints may occur later in game time than this event.
func (s *Service) CreateEvent(ctx execution.ExecutionContext[journal.Result], input CreateEventInput) (CreateEventOutput, error) {
	event, err := toNewEvent(input)
	if err != nil {
		err = mapRequestErr(err)
		s.error("failed to create event", "type", "Service", "func", "CreateEvent", "error", err)
		s.fail(ctx, err, "type", "Service", "func", "CreateEvent")
		return CreateEventOutput{}, err
	}

	incoming, err := toIncomingCreateCausals(input.Causals)
	if err != nil {
		err = mapRequestErr(err)
		s.error("failed to create event", "type", "Service", "func", "CreateEvent", "error", err)
		s.fail(ctx, err, "type", "Service", "func", "CreateEvent")
		return CreateEventOutput{}, err
	}

	result := CreateEventOutput{}

	if err := s.connection.WithTransaction(
		ctx,
		func(tx *gorm.DB) error {
			repo := s.generator(tx)
			created, err := repo.CreateEvent(ctx, event)
			if err != nil {
				return err
			}

			for i := range incoming {
				incoming[i].To = created.ID
				if err := repo.CreateEventCausal(ctx, &incoming[i]); err != nil {
					return err
				}
			}

			var mapErr error
			result, mapErr = toCreatedEvent(*created)
			if mapErr != nil {
				return mapErr
			}

			return ctx.Complete(
				journal.Result{
					Action: journal.Action_Create,
					Entity: eventEntity,
					Data:   result,
				}, tx,
			)
		},
	); err != nil {
		err = mapEventErr(err, result.ID)
		s.error("failed to create event", "type", "Service", "func", "CreateEvent", "error", err)
		s.fail(ctx, err, "type", "Service", "func", "CreateEvent")
		return CreateEventOutput{}, err
	}

	s.info("event is created", "type", "Service", "func", "CreateEvent", "data", result)
	return result, nil
}

// GetEvent loads an event by id (with entities and causals) and completes the execution context.
func (s *Service) GetEvent(ctx execution.ExecutionContext[journal.Result], id repoevents.ID) (GetEventOutput, error) {
	if isNilEventID(id) {
		err := mapRequestErr(errNilEventID)
		s.error("failed to get event", "type", "Service", "func", "GetEvent", "error", err)
		s.fail(ctx, err, "type", "Service", "func", "GetEvent")
		return GetEventOutput{}, err
	}

	event, err := s.repository().GetEvent(ctx, id)
	if err != nil {
		err = mapEventErr(err, id)
		s.error("failed to get event", "type", "Service", "func", "GetEvent", "id", id, "error", err)
		s.fail(ctx, err, "type", "Service", "func", "GetEvent", "id", id)
		return GetEventOutput{}, err
	}

	result, err := toGotEvent(*event)
	if err != nil {
		err = mapRequestErr(err)
		s.error("failed to get event", "type", "Service", "func", "GetEvent", "id", id, "error", err)
		s.fail(ctx, err, "type", "Service", "func", "GetEvent", "id", id)
		return GetEventOutput{}, err
	}

	if err := ctx.Complete(
		journal.Result{
			Action: journal.Action_Get,
			Entity: eventEntity,
			Data:   result,
		}, s.connection.DB(),
	); err != nil {
		err = mapEventErr(err, id)
		s.error("failed to get event", "type", "Service", "func", "GetEvent", "id", id, "error", err)
		s.fail(ctx, err, "type", "Service", "func", "GetEvent", "id", id)
		return GetEventOutput{}, err
	}

	s.debug("got event", "type", "Service", "func", "GetEvent", "data", result)
	return result, nil
}

// FindEvents searches events with optional filters and completes the execution context.
func (s *Service) FindEvents(ctx execution.ExecutionContext[journal.Result], in FindEventsInput) (FindEventsOutput, error) {
	filter, err := toFindEventsFilter(in)
	if err != nil {
		err = mapRequestErr(err)
		s.error("failed to find events", "type", "Service", "func", "FindEvents", "error", err)
		s.fail(ctx, err, "type", "Service", "func", "FindEvents")
		return FindEventsOutput{}, err
	}

	events, err := s.repository().FindEvents(ctx, filter)
	if err != nil {
		err = mapEventErr(err, repoevents.ID{})
		s.error("failed to find events", "type", "Service", "func", "FindEvents", "error", err)
		s.fail(ctx, err, "type", "Service", "func", "FindEvents")
		return FindEventsOutput{}, err
	}

	result, err := toFoundEvents(events)
	if err != nil {
		err = mapRequestErr(err)
		s.error("failed to find events", "type", "Service", "func", "FindEvents", "error", err)
		s.fail(ctx, err, "type", "Service", "func", "FindEvents")
		return FindEventsOutput{}, err
	}

	if err := ctx.Complete(
		journal.Result{
			Action: journal.Action_Get,
			Entity: eventEntity,
			Data:   result,
		}, s.connection.DB(),
	); err != nil {
		err = mapEventErr(err, repoevents.ID{})
		s.error("failed to find events", "type", "Service", "func", "FindEvents", "error", err)
		s.fail(ctx, err, "type", "Service", "func", "FindEvents")
		return FindEventsOutput{}, err
	}

	s.debug("found events", "type", "Service", "func", "FindEvents", "count", len(result.Events))
	return result, nil
}

// PatchEventLocation sets LocationID of an event. locationID must be non-zero.
func (s *Service) PatchEventLocation(ctx execution.ExecutionContext[journal.Result], id repoevents.ID, locationID repositories.ID) error {
	return s.patchEvent(ctx, "PatchEventLocation", id, func(tx *gorm.DB) error {
		return s.generator(tx).PatchEventLocation(ctx, id, locationID)
	})
}

// PatchEventImportance sets Importance of an event (0..100 inclusive).
// Tool-facing importance is int; values outside 0..100 are InvalidRequest.
func (s *Service) PatchEventImportance(ctx execution.ExecutionContext[journal.Result], id repoevents.ID, importance int) error {
	value, err := toImportance(importance)
	if err != nil {
		err = mapRequestErr(err)
		s.error("failed to patch event importance", "type", "Service", "func", "PatchEventImportance", "id", id, "error", err)
		s.fail(ctx, err, "type", "Service", "func", "PatchEventImportance", "id", id)
		return err
	}
	return s.patchEvent(ctx, "PatchEventImportance", id, func(tx *gorm.DB) error {
		return s.generator(tx).PatchEventImportance(ctx, id, value)
	})
}

// PatchEventDetails sets Details of an event.
func (s *Service) PatchEventDetails(ctx execution.ExecutionContext[journal.Result], id repoevents.ID, details repositories.JSON) error {
	return s.patchEvent(ctx, "PatchEventDetails", id, func(tx *gorm.DB) error {
		return s.generator(tx).PatchEventDetails(ctx, id, details)
	})
}

// PatchEventSummary sets Summary of an event.
func (s *Service) PatchEventSummary(ctx execution.ExecutionContext[journal.Result], id repoevents.ID, summary string) error {
	return s.patchEvent(ctx, "PatchEventSummary", id, func(tx *gorm.DB) error {
		return s.generator(tx).PatchEventSummary(ctx, id, summary)
	})
}

func (s *Service) patchEvent(
	ctx execution.ExecutionContext[journal.Result],
	funcName string,
	id repoevents.ID,
	patch func(*gorm.DB) error,
) error {
	if isNilEventID(id) {
		err := mapRequestErr(errNilEventID)
		s.error("failed to patch event", "type", "Service", "func", funcName, "error", err)
		s.fail(ctx, err, "type", "Service", "func", funcName)
		return err
	}

	if err := s.connection.WithTransaction(
		ctx,
		func(tx *gorm.DB) error {
			if err := patch(tx); err != nil {
				return err
			}
			return ctx.Complete(
				journal.Result{
					Action: journal.Action_Patch,
					Entity: eventEntity,
					Data:   id,
				}, tx,
			)
		},
	); err != nil {
		err = mapEventErr(err, id)
		s.error("failed to patch event", "type", "Service", "func", funcName, "id", id, "error", err)
		s.fail(ctx, err, "type", "Service", "func", funcName, "id", id)
		return err
	}

	s.info("event is patched", "type", "Service", "func", funcName, "id", id)
	return nil
}
