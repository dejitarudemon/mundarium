package locations

import (
	"github.com/dejitarudemon/mundarium/internal/libs/execution"
	journal "github.com/dejitarudemon/mundarium/internal/libs/journals/execution"
	"github.com/dejitarudemon/mundarium/internal/repositories"
	"gorm.io/gorm"
)

const locationEntity = "LOCATION"

// CreateLocation creates a location inside a transaction and completes the execution context.
func (s *Service) CreateLocation(ctx execution.ExecutionContext[journal.Result], input CreateLocationInput) (CreateLocationOutput, error) {
	location, err := toNewLocation(input)
	if err != nil {
		err = mapRequestErr(err)
		s.error("failed to create location", "type", "Service", "func", "CreateLocation", "error", err)
		s.fail(ctx, err, "type", "Service", "func", "CreateLocation")
		return CreateLocationOutput{}, err
	}

	result := CreateLocationOutput{}

	if err := s.connection.WithTransaction(
		ctx,
		func(tx *gorm.DB) error {
			location, err := s.generator(tx).CreateLocation(ctx, location)
			if err != nil {
				return err
			}

			result, err = toCreatedLocation(*location)
			if err != nil {
				return err
			}

			return ctx.Complete(
				journal.Result{
					Action: journal.Action_Create,
					Entity: locationEntity,
					Data:   result,
				}, tx,
			)
		},
	); err != nil {
		err = mapLocationErr(err, result.ID)
		s.error("failed to create location", "type", "Service", "func", "CreateLocation", "error", err)
		s.fail(ctx, err, "type", "Service", "func", "CreateLocation")
		return CreateLocationOutput{}, err
	}

	s.info("location is created", "type", "Service", "func", "CreateLocation", "data", result)
	return result, nil
}

// GetLocation loads a location by id, maps it to output, and completes the execution context.
func (s *Service) GetLocation(ctx execution.ExecutionContext[journal.Result], id repositories.ID) (GetLocationOutput, error) {
	location, err := s.repository().GetLocation(ctx, id)
	if err != nil {
		err = mapLocationErr(err, id)
		s.error("failed to get location", "type", "Service", "func", "GetLocation", "id", id, "error", err)
		s.fail(ctx, err, "type", "Service", "func", "GetLocation", "id", id)
		return GetLocationOutput{}, err
	}

	result, err := toGotLocation(*location)
	if err != nil {
		err = mapRequestErr(err)
		s.error("failed to get location", "type", "Service", "func", "GetLocation", "id", id, "error", err)
		s.fail(ctx, err, "type", "Service", "func", "GetLocation", "id", id)
		return GetLocationOutput{}, err
	}

	if err := ctx.Complete(
		journal.Result{
			Action: journal.Action_Get,
			Entity: locationEntity,
			Data:   result,
		}, s.connection.DB(),
	); err != nil {
		err = mapLocationErr(err, id)
		s.error("failed to get location", "type", "Service", "func", "GetLocation", "id", id, "error", err)
		s.fail(ctx, err, "type", "Service", "func", "GetLocation", "id", id)
		return GetLocationOutput{}, err
	}

	s.debug("got location", "type", "Service", "func", "GetLocation", "data", result)
	return result, nil
}

// FindLocations searches locations under a scope with optional filters and completes the execution context.
func (s *Service) FindLocations(ctx execution.ExecutionContext[journal.Result], in FindLocationsInput) (FindLocationsOutput, error) {
	repo := s.repository()
	filter, limit, err := toFindLocationsFilter(in, repo.MaxDepth())
	if err != nil {
		err = mapRequestErr(err)
		s.error("failed to find locations", "type", "Service", "func", "FindLocations", "scope_id", in.ScopeID, "error", err)
		s.fail(ctx, err, "type", "Service", "func", "FindLocations", "scope_id", in.ScopeID)
		return FindLocationsOutput{}, err
	}

	locs, err := repo.FindLocations(ctx, filter)
	if err != nil {
		err = mapLocationErr(err, in.ScopeID)
		s.error("failed to find locations", "type", "Service", "func", "FindLocations", "scope_id", in.ScopeID, "error", err)
		s.fail(ctx, err, "type", "Service", "func", "FindLocations", "scope_id", in.ScopeID)
		return FindLocationsOutput{}, err
	}

	result, err := toFoundLocations(locs, limit)
	if err != nil {
		err = mapRequestErr(err)
		s.error("failed to find locations", "type", "Service", "func", "FindLocations", "scope_id", in.ScopeID, "error", err)
		s.fail(ctx, err, "type", "Service", "func", "FindLocations", "scope_id", in.ScopeID)
		return FindLocationsOutput{}, err
	}

	if err := ctx.Complete(
		journal.Result{
			Action: journal.Action_Get,
			Entity: locationEntity,
			Data:   result,
		}, s.connection.DB(),
	); err != nil {
		err = mapLocationErr(err, in.ScopeID)
		s.error("failed to find locations", "type", "Service", "func", "FindLocations", "scope_id", in.ScopeID, "error", err)
		s.fail(ctx, err, "type", "Service", "func", "FindLocations", "scope_id", in.ScopeID)
		return FindLocationsOutput{}, err
	}

	s.debug("found locations", "type", "Service", "func", "FindLocations", "scope_id", in.ScopeID)
	return result, nil
}

