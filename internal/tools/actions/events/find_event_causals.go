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
	_ tools.Tool = (*FindEventCausalsTool)(nil)
)

// FindEventCausalsTool searches causal edges via the events service.
type FindEventCausalsTool struct {
	service    *serviceevents.Service
	definition definition.Definition
	logger     logger.Logger
}

// NewFindEventCausalsTool loads the tool definition from file and binds service.
func NewFindEventCausalsTool(file string, service *serviceevents.Service, logger logger.Logger) (*FindEventCausalsTool, error) {
	if service == nil {
		return nil, toolerrs.ErrServiceIsNil
	}
	def, err := actions.LoadDefinition(file)
	if err != nil {
		return nil, err
	}
	return &FindEventCausalsTool{service: service, definition: def, logger: logger}, nil
}

func (t *FindEventCausalsTool) Definition() definition.Definition { return t.definition.Copy() }
func (t *FindEventCausalsTool) IsIdempotent() bool                { return true }

func (t *FindEventCausalsTool) Execute(ctx execution.ExecutionContext[journal.Result], args json.RawMessage) (json.RawMessage, error) {
	in, err := t.unmarshal(args)
	if err != nil {
		errorf(t.logger, "failed to execute find_event_causals", "type", "FindEventCausalsTool", "func", "Execute", "error", err)
		return nil, err
	}

	result, err := t.service.FindEventCausals(ctx, in)
	if err != nil {
		err = toolerrs.From(err)
		errorf(t.logger, "failed to execute find_event_causals", "type", "FindEventCausalsTool", "func", "Execute", "error", err)
		return nil, err
	}

	raw, err := json.Marshal(result)
	if err != nil {
		return nil, toolerrs.NewInvalidRequest(err)
	}
	debug(t.logger, "find_event_causals is executed", "type", "FindEventCausalsTool", "func", "Execute", "count", len(result.Causals))
	return raw, nil
}

func (t *FindEventCausalsTool) unmarshal(args json.RawMessage) (serviceevents.FindEventCausalsInput, error) {
	var result serviceevents.FindEventCausalsInput
	if err := json.Unmarshal(args, &result); err != nil {
		return serviceevents.FindEventCausalsInput{}, toolerrs.NewInvalidRequest(err)
	}
	return result, nil
}
