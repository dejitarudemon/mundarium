package events

import (
	"github.com/dejitarudemon/mundarium/internal/libs/execution"
	journal "github.com/dejitarudemon/mundarium/internal/libs/journals/execution"
	"github.com/dejitarudemon/mundarium/internal/repositories"
	"github.com/dejitarudemon/mundarium/internal/repositories/entities"
	repoevents "github.com/dejitarudemon/mundarium/internal/repositories/events"
	"gorm.io/gorm"
)

const eventEntityLinkEntity = "EVENT_ENTITY"

// CreateEventEntity adds a participant link to an existing event inside a transaction.
func (s *Service) CreateEventEntity(ctx execution.ExecutionContext[journal.Result], input CreateEventEntityInput) (GetEventEntityOutput, error) {
	link, err := toNewEventEntity(input)
	if err != nil {
		err = mapRequestErr(err)
		s.error("failed to create event entity", "type", "Service", "func", "CreateEventEntity", "error", err)
		s.fail(ctx, err, "type", "Service", "func", "CreateEventEntity")
		return GetEventEntityOutput{}, err
	}

	result := GetEventEntityOutput{}

	if err := s.connection.WithTransaction(
		ctx,
		func(tx *gorm.DB) error {
			if err := s.generator(tx).CreateEventEntity(ctx, link); err != nil {
				return err
			}

			var mapErr error
			result, mapErr = toGotEventEntity(*link)
			if mapErr != nil {
				return mapErr
			}

			return ctx.Complete(
				journal.Result{
					Action: journal.Action_Create,
					Entity: eventEntityLinkEntity,
					Data:   result,
				}, tx,
			)
		},
	); err != nil {
		err = mapEventEntityErr(err, input.EntityID, input.EventID.String())
		s.error("failed to create event entity", "type", "Service", "func", "CreateEventEntity", "error", err)
		s.fail(ctx, err, "type", "Service", "func", "CreateEventEntity")
		return GetEventEntityOutput{}, err
	}

	s.info("event entity is created", "type", "Service", "func", "CreateEventEntity", "data", result)
	return result, nil
}

// GetEventEntity loads a participant link and completes the execution context.
func (s *Service) GetEventEntity(ctx execution.ExecutionContext[journal.Result], entityID entities.ID, eventID repoevents.ID) (GetEventEntityOutput, error) {
	if isNilEventID(eventID) {
		err := mapRequestErr(errNilEventID)
		s.error("failed to get event entity", "type", "Service", "func", "GetEventEntity", "error", err)
		s.fail(ctx, err, "type", "Service", "func", "GetEventEntity")
		return GetEventEntityOutput{}, err
	}

	link, err := s.repository().GetEventEntity(ctx, entityID, eventID)
	if err != nil {
		err = mapEventEntityErr(err, entityID.String(), eventID.String())
		s.error("failed to get event entity", "type", "Service", "func", "GetEventEntity", "error", err)
		s.fail(ctx, err, "type", "Service", "func", "GetEventEntity")
		return GetEventEntityOutput{}, err
	}

	result, err := toGotEventEntity(*link)
	if err != nil {
		err = mapRequestErr(err)
		s.error("failed to get event entity", "type", "Service", "func", "GetEventEntity", "error", err)
		s.fail(ctx, err, "type", "Service", "func", "GetEventEntity")
		return GetEventEntityOutput{}, err
	}

	if err := ctx.Complete(
		journal.Result{
			Action: journal.Action_Get,
			Entity: eventEntityLinkEntity,
			Data:   result,
		}, s.connection.DB(),
	); err != nil {
		err = mapEventEntityErr(err, entityID.String(), eventID.String())
		s.error("failed to get event entity", "type", "Service", "func", "GetEventEntity", "error", err)
		s.fail(ctx, err, "type", "Service", "func", "GetEventEntity")
		return GetEventEntityOutput{}, err
	}

	s.debug("got event entity", "type", "Service", "func", "GetEventEntity", "data", result)
	return result, nil
}

