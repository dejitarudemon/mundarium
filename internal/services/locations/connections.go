package locations

import (
	"errors"

	"github.com/dejitarudemon/mundarium/internal/libs/execution"
	journal "github.com/dejitarudemon/mundarium/internal/libs/journals/execution"
	"github.com/dejitarudemon/mundarium/internal/repositories"
	"gorm.io/gorm"
)

const locationConnectionEntity = "LOCATION_CONNECTION"

var (
	errZeroConnectionID = errors.New("connection id is zero")
	errZeroToID         = errors.New("to id is zero")
)

// CreateConnection creates a connection inside a transaction and completes the execution context.
// Self-links (FromID == ToID) are allowed. Endpoints must exist.
func (s *Service) CreateConnection(ctx execution.ExecutionContext[journal.Result], input CreateConnectionInput) (CreateConnectionOutput, error) {
	connection, err := toNewConnection(input)
	if err != nil {
		err = mapRequestErr(err)
		s.error("failed to create connection", "type", "Service", "func", "CreateConnection", "error", err)
		s.fail(ctx, err, "type", "Service", "func", "CreateConnection")
		return CreateConnectionOutput{}, err
	}

	result := CreateConnectionOutput{}

	if err := s.connection.WithTransaction(
		ctx,
		func(tx *gorm.DB) error {
			if err := s.generator(tx).CreateConnection(ctx, connection); err != nil {
				return err
			}

			var mapErr error
			result, mapErr = toCreatedConnection(*connection)
			if mapErr != nil {
				return mapErr
			}

			return ctx.Complete(
				journal.Result{
					Action: journal.Action_Create,
					Entity: locationConnectionEntity,
					Data:   result,
				}, tx,
			)
		},
	); err != nil {
		err = mapConnectionErr(err, result.ID)
		s.error("failed to create connection", "type", "Service", "func", "CreateConnection", "error", err)
		s.fail(ctx, err, "type", "Service", "func", "CreateConnection")
		return CreateConnectionOutput{}, err
	}

	s.info("connection is created", "type", "Service", "func", "CreateConnection", "data", result)
	return result, nil
}

// GetConnection loads a connection by id, maps it to output, and completes the execution context.
func (s *Service) GetConnection(ctx execution.ExecutionContext[journal.Result], id repositories.ID) (GetConnectionOutput, error) {
	connection, err := s.repository().GetConnection(ctx, id)
	if err != nil {
		err = mapConnectionErr(err, id)
		s.error("failed to get connection", "type", "Service", "func", "GetConnection", "id", id, "error", err)
		s.fail(ctx, err, "type", "Service", "func", "GetConnection", "id", id)
		return GetConnectionOutput{}, err
	}

	result, err := toGotConnection(*connection)
	if err != nil {
		err = mapRequestErr(err)
		s.error("failed to get connection", "type", "Service", "func", "GetConnection", "id", id, "error", err)
		s.fail(ctx, err, "type", "Service", "func", "GetConnection", "id", id)
		return GetConnectionOutput{}, err
	}

	if err := ctx.Complete(
		journal.Result{
			Action: journal.Action_Get,
			Entity: locationConnectionEntity,
			Data:   result,
		}, s.connection.DB(),
	); err != nil {
		err = mapConnectionErr(err, id)
		s.error("failed to get connection", "type", "Service", "func", "GetConnection", "id", id, "error", err)
		s.fail(ctx, err, "type", "Service", "func", "GetConnection", "id", id)
		return GetConnectionOutput{}, err
	}

	s.debug("got connection", "type", "Service", "func", "GetConnection", "data", result)
	return result, nil
}

// FindConnections searches connections with optional filters and completes the execution context.
func (s *Service) FindConnections(ctx execution.ExecutionContext[journal.Result], in FindConnectionsInput) (FindConnectionsOutput, error) {
	filter, err := toFindConnectionsFilter(in)
	if err != nil {
		err = mapRequestErr(err)
		s.error("failed to find connections", "type", "Service", "func", "FindConnections", "error", err)
		s.fail(ctx, err, "type", "Service", "func", "FindConnections")
		return FindConnectionsOutput{}, err
	}

	connections, err := s.repository().FindConnections(ctx, filter)
	if err != nil {
		err = mapConnectionErr(err, 0)
		s.error("failed to find connections", "type", "Service", "func", "FindConnections", "error", err)
		s.fail(ctx, err, "type", "Service", "func", "FindConnections")
		return FindConnectionsOutput{}, err
	}

	result, err := toFoundConnections(connections)
	if err != nil {
		err = mapRequestErr(err)
		s.error("failed to find connections", "type", "Service", "func", "FindConnections", "error", err)
		s.fail(ctx, err, "type", "Service", "func", "FindConnections")
		return FindConnectionsOutput{}, err
	}

	if err := ctx.Complete(
		journal.Result{
			Action: journal.Action_Get,
			Entity: locationConnectionEntity,
			Data:   result,
		}, s.connection.DB(),
	); err != nil {
		err = mapConnectionErr(err, 0)
		s.error("failed to find connections", "type", "Service", "func", "FindConnections", "error", err)
		s.fail(ctx, err, "type", "Service", "func", "FindConnections")
		return FindConnectionsOutput{}, err
	}

	s.debug("found connections", "type", "Service", "func", "FindConnections", "count", len(result.Connections))
	return result, nil
}

