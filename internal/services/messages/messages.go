package messages

import (
	"errors"
	"time"

	"github.com/dejitarudemon/mundarium/internal/libs/execution"
	journal "github.com/dejitarudemon/mundarium/internal/libs/journals/execution"
	"github.com/dejitarudemon/mundarium/internal/repositories"
	"gorm.io/gorm"
)

const messageEntity = "MESSAGE"

// CreateMessage creates a message inside a transaction and completes the execution context.
// Not idempotent: each successful call inserts a new message.
func (s *Service) CreateMessage(ctx execution.ExecutionContext[journal.Result], in CreateMessageInput) (CreateMessageOutput, error) {
	message, err := toNewMessage(in)
	if err != nil {
		err = mapRequestErr(err)
		s.error("failed to create message", "type", "Service", "func", "CreateMessage", "error", err)
		s.fail(ctx, err, "type", "Service", "func", "CreateMessage")
		return CreateMessageOutput{}, err
	}

	result := CreateMessageOutput{}

	if err := s.connection.WithTransaction(
		ctx,
		func(tx *gorm.DB) error {
			created, err := s.generator(tx).CreateMessage(ctx, message)
			if err != nil {
				return err
			}

			result, err = toCreatedMessage(*created)
			if err != nil {
				return err
			}

			return ctx.Complete(
				journal.Result{
					Action: journal.Action_Create,
					Entity: messageEntity,
					Data:   result,
				}, tx,
			)
		},
	); err != nil {
		err = mapMessageErr(err, result.ID)
		s.error("failed to create message", "type", "Service", "func", "CreateMessage", "error", err)
		s.fail(ctx, err, "type", "Service", "func", "CreateMessage")
		return CreateMessageOutput{}, err
	}

	s.info("message is created", "type", "Service", "func", "CreateMessage", "data", result)
	return result, nil
}

// GetMessage loads a message by id, maps it to output, and completes the execution context.
// Idempotent: repeated calls with the same id return the same row.
func (s *Service) GetMessage(ctx execution.ExecutionContext[journal.Result], id repositories.ID) (GetMessageOutput, error) {
	message, err := s.repository().GetMessage(ctx, id)
	if err != nil {
		err = mapMessageErr(err, id)
		s.error("failed to get message", "type", "Service", "func", "GetMessage", "id", id, "error", err)
		s.fail(ctx, err, "type", "Service", "func", "GetMessage", "id", id)
		return GetMessageOutput{}, err
	}

	result, err := toGotMessage(*message)
	if err != nil {
		err = mapRequestErr(err)
		s.error("failed to get message", "type", "Service", "func", "GetMessage", "id", id, "error", err)
		s.fail(ctx, err, "type", "Service", "func", "GetMessage", "id", id)
		return GetMessageOutput{}, err
	}

	if err := ctx.Complete(
		journal.Result{
			Action: journal.Action_Get,
			Entity: messageEntity,
			Data:   result,
		}, s.connection.DB(),
	); err != nil {
		err = mapMessageErr(err, id)
		s.error("failed to get message", "type", "Service", "func", "GetMessage", "id", id, "error", err)
		s.fail(ctx, err, "type", "Service", "func", "GetMessage", "id", id)
		return GetMessageOutput{}, err
	}

	s.debug("got message", "type", "Service", "func", "GetMessage", "data", result)
	return result, nil
}