// FindEventEntities searches participant links with optional filters.
func (s *Service) FindEventEntities(ctx execution.ExecutionContext[journal.Result], in FindEventEntitiesInput) (FindEventEntitiesOutput, error) {
	filter, err := toFindEventEntitiesFilter(in)
	if err != nil {
		err = mapRequestErr(err)
		s.error("failed to find event entities", "type", "Service", "func", "FindEventEntities", "error", err)
		s.fail(ctx, err, "type", "Service", "func", "FindEventEntities")
		return FindEventEntitiesOutput{}, err
	}

	links, err := s.repository().FindEventEntities(ctx, filter)
	if err != nil {
		err = mapEventEntityErr(err, "", "")
		s.error("failed to find event entities", "type", "Service", "func", "FindEventEntities", "error", err)
		s.fail(ctx, err, "type", "Service", "func", "FindEventEntities")
		return FindEventEntitiesOutput{}, err
	}

	result, err := toFoundEventEntities(links)
	if err != nil {
		err = mapRequestErr(err)
		s.error("failed to find event entities", "type", "Service", "func", "FindEventEntities", "error", err)
		s.fail(ctx, err, "type", "Service", "func", "FindEventEntities")
		return FindEventEntitiesOutput{}, err
	}

	if err := ctx.Complete(
		journal.Result{
			Action: journal.Action_Get,
			Entity: eventEntityLinkEntity,
			Data:   result,
		}, s.connection.DB(),
	); err != nil {
		err = mapEventEntityErr(err, "", "")
		s.error("failed to find event entities", "type", "Service", "func", "FindEventEntities", "error", err)
		s.fail(ctx, err, "type", "Service", "func", "FindEventEntities")
		return FindEventEntitiesOutput{}, err
	}

	s.debug("found event entities", "type", "Service", "func", "FindEventEntities", "count", len(result.Entities))
	return result, nil
}

// PatchEventEntityRole sets Role of a participant link.
func (s *Service) PatchEventEntityRole(ctx execution.ExecutionContext[journal.Result], entityID entities.ID, eventID repoevents.ID, role string) error {
	parsed, err := parseRole(role)
	if err != nil {
		err = mapRequestErr(err)
		s.error("failed to patch event entity role", "type", "Service", "func", "PatchEventEntityRole", "error", err)
		s.fail(ctx, err, "type", "Service", "func", "PatchEventEntityRole")
		return err
	}
	return s.patchEventEntity(ctx, "PatchEventEntityRole", entityID, eventID, func(tx *gorm.DB) error {
		return s.generator(tx).PatchEventEntityRole(ctx, entityID, eventID, parsed)
	})
}

// PatchEventEntityKnownAtGameTime establishes knowledge (KnownAtGameTime + Belief) once.
func (s *Service) PatchEventEntityKnownAtGameTime(
	ctx execution.ExecutionContext[journal.Result],
	entityID entities.ID,
	eventID repoevents.ID,
	knownAt repositories.Ticks,
	belief int8,
) error {
	return s.patchEventEntity(ctx, "PatchEventEntityKnownAtGameTime", entityID, eventID, func(tx *gorm.DB) error {
		return s.generator(tx).PatchEventEntityKnownAtGameTime(ctx, entityID, eventID, knownAt, belief)
	})
}

// PatchEventEntityBelief updates Belief after knowledge has been established.
func (s *Service) PatchEventEntityBelief(
	ctx execution.ExecutionContext[journal.Result],
	entityID entities.ID,
	eventID repoevents.ID,
	belief int8,
) error {
	return s.patchEventEntity(ctx, "PatchEventEntityBelief", entityID, eventID, func(tx *gorm.DB) error {
		return s.generator(tx).PatchEventEntityBelief(ctx, entityID, eventID, belief)
	})
}

// PatchEventEntityDetails sets Details of a participant link.
func (s *Service) PatchEventEntityDetails(
	ctx execution.ExecutionContext[journal.Result],
	entityID entities.ID,
	eventID repoevents.ID,
	details repositories.JSON,
) error {
	return s.patchEventEntity(ctx, "PatchEventEntityDetails", entityID, eventID, func(tx *gorm.DB) error {
		return s.generator(tx).PatchEventEntityDetails(ctx, entityID, eventID, details)
	})
}

func (s *Service) patchEventEntity(
	ctx execution.ExecutionContext[journal.Result],
	funcName string,
	entityID entities.ID,
	eventID repoevents.ID,
	patch func(*gorm.DB) error,
) error {
	if isNilEventID(eventID) {
		err := mapRequestErr(errNilEventID)
		s.error("failed to patch event entity", "type", "Service", "func", funcName, "error", err)
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
					Entity: eventEntityLinkEntity,
					Data: map[string]string{
						"entity_id": entityID.String(),
						"event_id":  eventID.String(),
					},
				}, tx,
			)
		},
	); err != nil {
		err = mapEventEntityErr(err, entityID.String(), eventID.String())
		s.error("failed to patch event entity", "type", "Service", "func", funcName, "error", err)
		s.fail(ctx, err, "type", "Service", "func", funcName)
		return err
	}

	s.info("event entity is patched", "type", "Service", "func", funcName, "entity_id", entityID, "event_id", eventID)
	return nil
}
