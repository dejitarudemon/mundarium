package types

import (
	"time"

	"github.com/dejitarudemon/mundarium/internal/libs/providers"
	"gorm.io/gorm"
)

// Message is one turn in an invoke conversation.
//
// Order is unique together with InvokeID (column message_order) and is assigned
// automatically in BeforeCreate as the next 0-based index for that invoke.
type Message struct {
	ID ID `gorm:"primaryKey" json:"id"`

	InvokeID string `gorm:"not null;uniqueIndex:idx_messages_invoke_order" json:"invoke_id"`
	Order    uint   `gorm:"column:message_order;not null;uniqueIndex:idx_messages_invoke_order" json:"order"`

	Text string `gorm:"type:text;not null" json:"text"`

	// From is the speaker role (for example providers.Role_Master).
	From providers.Role `gorm:"not null;type:text" json:"from"`

	CreatedAt time.Time `gorm:"not null" json:"created_at"`

	// DeliveredAt is set when the message was delivered to the client; nil if not yet.
	DeliveredAt *time.Time `json:"delivered_at,omitempty"`
}

// BeforeCreate assigns Order as the next 0-based index within InvokeID.
func (m *Message) BeforeCreate(tx *gorm.DB) error {
	var n int64
	if err := tx.Model(&Message{}).Where("invoke_id = ?", m.InvokeID).Count(&n).Error; err != nil {
		return err
	}
	m.Order = uint(n)
	return nil
}
