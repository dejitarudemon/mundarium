package messages

import (
	"errors"
	"github.com/dejitarudemon/mundarium/internal/repositories"
	"time"

	"github.com/dejitarudemon/mundarium/internal/libs/execution"
	journal "github.com/dejitarudemon/mundarium/internal/libs/journals/execution"
	"gorm.io/gorm"
)

const messageEntity = "MESSAGE"

// CreateMessage creates a message inside a transaction and completes the execution context.
// Not idempotent: each successful call inserts a new message.
func (s *Service) CreateMessage(ctx execution.ExecutionContext[journal.Result], in CreateMessageInput) (CreateMessageOutput, error) {
	message, err := toNewMessage(in)
	if err != nil {
		_ = ctx.Fail(err, s.connection.DB())
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
		_ = ctx.Fail(err, s.connection.DB())
		return CreateMessageOutput{}, err
	}

	return result, nil
}

// GetMessage loads a message by id, maps it to output, and completes the execution context.
// Idempotent: repeated calls with the same id return the same row.
func (s *Service) GetMessage(ctx execution.ExecutionContext[journal.Result], id repositories.ID) (GetMessageOutput, error) {
	message, err := s.repository().GetMessage(ctx, id)
	if err != nil {
		_ = ctx.Fail(err, s.connection.DB())
		return GetMessageOutput{}, err
	}

	result, err := toGotMessage(*message)
	if err != nil {
		_ = ctx.Fail(err, s.connection.DB())
		return GetMessageOutput{}, err
	}

	if err := ctx.Complete(
		journal.Result{
			Action: journal.Action_Get,
			Entity: messageEntity,
			Data:   result,
		}, s.connection.DB(),
	); err != nil {
		_ = ctx.Fail(err, s.connection.DB())
		return GetMessageOutput{}, err
	}

	return result, nil
}

// GetMessages loads messages for invokeID ordered by seq_num and completes the execution context.
// When startFrom is non-nil, messages with SeqNum < *startFrom are omitted (SQL-side filter).
// Returns an error if invokeID is empty.
// Idempotent: repeated calls with the same invokeID and startFrom return the same ordered set.
func (s *Service) GetMessages(ctx execution.ExecutionContext[journal.Result], invokeID string, startFrom *uint) (GetMessagesOutput, error) {
	if invokeID == "" {
		err := errors.New("invoke id is empty")
		_ = ctx.Fail(err, s.connection.DB())
		return nil, err
	}

	messages, err := s.repository().GetMessages(ctx, invokeID, startFrom)
	if err != nil {
		_ = ctx.Fail(err, s.connection.DB())
		return nil, err
	}

	result, err := toGotMessages(messages)
	if err != nil {
		_ = ctx.Fail(err, s.connection.DB())
		return nil, err
	}

	if err := ctx.Complete(
		journal.Result{
			Action: journal.Action_Get,
			Entity: messageEntity,
			Data:   result,
		}, s.connection.DB(),
	); err != nil {
		_ = ctx.Fail(err, s.connection.DB())
		return nil, err
	}

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
		_ = ctx.Fail(err, s.connection.DB())
		return err
	}

	return nil
}

// DeleteMessages deletes all messages for invokeID inside a transaction and completes the execution context.
// Returns an error if invokeID is empty.
// Idempotent: repeating the same invokeID when no rows remain succeeds.
func (s *Service) DeleteMessages(ctx execution.ExecutionContext[journal.Result], invokeID string) error {
	if invokeID == "" {
		err := errors.New("invoke id is empty")
		_ = ctx.Fail(err, s.connection.DB())
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
		_ = ctx.Fail(err, s.connection.DB())
		return err
	}

	return nil
}
