// Package messages implements message-related tools.
package messages

import (
	"context"
	"encoding/json"

	"github.com/dejitarudemon/mundarium/internal/libs/alarm"
	"github.com/dejitarudemon/mundarium/internal/libs/execution"
	journal "github.com/dejitarudemon/mundarium/internal/libs/journals/execution"
	"github.com/dejitarudemon/mundarium/internal/libs/logger"
	servicemessage "github.com/dejitarudemon/mundarium/internal/services/messages"
	"github.com/dejitarudemon/mundarium/internal/tools"
	"github.com/dejitarudemon/mundarium/internal/tools/actions"
	"github.com/dejitarudemon/mundarium/internal/tools/definition"
	toolerrs "github.com/dejitarudemon/mundarium/internal/tools/errs"
)

var (
	_ tools.Tool = (*CreateMessageTool)(nil)
)

// CreateMessageTool creates a message via the messages service.
// After a successful create it Rings alarm (if non-nil) so waiters can proceed.
type CreateMessageTool struct {
	service    *servicemessage.Service
	definition definition.Definition
	logger     logger.Logger

	alarm *alarm.Alarm
}

// NewCreateMessageTool loads the tool definition from file and binds service.
// alarm and logger may be nil; a nil alarm skips notification.
func NewCreateMessageTool(file string, service *servicemessage.Service, alarm *alarm.Alarm, logger logger.Logger) (*CreateMessageTool, error) {
	if service == nil {
		return nil, toolerrs.ErrServiceIsNil
	}

	def, err := actions.LoadDefinition(file)
	if err != nil {
		return nil, err
	}

	return &CreateMessageTool{service: service, definition: def, alarm: alarm, logger: logger}, nil
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
func (t *CreateMessageTool) Execute(ctx execution.ExecutionContext[journal.Result], args json.RawMessage) (json.RawMessage, error) {
	message, err := t.unmarshal(args)
	if err != nil {
		t.error("failed to execute create_message", "type", "CreateMessageTool", "func", "Execute", "error", err)
		return nil, err
	}

	message.From = ctx.Role().String()
	message.InvokeID = ctx.InvokeID().String()

	result, err := t.service.CreateMessage(ctx, message)
	if err != nil {
		err = toolerrs.From(err)
		t.error("failed to execute create_message", "type", "CreateMessageTool", "func", "Execute", "error", err)
		return nil, err
	}

	// Message is already persisted; a Ring failure must not fail the tool result.
	if err := t.notify(ctx); err != nil {
		t.warn("failed to ring alarm after create_message", "type", "CreateMessageTool", "func", "Execute", "error", err)
	}

	raw, err := t.marshal(result)
	if err != nil {
		t.error("failed to execute create_message", "type", "CreateMessageTool", "func", "Execute", "error", err)
		return nil, err
	}

	t.info("create_message is executed", "type", "CreateMessageTool", "func", "Execute", "data", result)
	return raw, nil
}

// unmarshal decodes provider args into CreateMessageInput (text).
// InvokeID and From are overwritten from ctx in Execute.
func (t *CreateMessageTool) unmarshal(args json.RawMessage) (servicemessage.CreateMessageInput, error) {
	var result servicemessage.CreateMessageInput
	if err := json.Unmarshal(args, &result); err != nil {
		return servicemessage.CreateMessageInput{}, toolerrs.NewInvalidRequest(err)
	}

	return result, nil
}

func (t *CreateMessageTool) marshal(output servicemessage.CreateMessageOutput) (json.RawMessage, error) {
	raw, err := json.Marshal(output)
	if err != nil {
		return nil, toolerrs.NewInvalidRequest(err)
	}
	return raw, nil
}

// notify Rings alarm when configured. A nil alarm is a no-op.
func (t *CreateMessageTool) notify(ctx context.Context) error {
	if t.alarm != nil {
		return t.alarm.Ring(ctx)
	}

	return nil
}
