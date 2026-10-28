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
	_ tools.Tool = (*CreateEventCausalTool)(nil)
)

// CreateEventCausalTool creates a causal edge via the events service.
type CreateEventCausalTool struct {
	service    *serviceevents.Service
	definition definition.Definition
	logger     logger.Logger
}

// NewCreateEventCausalTool loads the tool definition from file and binds service.
func NewCreateEventCausalTool(file string, service *serviceevents.Service, logger logger.Logger) (*CreateEventCausalTool, error) {
	if service == nil {
		return nil, toolerrs.ErrServiceIsNil
	}
	def, err := actions.LoadDefinition(file)
	if err != nil {
		return nil, err
	}
	return &CreateEventCausalTool{service: service, definition: def, logger: logger}, nil
}

func (t *CreateEventCausalTool) Definition() definition.Definition { return t.definition.Copy() }
func (t *CreateEventCausalTool) IsIdempotent() bool                { return false }

func (t *CreateEventCausalTool) Execute(ctx execution.ExecutionContext[journal.Result], args json.RawMessage) (json.RawMessage, error) {
	in, err := t.unmarshal(args)
	if err != nil {
		errorf(t.logger, "failed to execute create_event_causal", "type", "CreateEventCausalTool", "func", "Execute", "error", err)
		return nil, err
	}

	result, err := t.service.CreateEventCausal(ctx, in)
	if err != nil {
		err = toolerrs.From(err)
		errorf(t.logger, "failed to execute create_event_causal", "type", "CreateEventCausalTool", "func", "Execute", "error", err)
		return nil, err
	}

	raw, err := json.Marshal(result)
	if err != nil {
		return nil, toolerrs.NewInvalidRequest(err)
	}
	info(t.logger, "create_event_causal is executed", "type", "CreateEventCausalTool", "func", "Execute", "data", result)
	return raw, nil
}

func (t *CreateEventCausalTool) unmarshal(args json.RawMessage) (serviceevents.CreateEventCausalInput, error) {
	var result serviceevents.CreateEventCausalInput
	if err := json.Unmarshal(args, &result); err != nil {
		return serviceevents.CreateEventCausalInput{}, toolerrs.NewInvalidRequest(err)
	}
	if err := requireEventID(result.From); err != nil {
		return serviceevents.CreateEventCausalInput{}, err
	}
	if err := requireEventID(result.To); err != nil {
		return serviceevents.CreateEventCausalInput{}, err
	}
	return result, nil
}
