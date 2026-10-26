package locations

import (
	"github.com/dejitarudemon/mundarium/internal/libs/execution"
	journal "github.com/dejitarudemon/mundarium/internal/libs/journals/execution"
	"github.com/dejitarudemon/mundarium/internal/repositories"
	"gorm.io/gorm"
)

const locationSubtypeEntity = "LOCATION_SUBTYPE"

// CreateSubtype creates a location subtype inside a transaction and completes the execution context.
func (s *Service) CreateSubtype(ctx execution.ExecutionContext[journal.Result], input CreateSubtypeInput) (CreateSubtypeOutput, error) {
	subtype, err := toNewSubtype(input)
	if err != nil {
		err = mapRequestErr(err)
		s.error("failed to create subtype", "type", "LocationService", "func", "CreateSubtype", "error", err)
		s.fail(ctx, err, "type", "LocationService", "func", "CreateSubtype")
		return CreateSubtypeOutput{}, err
	}

	result := CreateSubtypeOutput{}

	if err := s.connection.WithTransaction(
		ctx,
		func(tx *gorm.DB) error {
			subtype, err := s.generator(tx).CreateSubtype(ctx, subtype)
			if err != nil {
				return err
			}

			result, err = toCreatedSubtype(*subtype)
			if err != nil {
				return err
			}

			return ctx.Complete(
				journal.Result{
					Action: journal.Action_Create,
					Entity: locationSubtypeEntity,
					Data:   result,
				}, tx,
			)
		},
	); err != nil {
		err = mapSubtypeErr(err, result.ID)
		s.error("failed to create subtype", "type", "LocationService", "func", "CreateSubtype", "error", err)
		s.fail(ctx, err, "type", "LocationService", "func", "CreateSubtype")
		return CreateSubtypeOutput{}, err
	}

	s.info("subtype is created", "type", "LocationService", "func", "CreateSubtype", "data", result)
	return result, nil
}

// GetSubtype loads a subtype by id, maps it to output, and completes the execution context.
func (s *Service) GetSubtype(ctx execution.ExecutionContext[journal.Result], id repositories.ID) (GetSubtypeOutput, error) {
	subtype, err := s.repository().GetSubtype(ctx, id)
	if err != nil {
		err = mapSubtypeErr(err, id)
		s.error("failed to get subtype", "type", "LocationService", "func", "GetSubtype", "id", id, "error", err)
		s.fail(ctx, err, "type", "LocationService", "func", "GetSubtype", "id", id)
		return GetSubtypeOutput{}, err
	}

	result, err := toGotSubtype(*subtype)
	if err != nil {
		err = mapRequestErr(err)
		s.error("failed to get subtype", "type", "LocationService", "func", "GetSubtype", "id", id, "error", err)
		s.fail(ctx, err, "type", "LocationService", "func", "GetSubtype", "id", id)
		return GetSubtypeOutput{}, err
	}

	if err := ctx.Complete(
		journal.Result{
			Action: journal.Action_Get,
			Entity: locationSubtypeEntity,
			Data:   result,
		}, s.connection.DB(),
	); err != nil {
		err = mapSubtypeErr(err, id)
		s.error("failed to get subtype", "type", "LocationService", "func", "GetSubtype", "id", id, "error", err)
		s.fail(ctx, err, "type", "LocationService", "func", "GetSubtype", "id", id)
		return GetSubtypeOutput{}, err
	}

	s.debug("got subtype", "type", "LocationService", "func", "GetSubtype", "data", result)
	return result, nil
}

// FindSubtypes searches subtypes with optional filters and completes the execution context.
func (s *Service) FindSubtypes(ctx execution.ExecutionContext[journal.Result], in FindSubtypesInput) (FindSubtypesOutput, error) {
	filter, err := toFindSubtypesFilter(in)
	if err != nil {
		err = mapRequestErr(err)
		s.error("failed to find subtypes", "type", "LocationService", "func", "FindSubtypes", "error", err)
		s.fail(ctx, err, "type", "LocationService", "func", "FindSubtypes")
		return FindSubtypesOutput{}, err
	}

	subtypes, err := s.repository().FindSubtypes(ctx, filter)
	if err != nil {
		err = mapSubtypeErr(err, 0)
		s.error("failed to find subtypes", "type", "LocationService", "func", "FindSubtypes", "error", err)
		s.fail(ctx, err, "type", "LocationService", "func", "FindSubtypes")
		return FindSubtypesOutput{}, err
	}

	result, err := toFoundSubtypes(subtypes)
	if err != nil {
		err = mapRequestErr(err)
		s.error("failed to find subtypes", "type", "LocationService", "func", "FindSubtypes", "error", err)
		s.fail(ctx, err, "type", "LocationService", "func", "FindSubtypes")
		return FindSubtypesOutput{}, err
	}

	if err := ctx.Complete(
		journal.Result{
			Action: journal.Action_Get,
			Entity: locationSubtypeEntity,
			Data:   result,
		}, s.connection.DB(),
	); err != nil {
		err = mapSubtypeErr(err, 0)
		s.error("failed to find subtypes", "type", "LocationService", "func", "FindSubtypes", "error", err)
		s.fail(ctx, err, "type", "LocationService", "func", "FindSubtypes")
		return FindSubtypesOutput{}, err
	}

	s.debug("found subtypes", "type", "LocationService", "func", "FindSubtypes", "count", len(result.Subtypes))
	return result, nil
}
