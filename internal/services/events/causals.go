package events

import (
	"github.com/dejitarudemon/mundarium/internal/libs/execution"
	journal "github.com/dejitarudemon/mundarium/internal/libs/journals/execution"
	repoevents "github.com/dejitarudemon/mundarium/internal/repositories/events"
	"gorm.io/gorm"
)

const eventCausalEntity = "EVENT_CAUSAL"

// CreateEventCausal inserts a directed causal edge inside a transaction.
// Cycles across distinct events are allowed; self-links are rejected.
// Endpoint game times are not ordered: a cause may occur in the future relative to its effect.
func (s *Service) CreateEventCausal(ctx execution.ExecutionContext[journal.Result], input CreateEventCausalInput) (GetEventCausalOutput, error) {
	edge, err := toNewEventCausal(input)
	if err != nil {
		err = mapRequestErr(err)
		s.error("failed to create event causal", "type", "Service", "func", "CreateEventCausal", "error", err)
		s.fail(ctx, err, "type", "Service", "func", "CreateEventCausal")
		return GetEventCausalOutput{}, err
	}

	result := GetEventCausalOutput{}

	if err := s.connection.WithTransaction(
		ctx,
		func(tx *gorm.DB) error {
			if err := s.generator(tx).CreateEventCausal(ctx, edge); err != nil {
				return err
			}

			var mapErr error
			result, mapErr = toGotEventCausal(*edge)
			if mapErr != nil {
				return mapErr
			}

			return ctx.Complete(
				journal.Result{
					Action: journal.Action_Create,
					Entity: eventCausalEntity,
					Data:   result,
				}, tx,
			)
		},
	); err != nil {
		err = mapEventCausalErr(err, input.From, input.To)
		s.error("failed to create event causal", "type", "Service", "func", "CreateEventCausal", "error", err)
		s.fail(ctx, err, "type", "Service", "func", "CreateEventCausal")
		return GetEventCausalOutput{}, err
	}

	s.info("event causal is created", "type", "Service", "func", "CreateEventCausal", "data", result)
	return result, nil
}

// GetEventCausal loads a causal edge and completes the execution context.
func (s *Service) GetEventCausal(ctx execution.ExecutionContext[journal.Result], from, to repoevents.ID) (GetEventCausalOutput, error) {
	if isNilEventID(from) || isNilEventID(to) {
		err := mapRequestErr(repoevents.ErrEventCausalIncomplete)
		s.error("failed to get event causal", "type", "Service", "func", "GetEventCausal", "error", err)
		s.fail(ctx, err, "type", "Service", "func", "GetEventCausal")
		return GetEventCausalOutput{}, err
	}

	edge, err := s.repository().GetEventCausal(ctx, from, to)
	if err != nil {
		err = mapEventCausalErr(err, from, to)
		s.error("failed to get event causal", "type", "Service", "func", "GetEventCausal", "error", err)
		s.fail(ctx, err, "type", "Service", "func", "GetEventCausal")
		return GetEventCausalOutput{}, err
	}

	result, err := toGotEventCausal(*edge)
	if err != nil {
		err = mapRequestErr(err)
		s.error("failed to get event causal", "type", "Service", "func", "GetEventCausal", "error", err)
		s.fail(ctx, err, "type", "Service", "func", "GetEventCausal")
		return GetEventCausalOutput{}, err
	}

	if err := ctx.Complete(
		journal.Result{
			Action: journal.Action_Get,
			Entity: eventCausalEntity,
			Data:   result,
		}, s.connection.DB(),
	); err != nil {
		err = mapEventCausalErr(err, from, to)
		s.error("failed to get event causal", "type", "Service", "func", "GetEventCausal", "error", err)
		s.fail(ctx, err, "type", "Service", "func", "GetEventCausal")
		return GetEventCausalOutput{}, err
	}

	s.debug("got event causal", "type", "Service", "func", "GetEventCausal", "data", result)
	return result, nil
}

// FindEventCausals searches causal edges with optional filters.
func (s *Service) FindEventCausals(ctx execution.ExecutionContext[journal.Result], in FindEventCausalsInput) (FindEventCausalsOutput, error) {
	filter, err := toFindEventCausalsFilter(in)
	if err != nil {
		err = mapRequestErr(err)
		s.error("failed to find event causals", "type", "Service", "func", "FindEventCausals", "error", err)
		s.fail(ctx, err, "type", "Service", "func", "FindEventCausals")
		return FindEventCausalsOutput{}, err
	}

	edges, err := s.repository().FindEventCausals(ctx, filter)
	if err != nil {
		err = mapEventCausalErr(err, repoevents.ID{}, repoevents.ID{})
		s.error("failed to find event causals", "type", "Service", "func", "FindEventCausals", "error", err)
		s.fail(ctx, err, "type", "Service", "func", "FindEventCausals")
		return FindEventCausalsOutput{}, err
	}

	result, err := toFoundEventCausals(edges)
	if err != nil {
		err = mapRequestErr(err)
		s.error("failed to find event causals", "type", "Service", "func", "FindEventCausals", "error", err)
		s.fail(ctx, err, "type", "Service", "func", "FindEventCausals")
		return FindEventCausalsOutput{}, err
	}

	if err := ctx.Complete(
		journal.Result{
			Action: journal.Action_Get,
			Entity: eventCausalEntity,
			Data:   result,
		}, s.connection.DB(),
	); err != nil {
		err = mapEventCausalErr(err, repoevents.ID{}, repoevents.ID{})
		s.error("failed to find event causals", "type", "Service", "func", "FindEventCausals", "error", err)
		s.fail(ctx, err, "type", "Service", "func", "FindEventCausals")
		return FindEventCausalsOutput{}, err
	}

	s.debug("found event causals", "type", "Service", "func", "FindEventCausals", "count", len(result.Causals))
	return result, nil
}

// PatchEventCausalCause updates Cause of an existing edge.
func (s *Service) PatchEventCausalCause(ctx execution.ExecutionContext[journal.Result], from, to repoevents.ID, cause string) error {
	if isNilEventID(from) || isNilEventID(to) {
		err := mapRequestErr(repoevents.ErrEventCausalIncomplete)
		s.error("failed to patch event causal cause", "type", "Service", "func", "PatchEventCausalCause", "error", err)
		s.fail(ctx, err, "type", "Service", "func", "PatchEventCausalCause")
		return err
	}
	if from == to {
		err := mapRequestErr(repoevents.ErrEventCausalSelfLink)
		s.error("failed to patch event causal cause", "type", "Service", "func", "PatchEventCausalCause", "error", err)
		s.fail(ctx, err, "type", "Service", "func", "PatchEventCausalCause")
		return err
	}

	if err := s.connection.WithTransaction(
		ctx,
		func(tx *gorm.DB) error {
			if err := s.generator(tx).PatchEventCausalCause(ctx, from, to, cause); err != nil {
				return err
			}
			return ctx.Complete(
				journal.Result{
					Action: journal.Action_Patch,
					Entity: eventCausalEntity,
					Data: GetEventCausalOutput{
						From:  from,
						To:    to,
						Cause: cause,
					},
				}, tx,
			)
		},
	); err != nil {
		err = mapEventCausalErr(err, from, to)
		s.error("failed to patch event causal cause", "type", "Service", "func", "PatchEventCausalCause", "error", err)
		s.fail(ctx, err, "type", "Service", "func", "PatchEventCausalCause")
		return err
	}

	s.info("event causal cause is patched", "type", "Service", "func", "PatchEventCausalCause", "from", from, "to", to)
	return nil
}
