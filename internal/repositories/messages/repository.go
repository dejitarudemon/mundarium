// Package messages provides data-access helpers for Message records.
package messages

import (
	"context"
	"errors"
	"time"

	"github.com/dejitarudemon/mundarium/internal/libs/types"
	"gorm.io/gorm"
)

// MessagesRepository persists and queries Message records.
type MessagesRepository struct {
	db *gorm.DB
}

// NewMessagesRepository creates a MessagesRepository backed by db.
// Returns an error if db is nil.
func NewMessagesRepository(db *gorm.DB) (*MessagesRepository, error) {
	if db == nil {
		return nil, errors.New("expected *gorm.DB, got nil")
	}

	return &MessagesRepository{db: db}, nil
}

// CreateMessage inserts a new message and returns the persisted record
// (including the generated ID, CreatedAt, and Order from BeforeCreate).
func (r *MessagesRepository) CreateMessage(ctx context.Context, message *types.Message) (*types.Message, error) {
	if err := r.db.WithContext(ctx).Create(message).Error; err != nil {
		return nil, err
	}

	return message, nil
}

// GetMessage returns the message with the given id.
// Returns gorm.ErrRecordNotFound if no row exists.
func (r *MessagesRepository) GetMessage(ctx context.Context, id types.ID) (*types.Message, error) {
	var message types.Message

	if err := r.db.WithContext(ctx).First(&message, "id = ?", id).Error; err != nil {
		return nil, err
	}

	return &message, nil
}

// GetMessages returns all messages for invokeID ordered by message_order ascending
// (0-based Order within the invoke). An empty slice is returned when none match.
func (r *MessagesRepository) GetMessages(ctx context.Context, invokeID string) ([]types.Message, error) {
	var messages []types.Message

	if err := r.db.WithContext(ctx).
		Where("invoke_id = ?", invokeID).
		Order("message_order ASC").
		Find(&messages).Error; err != nil {
		return nil, err
	}

	return messages, nil
}

// PatchDeliveredAt sets delivered_at for the message identified by id.
// A nil deliveredAt clears the field.
// Returns gorm.ErrRecordNotFound if no row was updated.
func (r *MessagesRepository) PatchDeliveredAt(ctx context.Context, id types.ID, deliveredAt *time.Time) error {
	query := r.db.WithContext(ctx).Model(&types.Message{}).Where("id = ?", id).Update("delivered_at", deliveredAt)

	if err := query.Error; err != nil {
		return err
	}

	if query.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}

	return nil
}
