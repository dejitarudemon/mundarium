package messages

import (
	"errors"

	input "github.com/dejitarudemon/mundarium/internal/libs/inputs/messages"
	output "github.com/dejitarudemon/mundarium/internal/libs/outputs/messages"
	"github.com/dejitarudemon/mundarium/internal/libs/providers"
	"github.com/dejitarudemon/mundarium/internal/libs/types"
)

// toNewMessage maps CreateMessageInput to a domain Message ready for insert.
func toNewMessage(in input.CreateMessageInput) (*types.Message, error) {
	if in.InvokeID == "" {
		return nil, errors.New("invoke id is empty")
	}
	if in.Text == "" {
		return nil, errors.New("text is empty")
	}
	if in.From == "" {
		return nil, errors.New("from is empty")
	}

	return &types.Message{
		InvokeID: in.InvokeID,
		Text:     in.Text,
		From:     providers.Role(in.From),
	}, nil
}

// toCreatedMessage maps a persisted Message to CreateMessageOutput.
func toCreatedMessage(message types.Message) (output.CreateMessageOutput, error) {
	if err := checkMessage(message); err != nil {
		return output.CreateMessageOutput{}, err
	}

	return output.CreateMessageOutput{
		ID:       message.ID,
		InvokeID: message.InvokeID,
		SeqNum:   message.SeqNum,
	}, nil
}

// toGotMessage maps a persisted Message to GetMessageOutput.
func toGotMessage(message types.Message) (output.GetMessageOutput, error) {
	if err := checkMessage(message); err != nil {
		return output.GetMessageOutput{}, err
	}

	return output.GetMessageOutput{
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
func toGotMessages(messages []types.Message) (output.GetMessagesOutput, error) {
	result := make(output.GetMessagesOutput, 0, len(messages))
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
func checkMessage(message types.Message) error {
	if message.ID == 0 {
		return errors.New("message id is zero")
	}
	if message.InvokeID == "" {
		return errors.New("invoke id is empty")
	}

	return nil
}
