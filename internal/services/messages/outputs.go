package messages

import (
	"time"

	"github.com/dejitarudemon/mundarium/internal/repositories"
)

// CreateMessageOutput is returned after a message has been created.
type CreateMessageOutput struct {
	ID       repositories.ID `json:"id"`
	InvokeID string          `json:"invoke_id"`
	SeqNum   uint            `json:"seq_num"` // 0-based sequence within InvokeID
}

// GetMessageOutput is returned when fetching a single message.
type GetMessageOutput struct {
	ID       repositories.ID `json:"id"`
	InvokeID string          `json:"invoke_id"`
	SeqNum   uint            `json:"seq_num"` // 0-based sequence within InvokeID
	Text     string          `json:"text"`
	From     string          `json:"from"` // speaker role name

	CreatedAt   time.Time  `json:"created_at"`
	DeliveredAt *time.Time `json:"delivered_at,omitempty"` // nil until marked delivered
}

// GetMessagesOutput is messages for one invoke ordered by SeqNum ascending.
type GetMessagesOutput []GetMessageOutput
