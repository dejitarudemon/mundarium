// Package messages implements message-related tools.
package messages

import (
	"context"
	"encoding/json"

	"github.com/dejitarudemon/mundarium/internal/alarm"
	"github.com/dejitarudemon/mundarium/internal/libs/execution"
	inputsmessage "github.com/dejitarudemon/mundarium/internal/libs/inputs/messages"
	outputsmessage "github.com/dejitarudemon/mundarium/internal/libs/outputs/messages"
	servicemessage "github.com/dejitarudemon/mundarium/internal/services/messages"
	"github.com/dejitarudemon/mundarium/internal/tools"
	"github.com/dejitarudemon/mundarium/internal/tools/actions"
	"github.com/dejitarudemon/mundarium/internal/tools/definition"
)

var (
	_ tools.Tool = (*CreateMessageTool)(nil)
)

// CreateMessageTool creates a message via the messages service.
// After a successful create it Rings alarm (if non-nil) so waiters can proceed.
type CreateMessageTool struct {
	service    *servicemessage.Service
	definition definition.Definition

	alarm *alarm.Alarm
}

// NewCreateMessageTool loads the tool definition from file and binds service.
// alarm may be nil; in that case Execute skips notification.
func NewCreateMessageTool(file string, service *servicemessage.Service, alarm *alarm.Alarm) (*CreateMessageTool, error) {
	if service == nil {
		return nil, actions.ErrServiceIsNil
	}

	def, err := actions.LoadDefinition(file)
	if err != nil {
		return nil, err
	}

	return &CreateMessageTool{service: service, definition: def, alarm: alarm}, nil
}

// Definition returns a copy of the loaded tool schema.
func (t *CreateMessageTool) Definition() definition.Definition {
	return t.definition.Copy()
}

// IsIdempotent reports that create_message is not safe to retry without journaling.
func (t *CreateMessageTool) IsIdempotent() bool {
	return false
}

// Execute unmarshals provider args (text only), binds InvokeID and From from ctx,
// calls the service, and returns JSON output.
func (t *CreateMessageTool) Execute(ctx execution.ExecutionContext, args json.RawMessage) (json.RawMessage, error) {
	message, err := t.unmarshal(args)
	if err != nil {
		return nil, err
	}

	message.From = ctx.Role().String()
	message.InvokeID = ctx.InvokeID().String()

	result, err := t.service.CreateMessage(ctx, message)
	if err != nil {
		return nil, err
	}

	// Message is already persisted; a Ring failure must not fail the tool result.
	_ = t.notify(ctx)

	return t.marshal(result)
}

// unmarshal decodes provider args into CreateMessageInput (text).
// InvokeID and From are overwritten from ctx in Execute.
func (t *CreateMessageTool) unmarshal(args json.RawMessage) (inputsmessage.CreateMessageInput, error) {
	var result inputsmessage.CreateMessageInput
	if err := json.Unmarshal(args, &result); err != nil {
		return inputsmessage.CreateMessageInput{}, err
	}

	return result, nil
}

func (t *CreateMessageTool) marshal(output outputsmessage.CreateMessageOutput) (json.RawMessage, error) {
	return json.Marshal(output)
}

// notify Rings alarm when configured. A nil alarm is a no-op.
func (t *CreateMessageTool) notify(ctx context.Context) error {
	if t.alarm != nil {
		return t.alarm.Ring(ctx)
	}

	return nil
}
