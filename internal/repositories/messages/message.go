package messages

import (
	"time"

	"github.com/dejitarudemon/mundarium/internal/providers"
	"github.com/dejitarudemon/mundarium/internal/repositories"
	"gorm.io/gorm"
)

// Message is one turn in an invoke conversation.
//
// SeqNum is unique together with InvokeID (column seq_num) and is assigned
// automatically in BeforeCreate as the next 0-based index for that invoke.
type Message struct {
	ID repositories.ID `gorm:"primaryKey" json:"id"`

	InvokeID string `gorm:"not null;uniqueIndex:idx_messages_invoke_seq" json:"invoke_id"`
	SeqNum   uint   `gorm:"column:seq_num;not null;uniqueIndex:idx_messages_invoke_seq" json:"seq_num"`

	Text string `gorm:"type:text;not null" json:"text"`

	// From is the speaker role (for example providers.Role_Master).
	From providers.Role `gorm:"not null;type:text" json:"from"`

	CreatedAt time.Time `gorm:"not null" json:"created_at"`

	// DeliveredAt is set when the message was delivered to the client; nil if not yet.
	DeliveredAt *time.Time `json:"delivered_at,omitempty"`
}

// BeforeCreate assigns SeqNum as the next 0-based index within InvokeID.
func (m *Message) BeforeCreate(tx *gorm.DB) error {
	var n int64
	if err := tx.Model(&Message{}).Where("invoke_id = ?", m.InvokeID).Count(&n).Error; err != nil {
		return err
	}
	m.SeqNum = uint(n)
	return nil
}
