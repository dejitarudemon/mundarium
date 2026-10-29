// Package events implements event-related tools.
package events

import (
	"encoding/json"

	"github.com/dejitarudemon/mundarium/internal/libs/execution"
	journal "github.com/dejitarudemon/mundarium/internal/libs/journals/execution"
	"github.com/dejitarudemon/mundarium/internal/libs/logger"
	serviceevents "github.com/dejitarudemon/mundarium/internal/services/events"
	"github.com/dejitarudemon/mundarium/internal/tools"
	"github.com/dejitarudemon/mundarium/internal/tools/actions"
	"github.com/dejitarudemon/mundarium/internal/tools/definition"
	toolerrs "github.com/dejitarudemon/mundarium/internal/tools/errs"
)

var (
	_ tools.Tool = (*CreateEventTool)(nil)
)

// CreateEventTool creates an event via the events service.
type CreateEventTool struct {
	service    *serviceevents.Service
	definition definition.Definition
	logger     logger.Logger
}

// NewCreateEventTool loads the tool definition from file and binds service.
// logger may be nil.
func NewCreateEventTool(file string, service *serviceevents.Service, logger logger.Logger) (*CreateEventTool, error) {
	if service == nil {
		return nil, toolerrs.ErrServiceIsNil
	}

	def, err := actions.LoadDefinition(file)
	if err != nil {
		return nil, err
	}

	return &CreateEventTool{service: service, definition: def, logger: logger}, nil
}

// Definition returns a copy of the loaded tool schema.
func (t *CreateEventTool) Definition() definition.Definition {
	return t.definition.Copy()
}

// IsIdempotent reports that create_event is not safe to retry without journaling.
func (t *CreateEventTool) IsIdempotent() bool {
	return false
}

// Execute unmarshals create-event args, calls the service, and returns JSON output.
func (t *CreateEventTool) Execute(ctx execution.ExecutionContext[journal.Result], args json.RawMessage) (json.RawMessage, error) {
	in, err := t.unmarshal(args)
	if err != nil {
		errorf(t.logger, "failed to execute create_event", "type", "CreateEventTool", "func", "Execute", "error", err)
		return nil, err
	}

	result, err := t.service.CreateEvent(ctx, in)
	if err != nil {
		err = toolerrs.From(err)
		errorf(t.logger, "failed to execute create_event", "type", "CreateEventTool", "func", "Execute", "error", err)
		return nil, err
	}

	raw, err := t.marshal(result)
	if err != nil {
		errorf(t.logger, "failed to execute create_event", "type", "CreateEventTool", "func", "Execute", "error", err)
		return nil, err
	}

	info(t.logger, "create_event is executed", "type", "CreateEventTool", "func", "Execute", "data", result)
	return raw, nil
}

func (t *CreateEventTool) unmarshal(args json.RawMessage) (serviceevents.CreateEventInput, error) {
	var result serviceevents.CreateEventInput
	if err := json.Unmarshal(args, &result); err != nil {
		return serviceevents.CreateEventInput{}, toolerrs.NewInvalidRequest(err)
	}
	return result, nil
}

func (t *CreateEventTool) marshal(output serviceevents.CreateEventOutput) (json.RawMessage, error) {
	raw, err := json.Marshal(output)
	if err != nil {
		return nil, toolerrs.NewInvalidRequest(err)
	}
	return raw, nil
}