// GetMessages loads messages for invokeID ordered by seq_num and completes the execution context.
// When startFrom is non-nil, messages with SeqNum < *startFrom are omitted (SQL-side filter).
// Returns an error if invokeID is empty.
// Idempotent: repeated calls with the same invokeID and startFrom return the same ordered set.
func (s *Service) GetMessages(ctx execution.ExecutionContext[journal.Result], invokeID string, startFrom *uint) (GetMessagesOutput, error) {
	if invokeID == "" {
		err := mapRequestErr(errors.New("invoke id is empty"))
		s.error("failed to get messages", "type", "Service", "func", "GetMessages", "error", err)
		s.fail(ctx, err, "type", "Service", "func", "GetMessages")
		return nil, err
	}

	messages, err := s.repository().GetMessages(ctx, invokeID, startFrom)
	if err != nil {
		err = mapMessageKeyErr(err, invokeID)
		s.error("failed to get messages", "type", "Service", "func", "GetMessages", "invoke_id", invokeID, "error", err)
		s.fail(ctx, err, "type", "Service", "func", "GetMessages", "invoke_id", invokeID)
		return nil, err
	}

	result, err := toGotMessages(messages)
	if err != nil {
		err = mapRequestErr(err)
		s.error("failed to get messages", "type", "Service", "func", "GetMessages", "invoke_id", invokeID, "error", err)
		s.fail(ctx, err, "type", "Service", "func", "GetMessages", "invoke_id", invokeID)
		return nil, err
	}

	if err := ctx.Complete(
		journal.Result{
			Action: journal.Action_Get,
			Entity: messageEntity,
			Data:   result,
		}, s.connection.DB(),
	); err != nil {
		err = mapMessageKeyErr(err, invokeID)
		s.error("failed to get messages", "type", "Service", "func", "GetMessages", "invoke_id", invokeID, "error", err)
		s.fail(ctx, err, "type", "Service", "func", "GetMessages", "invoke_id", invokeID)
		return nil, err
	}

	s.debug("got messages", "type", "Service", "func", "GetMessages", "invoke_id", invokeID, "count", len(result))
	return result, nil
}

// SetDeliveredAt updates delivered_at for the message identified by id inside a transaction
// and completes the execution context. A nil deliveredAt clears the field.
// Idempotent: repeating the same id and timestamp yields the same stored value.
func (s *Service) SetDeliveredAt(ctx execution.ExecutionContext[journal.Result], id repositories.ID, deliveredAt *time.Time) error {
	if err := s.connection.WithTransaction(
		ctx,
		func(tx *gorm.DB) error {
			if err := s.generator(tx).PatchDeliveredAt(ctx, id, deliveredAt); err != nil {
				return err
			}

			return ctx.Complete(
				journal.Result{
					Action: journal.Action_Patch,
					Entity: messageEntity,
					Data:   id,
				}, tx,
			)
		},
	); err != nil {
		err = mapMessageErr(err, id)
		s.error("failed to set delivered_at", "type", "Service", "func", "SetDeliveredAt", "id", id, "error", err)
		s.fail(ctx, err, "type", "Service", "func", "SetDeliveredAt", "id", id)
		return err
	}

	s.info("delivered_at is set", "type", "Service", "func", "SetDeliveredAt", "id", id)
	return nil
}

// DeleteMessages deletes all messages for invokeID inside a transaction and completes the execution context.
// Returns an error if invokeID is empty.
// Idempotent: repeating the same invokeID when no rows remain succeeds.
func (s *Service) DeleteMessages(ctx execution.ExecutionContext[journal.Result], invokeID string) error {
	if invokeID == "" {
		err := mapRequestErr(errors.New("invoke id is empty"))
		s.error("failed to delete messages", "type", "Service", "func", "DeleteMessages", "error", err)
		s.fail(ctx, err, "type", "Service", "func", "DeleteMessages")
		return err
	}

	if err := s.connection.WithTransaction(
		ctx,
		func(tx *gorm.DB) error {
			if err := s.generator(tx).DeleteMessages(ctx, invokeID); err != nil {
				return err
			}

			return ctx.Complete(
				journal.Result{
					Action: journal.Action_Delete,
					Entity: messageEntity,
					Data:   invokeID,
				}, tx,
			)
		},
	); err != nil {
		err = mapMessageKeyErr(err, invokeID)
		s.error("failed to delete messages", "type", "Service", "func", "DeleteMessages", "invoke_id", invokeID, "error", err)
		s.fail(ctx, err, "type", "Service", "func", "DeleteMessages", "invoke_id", invokeID)
		return err
	}

	s.info("messages are deleted", "type", "Service", "func", "DeleteMessages", "invoke_id", invokeID)
	return nil
}
