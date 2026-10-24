// Package messages defines output DTOs for message-related use cases.
package messages

import "github.com/dejitarudemon/mundarium/internal/libs/types"

// CreateMessageOutput is returned after a message has been created.
type CreateMessageOutput struct {
	ID       types.ID `json:"id"`
	InvokeID string   `json:"invoke_id"`
	SeqNum   uint     `json:"seq_num"` // 0-based sequence within InvokeID
}
