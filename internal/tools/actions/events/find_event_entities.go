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
	_ tools.Tool = (*FindEventEntitiesTool)(nil)
)

// FindEventEntitiesTool searches entity–event links via the events service.
type FindEventEntitiesTool struct {
	service    *serviceevents.Service
	definition definition.Definition
	logger     logger.Logger
}

// NewFindEventEntitiesTool loads the tool definition from file and binds service.
func NewFindEventEntitiesTool(file string, service *serviceevents.Service, logger logger.Logger) (*FindEventEntitiesTool, error) {
	if service == nil {
		return nil, toolerrs.ErrServiceIsNil
	}
	def, err := actions.LoadDefinition(file)
	if err != nil {
		return nil, err
	}
	return &FindEventEntitiesTool{service: service, definition: def, logger: logger}, nil
}

func (t *FindEventEntitiesTool) Definition() definition.Definition { return t.definition.Copy() }
func (t *FindEventEntitiesTool) IsIdempotent() bool                { return true }

func (t *FindEventEntitiesTool) Execute(ctx execution.ExecutionContext[journal.Result], args json.RawMessage) (json.RawMessage, error) {
	in, err := t.unmarshal(args)
	if err != nil {
		errorf(t.logger, "failed to execute find_event_entities", "type", "FindEventEntitiesTool", "func", "Execute", "error", err)
		return nil, err
	}

	result, err := t.service.FindEventEntities(ctx, in)
	if err != nil {
		err = toolerrs.From(err)
		errorf(t.logger, "failed to execute find_event_entities", "type", "FindEventEntitiesTool", "func", "Execute", "error", err)
		return nil, err
	}

	raw, err := json.Marshal(result)
	if err != nil {
		return nil, toolerrs.NewInvalidRequest(err)
	}
	debug(t.logger, "find_event_entities is executed", "type", "FindEventEntitiesTool", "func", "Execute", "count", len(result.Entities))
	return raw, nil
}

func (t *FindEventEntitiesTool) unmarshal(args json.RawMessage) (serviceevents.FindEventEntitiesInput, error) {
	var result serviceevents.FindEventEntitiesInput
	if err := json.Unmarshal(args, &result); err != nil {
		return serviceevents.FindEventEntitiesInput{}, toolerrs.NewInvalidRequest(err)
	}
	return result, nil
}
