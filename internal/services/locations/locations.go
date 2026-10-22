package locations

import (
	"github.com/dejitarudemon/mundarium/internal/journal"
	"github.com/dejitarudemon/mundarium/internal/libs/execution"
	input "github.com/dejitarudemon/mundarium/internal/libs/inputs/locations"
	libjournal "github.com/dejitarudemon/mundarium/internal/libs/journal"
	output "github.com/dejitarudemon/mundarium/internal/libs/outputs/locations"
	"github.com/dejitarudemon/mundarium/internal/libs/types"
	"gorm.io/gorm"
)

const locationEntity = "LOCATION"

// CreateLocation creates a location inside a transaction and completes the execution context.
func (s *Service) CreateLocation(ctx execution.ExecutionContext, input input.CreateLocationInput) (output.CreateLocationOutput, error) {
	location, err := toNewLocation(input)
	if err != nil {
		_ = ctx.Fail(err)

		return output.CreateLocationOutput{}, err
	}

	result := output.CreateLocationOutput{}

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
					Action: libjournal.Action_Create,
					Entity: locationEntity,
					Data:   result,
				},
			)
		},
	); err != nil {
		_ = ctx.Fail(err)
		return output.CreateLocationOutput{}, err
	}

	return result, nil
}

// GetLocation loads a location by id, maps it to output, and completes the execution context.
func (s *Service) GetLocation(ctx execution.ExecutionContext, id types.ID) (output.GetLocationOutput, error) {
	location, err := s.repository().GetLocation(ctx, id)
	if err != nil {
		_ = ctx.Fail(err)
		return output.GetLocationOutput{}, err
	}

	result, err := toGotLocation(*location)
	if err != nil {
		_ = ctx.Fail(err)
		return output.GetLocationOutput{}, err
	}

	if err := ctx.Complete(
		journal.Result{
			Action: libjournal.Action_Get,
			Entity: locationEntity,
			Data:   result,
		},
	); err != nil {
		_ = ctx.Fail(err)
		return output.GetLocationOutput{}, err
	}

	return result, nil
}

// FindLocations searches locations under a scope with optional filters and completes the execution context.
func (s *Service) FindLocations(ctx execution.ExecutionContext, in input.FindLocationsInput) (output.FindLocationsOutput, error) {
	filter, limit, err := toFindLocationsFilter(in)
	if err != nil {
		_ = ctx.Fail(err)
		return output.FindLocationsOutput{}, err
	}

	locs, err := s.repository().FindLocations(ctx, filter)
	if err != nil {
		_ = ctx.Fail(err)
		return output.FindLocationsOutput{}, err
	}

	result, err := toFoundLocations(locs, limit)
	if err != nil {
		_ = ctx.Fail(err)
		return output.FindLocationsOutput{}, err
	}

	if err := ctx.Complete(
		journal.Result{
			Action: libjournal.Action_Get,
			Entity: locationEntity,
			Data:   result,
		},
	); err != nil {
		_ = ctx.Fail(err)
		return output.FindLocationsOutput{}, err
	}

	return result, nil
}

// FindChildLocations returns descendants of id grouped by depth and completes the execution context.
// Depth semantics match the repository: 0 or less yields an empty map; 1 is direct children.
func (s *Service) FindChildLocations(ctx execution.ExecutionContext, id types.ID, depth int) (output.FindChildLocationsOutput, error) {
	children, err := s.repository().FindChildLocations(ctx, id, depth, false)
	if err != nil {
		_ = ctx.Fail(err)
		return nil, err
	}

	result, err := toGotChildLocations(children)
	if err != nil {
		_ = ctx.Fail(err)
		return nil, err
	}

	if err := ctx.Complete(
		journal.Result{
			Action: libjournal.Action_Get,
			Entity: locationEntity,
			Data:   result,
		},
	); err != nil {
		_ = ctx.Fail(err)
		return nil, err
	}

	return result, nil
}

// MoveLocation reparents a location to parentID inside a transaction and completes the execution context.
// A nil parentID makes the location a root.
func (s *Service) MoveLocation(ctx execution.ExecutionContext, id types.ID, parentID *types.ID) error {
	if err := s.connection.WithTransaction(
		ctx,
		func(tx *gorm.DB) error {
			if err := s.generator(tx).PatchLocationParent(ctx, id, parentID); err != nil {
				return err
			}

			return ctx.Complete(
				journal.Result{
					Action: libjournal.Action_Patch,
					Entity: locationEntity,
					Data:   id,
				},
			)
		},
	); err != nil {
		_ = ctx.Fail(err)
		return err
	}

	return nil
}

// RenameLocation updates a location's name inside a transaction and completes the execution context.
func (s *Service) RenameLocation(ctx execution.ExecutionContext, id types.ID, name string) error {
	if err := s.connection.WithTransaction(
		ctx,
		func(tx *gorm.DB) error {
			if err := s.generator(tx).PatchLocationName(ctx, id, name); err != nil {
				return err
			}

			return ctx.Complete(
				journal.Result{
					Action: libjournal.Action_Patch,
					Entity: locationEntity,
					Data:   id,
				},
			)
		},
	); err != nil {
		_ = ctx.Fail(err)
		return err
	}

	return nil
}

// UpdateLocationDetails updates a location's details inside a transaction and completes the execution context.
func (s *Service) UpdateLocationDetails(ctx execution.ExecutionContext, id types.ID, details types.JSON) error {
	if err := s.connection.WithTransaction(
		ctx,
		func(tx *gorm.DB) error {
			if err := s.generator(tx).PatchLocationDetails(ctx, id, details); err != nil {
				return err
			}

			return ctx.Complete(
				journal.Result{
					Action: libjournal.Action_Patch,
					Entity: locationEntity,
					Data:   id,
				},
			)
		},
	); err != nil {
		_ = ctx.Fail(err)
		return err
	}

	return nil
}

// UpdateLocationDescription updates a location's description inside a transaction and completes the execution context.
func (s *Service) UpdateLocationDescription(ctx execution.ExecutionContext, id types.ID, description string) error {
	if err := s.connection.WithTransaction(
		ctx,
		func(tx *gorm.DB) error {
			if err := s.generator(tx).PatchLocationDescription(ctx, id, description); err != nil {
				return err
			}

			return ctx.Complete(
				journal.Result{
					Action: libjournal.Action_Patch,
					Entity: locationEntity,
					Data:   id,
				},
			)
		},
	); err != nil {
		_ = ctx.Fail(err)
		return err
	}

	return nil
}
