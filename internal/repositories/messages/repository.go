// Package messages provides data-access helpers for Message records.
package messages

import (
	"context"
	"errors"
	"time"

	"github.com/dejitarudemon/mundarium/internal/libs/logger"
	"github.com/dejitarudemon/mundarium/internal/repositories"
	"gorm.io/gorm"
)

// MessagesRepository persists and queries Message records.
type MessagesRepository struct {
	db     *gorm.DB
	logger logger.Logger
}

// NewMessagesRepository creates a MessagesRepository backed by db.
// Returns an error if db is nil. logger may be nil.
func NewMessagesRepository(db *gorm.DB, logger logger.Logger) (*MessagesRepository, error) {
	if db == nil {
		return nil, errors.New("expected *gorm.DB, got nil")
	}

	return &MessagesRepository{db: db, logger: logger}, nil
}

// CreateMessage inserts a new message and returns the persisted record
// (including the generated ID, CreatedAt, and SeqNum from BeforeCreate).
func (r *MessagesRepository) CreateMessage(ctx context.Context, message *Message) (*Message, error) {
	if err := r.db.WithContext(ctx).Create(message).Error; err != nil {
		r.error("failed to create message", "type", "MessagesRepository", "func", "CreateMessage", "data", message, "error", err)
		return nil, err
	}

	r.info("message is created", "type", "MessagesRepository", "func", "CreateMessage", "data", message)
	return message, nil
}

// GetMessage returns the message with the given id.
// Returns gorm.ErrRecordNotFound if no row exists.
func (r *MessagesRepository) GetMessage(ctx context.Context, id repositories.ID) (*Message, error) {
	var message Message

	if err := r.db.WithContext(ctx).First(&message, "id = ?", id).Error; err != nil {
		r.error("failed to get message", "type", "MessagesRepository", "func", "GetMessage", "id", id, "error", err)
		return nil, err
	}

	r.debug("got message", "type", "MessagesRepository", "func", "GetMessage", "data", &message)
	return &message, nil
}

// GetMessages returns messages for invokeID ordered by seq_num ascending
// (0-based SeqNum within the invoke). An empty slice is returned when none match.
// When startFrom is non-nil, only rows with seq_num >= *startFrom are returned
// (filter applied in SQL).
func (r *MessagesRepository) GetMessages(ctx context.Context, invokeID string, startFrom *uint) ([]Message, error) {
	var messages []Message

	q := r.db.WithContext(ctx).Where("invoke_id = ?", invokeID)
	if startFrom != nil {
		q = q.Where("seq_num >= ?", *startFrom)
	}

	if err := q.Order("seq_num ASC").Find(&messages).Error; err != nil {
		r.error("failed to get messages", "type", "MessagesRepository", "func", "GetMessages", "invoke_id", invokeID, "error", err)
		return nil, err
	}

	r.debug("got messages", "type", "MessagesRepository", "func", "GetMessages", "invoke_id", invokeID, "count", len(messages))
	return messages, nil
}

// PatchDeliveredAt sets delivered_at for the message identified by id.
// A nil deliveredAt clears the field.
// Returns gorm.ErrRecordNotFound if no row was updated.
func (r *MessagesRepository) PatchDeliveredAt(ctx context.Context, id repositories.ID, deliveredAt *time.Time) error {
	query := r.db.WithContext(ctx).Model(&Message{}).Where("id = ?", id).Update("delivered_at", deliveredAt)

	if err := query.Error; err != nil {
		r.error("failed to patch delivered_at", "type", "MessagesRepository", "func", "PatchDeliveredAt", "id", id, "error", err)
		return err
	}

	if query.RowsAffected == 0 {
		r.error("failed to patch delivered_at", "type", "MessagesRepository", "func", "PatchDeliveredAt", "id", id, "error", gorm.ErrRecordNotFound)
		return gorm.ErrRecordNotFound
	}

	r.info("delivered_at is patched", "type", "MessagesRepository", "func", "PatchDeliveredAt", "id", id)
	return nil
}

// DeleteMessages deletes all messages with the given invokeID.
// Deleting when none exist is a no-op (idempotent).
func (r *MessagesRepository) DeleteMessages(ctx context.Context, invokeID string) error {
	if err := r.db.WithContext(ctx).Delete(&Message{}, "invoke_id = ?", invokeID).Error; err != nil {
		r.error("failed to delete messages", "type", "MessagesRepository", "func", "DeleteMessages", "invoke_id", invokeID, "error", err)
		return err
	}
	r.info("messages are deleted", "type", "MessagesRepository", "func", "DeleteMessages", "invoke_id", invokeID)
	return nil
}