// FindChildLocations returns descendants of id grouped by depth and completes the execution context.
// Depth semantics match the repository: 0 or less yields an empty map; 1 is direct children.
func (s *Service) FindChildLocations(ctx execution.ExecutionContext[journal.Result], id repositories.ID, depth int) (FindChildLocationsOutput, error) {
	children, err := s.repository().FindChildLocations(ctx, id, depth, false)
	if err != nil {
		err = mapLocationErr(err, id)
		s.error("failed to find child locations", "type", "Service", "func", "FindChildLocations", "id", id, "error", err)
		s.fail(ctx, err, "type", "Service", "func", "FindChildLocations", "id", id)
		return nil, err
	}

	result, err := toGotChildLocations(children)
	if err != nil {
		err = mapRequestErr(err)
		s.error("failed to find child locations", "type", "Service", "func", "FindChildLocations", "id", id, "error", err)
		s.fail(ctx, err, "type", "Service", "func", "FindChildLocations", "id", id)
		return nil, err
	}

	if err := ctx.Complete(
		journal.Result{
			Action: journal.Action_Get,
			Entity: locationEntity,
			Data:   result,
		}, s.connection.DB(),
	); err != nil {
		err = mapLocationErr(err, id)
		s.error("failed to find child locations", "type", "Service", "func", "FindChildLocations", "id", id, "error", err)
		s.fail(ctx, err, "type", "Service", "func", "FindChildLocations", "id", id)
		return nil, err
	}

	s.debug("found child locations", "type", "Service", "func", "FindChildLocations", "id", id)
	return result, nil
}

// MoveLocation reparents a location to parentID inside a transaction and completes the execution context.
// A nil parentID makes the location a root.
func (s *Service) MoveLocation(ctx execution.ExecutionContext[journal.Result], id repositories.ID, parentID *repositories.ID) error {
	if err := s.connection.WithTransaction(
		ctx,
		func(tx *gorm.DB) error {
			if err := s.generator(tx).PatchLocationParent(ctx, id, parentID); err != nil {
				return err
			}

			return ctx.Complete(
				journal.Result{
					Action: journal.Action_Patch,
					Entity: locationEntity,
					Data:   id,
				}, tx,
			)
		},
	); err != nil {
		err = mapLocationErr(err, id)
		s.error("failed to move location", "type", "Service", "func", "MoveLocation", "id", id, "error", err)
		s.fail(ctx, err, "type", "Service", "func", "MoveLocation", "id", id)
		return err
	}

	s.info("location is moved", "type", "Service", "func", "MoveLocation", "id", id, "parent_id", parentID)
	return nil
}

// RenameLocation updates a location's name inside a transaction and completes the execution context.
func (s *Service) RenameLocation(ctx execution.ExecutionContext[journal.Result], id repositories.ID, name string) error {
	if err := s.connection.WithTransaction(
		ctx,
		func(tx *gorm.DB) error {
			if err := s.generator(tx).PatchLocationName(ctx, id, name); err != nil {
				return err
			}

			return ctx.Complete(
				journal.Result{
					Action: journal.Action_Patch,
					Entity: locationEntity,
					Data:   id,
				}, tx,
			)
		},
	); err != nil {
		err = mapLocationErr(err, id)
		s.error("failed to rename location", "type", "Service", "func", "RenameLocation", "id", id, "error", err)
		s.fail(ctx, err, "type", "Service", "func", "RenameLocation", "id", id)
		return err
	}

	s.info("location is renamed", "type", "Service", "func", "RenameLocation", "id", id, "name", name)
	return nil
}

// UpdateLocationDetails updates a location's details inside a transaction and completes the execution context.
func (s *Service) UpdateLocationDetails(ctx execution.ExecutionContext[journal.Result], id repositories.ID, details repositories.JSON) error {
	if err := s.connection.WithTransaction(
		ctx,
		func(tx *gorm.DB) error {
			if err := s.generator(tx).PatchLocationDetails(ctx, id, details); err != nil {
				return err
			}

			return ctx.Complete(
				journal.Result{
					Action: journal.Action_Patch,
					Entity: locationEntity,
					Data:   id,
				}, tx,
			)
		},
	); err != nil {
		err = mapLocationErr(err, id)
		s.error("failed to update location details", "type", "Service", "func", "UpdateLocationDetails", "id", id, "error", err)
		s.fail(ctx, err, "type", "Service", "func", "UpdateLocationDetails", "id", id)
		return err
	}

	s.info("location details are updated", "type", "Service", "func", "UpdateLocationDetails", "id", id)
	return nil
}

// UpdateLocationDescription updates a location's description inside a transaction and completes the execution context.
func (s *Service) UpdateLocationDescription(ctx execution.ExecutionContext[journal.Result], id repositories.ID, description string) error {
	if err := s.connection.WithTransaction(
		ctx,
		func(tx *gorm.DB) error {
			if err := s.generator(tx).PatchLocationDescription(ctx, id, description); err != nil {
				return err
			}

			return ctx.Complete(
				journal.Result{
					Action: journal.Action_Patch,
					Entity: locationEntity,
					Data:   id,
				}, tx,
			)
		},
	); err != nil {
		err = mapLocationErr(err, id)
		s.error("failed to update location description", "type", "Service", "func", "UpdateLocationDescription", "id", id, "error", err)
		s.fail(ctx, err, "type", "Service", "func", "UpdateLocationDescription", "id", id)
		return err
	}

	s.info("location description is updated", "type", "Service", "func", "UpdateLocationDescription", "id", id)
	return nil
}
