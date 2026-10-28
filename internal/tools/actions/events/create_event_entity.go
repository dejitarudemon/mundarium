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
	_ tools.Tool = (*CreateEventEntityTool)(nil)
)

// CreateEventEntityTool links an entity to an event via the events service.
type CreateEventEntityTool struct {
	service    *serviceevents.Service
	definition definition.Definition
	logger     logger.Logger
}

// NewCreateEventEntityTool loads the tool definition from file and binds service.
func NewCreateEventEntityTool(file string, service *serviceevents.Service, logger logger.Logger) (*CreateEventEntityTool, error) {
	if service == nil {
		return nil, toolerrs.ErrServiceIsNil
	}
	def, err := actions.LoadDefinition(file)
	if err != nil {
		return nil, err
	}
	return &CreateEventEntityTool{service: service, definition: def, logger: logger}, nil
}

func (t *CreateEventEntityTool) Definition() definition.Definition { return t.definition.Copy() }
func (t *CreateEventEntityTool) IsIdempotent() bool                { return false }

func (t *CreateEventEntityTool) Execute(ctx execution.ExecutionContext[journal.Result], args json.RawMessage) (json.RawMessage, error) {
	in, err := t.unmarshal(args)
	if err != nil {
		errorf(t.logger, "failed to execute create_event_entity", "type", "CreateEventEntityTool", "func", "Execute", "error", err)
		return nil, err
	}

	result, err := t.service.CreateEventEntity(ctx, in)
	if err != nil {
		err = toolerrs.From(err)
		errorf(t.logger, "failed to execute create_event_entity", "type", "CreateEventEntityTool", "func", "Execute", "error", err)
		return nil, err
	}

	raw, err := json.Marshal(result)
	if err != nil {
		return nil, toolerrs.NewInvalidRequest(err)
	}
	info(t.logger, "create_event_entity is executed", "type", "CreateEventEntityTool", "func", "Execute", "data", result)
	return raw, nil
}

func (t *CreateEventEntityTool) unmarshal(args json.RawMessage) (serviceevents.CreateEventEntityInput, error) {
	var result serviceevents.CreateEventEntityInput
	if err := json.Unmarshal(args, &result); err != nil {
		return serviceevents.CreateEventEntityInput{}, toolerrs.NewInvalidRequest(err)
	}
	if _, err := parseEntityID(result.EntityID); err != nil {
		return serviceevents.CreateEventEntityInput{}, err
	}
	if err := requireEventID(result.EventID); err != nil {
		return serviceevents.CreateEventEntityInput{}, err
	}
	return result, nil
}
