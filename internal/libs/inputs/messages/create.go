// Package messages defines input DTOs for message-related use cases.
package messages

// CreateMessageInput carries data required to create a new message.
type CreateMessageInput struct {
	InvokeID string `json:"invoke_id"` // conversation / invoke group id
	Text     string `json:"text"`      // message body
	From     string `json:"from"`      // speaker role, e.g. "master"
}