// UpdateConnection replaces the specified fields of a connection inside a transaction.
// Only non-nil pointer fields in input are written. ClearExpiresAt clears ExpiresAt.
func (s *Service) UpdateConnection(ctx execution.ExecutionContext[journal.Result], input UpdateConnectionInput) error {
	if input.ID == 0 {
		err := mapRequestErr(errZeroConnectionID)
		s.error("failed to update connection", "type", "Service", "func", "UpdateConnection", "error", err)
		s.fail(ctx, err, "type", "Service", "func", "UpdateConnection")
		return err
	}

	if err := s.connection.WithTransaction(
		ctx,
		func(tx *gorm.DB) error {
			repo := s.generator(tx)
			connection, err := repo.GetConnection(ctx, input.ID)
			if err != nil {
				return err
			}

			if err := applyConnectionUpdate(connection, input); err != nil {
				return mapRequestErr(err)
			}

			if err := repo.PutConnection(ctx, connection); err != nil {
				return err
			}

			return ctx.Complete(
				journal.Result{
					Action: journal.Action_Update,
					Entity: locationConnectionEntity,
					Data:   input.ID,
				}, tx,
			)
		},
	); err != nil {
		err = mapConnectionErr(err, input.ID)
		s.error("failed to update connection", "type", "Service", "func", "UpdateConnection", "id", input.ID, "error", err)
		s.fail(ctx, err, "type", "Service", "func", "UpdateConnection", "id", input.ID)
		return err
	}

	s.info("connection is updated", "type", "Service", "func", "UpdateConnection", "id", input.ID)
	return nil
}

// PatchConnectionTo sets the destination location of a connection inside a transaction.
func (s *Service) PatchConnectionTo(ctx execution.ExecutionContext[journal.Result], id, toID repositories.ID) error {
	if toID == 0 {
		err := mapRequestErr(errZeroToID)
		s.error("failed to patch connection to", "type", "Service", "func", "PatchConnectionTo", "id", id, "error", err)
		s.fail(ctx, err, "type", "Service", "func", "PatchConnectionTo", "id", id)
		return err
	}

	return s.patchConnection(ctx, "PatchConnectionTo", id, func(tx *gorm.DB) error {
		return s.generator(tx).PatchConnectionTo(ctx, id, toID)
	})
}

// PatchConnectionState sets the state of a connection inside a transaction.
func (s *Service) PatchConnectionState(ctx execution.ExecutionContext[journal.Result], id repositories.ID, state repositories.JSON) error {
	return s.patchConnection(ctx, "PatchConnectionState", id, func(tx *gorm.DB) error {
		return s.generator(tx).PatchConnectionState(ctx, id, state)
	})
}

// PatchConnectionDetails sets the details of a connection inside a transaction.
func (s *Service) PatchConnectionDetails(ctx execution.ExecutionContext[journal.Result], id repositories.ID, details repositories.JSON) error {
	return s.patchConnection(ctx, "PatchConnectionDetails", id, func(tx *gorm.DB) error {
		return s.generator(tx).PatchConnectionDetails(ctx, id, details)
	})
}

// PatchConnectionDescription sets the description of a connection inside a transaction.
func (s *Service) PatchConnectionDescription(ctx execution.ExecutionContext[journal.Result], id repositories.ID, description string) error {
	return s.patchConnection(ctx, "PatchConnectionDescription", id, func(tx *gorm.DB) error {
		return s.generator(tx).PatchConnectionDescription(ctx, id, description)
	})
}

// PatchConnectionExpiresAt sets ExpiresAt of a connection inside a transaction.
// A nil expiresAt clears the value.
func (s *Service) PatchConnectionExpiresAt(ctx execution.ExecutionContext[journal.Result], id repositories.ID, expiresAt *int64) error {
	return s.patchConnection(ctx, "PatchConnectionExpiresAt", id, func(tx *gorm.DB) error {
		return s.generator(tx).PatchConnectionExpiresAt(ctx, id, expiresAt)
	})
}

// PatchConnectionBidirectional sets Bidirectional of a connection inside a transaction.
func (s *Service) PatchConnectionBidirectional(ctx execution.ExecutionContext[journal.Result], id repositories.ID, bidirectional bool) error {
	return s.patchConnection(ctx, "PatchConnectionBidirectional", id, func(tx *gorm.DB) error {
		return s.generator(tx).PatchConnectionBidirectional(ctx, id, bidirectional)
	})
}

func (s *Service) patchConnection(
	ctx execution.ExecutionContext[journal.Result],
	funcName string,
	id repositories.ID,
	patch func(*gorm.DB) error,
) error {
	if err := s.connection.WithTransaction(
		ctx,
		func(tx *gorm.DB) error {
			if err := patch(tx); err != nil {
				return err
			}

			return ctx.Complete(
				journal.Result{
					Action: journal.Action_Patch,
					Entity: locationConnectionEntity,
					Data:   id,
				}, tx,
			)
		},
	); err != nil {
		err = mapConnectionErr(err, id)
		s.error("failed to patch connection", "type", "Service", "func", funcName, "id", id, "error", err)
		s.fail(ctx, err, "type", "Service", "func", funcName, "id", id)
		return err
	}

	s.info("connection is patched", "type", "Service", "func", funcName, "id", id)
	return nil
}
