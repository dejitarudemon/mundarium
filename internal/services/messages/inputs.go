package messages

// CreateMessageInput carries data required to create a new message.
//
// Text comes from the provider tool args. InvokeID and From are set from
// execution.ExecutionContext after unmarshal (not in the tool schema).
type CreateMessageInput struct {
	InvokeID string `json:"-"`
	Text     string `json:"text"`
	From     string `json:"-"`
}
