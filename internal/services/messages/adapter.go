package messages

import (
	"errors"
	repomessages "github.com/dejitarudemon/mundarium/internal/repositories/messages"

	"github.com/dejitarudemon/mundarium/internal/providers"
)

// toNewMessage maps CreateMessageInput to a domain Message ready for insert.
func toNewMessage(in CreateMessageInput) (*repomessages.Message, error) {
	if in.InvokeID == "" {
		return nil, errors.New("invoke id is empty")
	}
	if in.Text == "" {
		return nil, errors.New("text is empty")
	}
	if in.From == "" {
		return nil, errors.New("from is empty")
	}

	return &repomessages.Message{
		InvokeID: in.InvokeID,
		Text:     in.Text,
		From:     providers.Role(in.From),
	}, nil
}

// toCreatedMessage maps a persisted Message to CreateMessageOutput.
func toCreatedMessage(message repomessages.Message) (CreateMessageOutput, error) {
	if err := checkMessage(message); err != nil {
		return CreateMessageOutput{}, err
	}

	return CreateMessageOutput{
		ID:       message.ID,
		InvokeID: message.InvokeID,
		SeqNum:   message.SeqNum,
	}, nil
}

// toGotMessage maps a persisted Message to GetMessageOutput.
func toGotMessage(message repomessages.Message) (GetMessageOutput, error) {
	if err := checkMessage(message); err != nil {
		return GetMessageOutput{}, err
	}

	return GetMessageOutput{
		ID:          message.ID,
		InvokeID:    message.InvokeID,
		SeqNum:      message.SeqNum,
		Text:        message.Text,
		From:        string(message.From),
		CreatedAt:   message.CreatedAt,
		DeliveredAt: message.DeliveredAt,
	}, nil
}

// toGotMessages maps persisted messages to GetMessagesOutput.
func toGotMessages(messages []repomessages.Message) (GetMessagesOutput, error) {
	result := make(GetMessagesOutput, 0, len(messages))
	for _, msg := range messages {
		item, err := toGotMessage(msg)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, nil
}

// checkMessage validates required identity fields on a Message.
func checkMessage(message repomessages.Message) error {
	if message.ID == 0 {
		return errors.New("message id is zero")
	}
	if message.InvokeID == "" {
		return errors.New("invoke id is empty")
	}

	return nil
}
