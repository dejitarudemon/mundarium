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
	_ tools.Tool = (*FindEventsTool)(nil)
)

// FindEventsTool searches events via the events service.
type FindEventsTool struct {
	service    *serviceevents.Service
	definition definition.Definition
	logger     logger.Logger
}

// NewFindEventsTool loads the tool definition from file and binds service.
// logger may be nil.
func NewFindEventsTool(file string, service *serviceevents.Service, logger logger.Logger) (*FindEventsTool, error) {
	if service == nil {
		return nil, toolerrs.ErrServiceIsNil
	}

	def, err := actions.LoadDefinition(file)
	if err != nil {
		return nil, err
	}

	return &FindEventsTool{service: service, definition: def, logger: logger}, nil
}

// Definition returns a copy of the loaded tool schema.
func (t *FindEventsTool) Definition() definition.Definition {
	return t.definition.Copy()
}

// IsIdempotent reports that find_events is safe to retry without journaling.
func (t *FindEventsTool) IsIdempotent() bool {
	return true
}

// Execute unmarshals find-events args, calls the service, and returns JSON output.
func (t *FindEventsTool) Execute(ctx execution.ExecutionContext[journal.Result], args json.RawMessage) (json.RawMessage, error) {
	in, err := t.unmarshal(args)
	if err != nil {
		errorf(t.logger, "failed to execute find_events", "type", "FindEventsTool", "func", "Execute", "error", err)
		return nil, err
	}

	result, err := t.service.FindEvents(ctx, in)
	if err != nil {
		err = toolerrs.From(err)
		errorf(t.logger, "failed to execute find_events", "type", "FindEventsTool", "func", "Execute", "error", err)
		return nil, err
	}

	raw, err := t.marshal(result)
	if err != nil {
		errorf(t.logger, "failed to execute find_events", "type", "FindEventsTool", "func", "Execute", "error", err)
		return nil, err
	}

	debug(t.logger, "find_events is executed", "type", "FindEventsTool", "func", "Execute", "count", len(result.Events))
	return raw, nil
}

func (t *FindEventsTool) unmarshal(args json.RawMessage) (serviceevents.FindEventsInput, error) {
	var result serviceevents.FindEventsInput
	if err := json.Unmarshal(args, &result); err != nil {
		return serviceevents.FindEventsInput{}, toolerrs.NewInvalidRequest(err)
	}
	return result, nil
}

func (t *FindEventsTool) marshal(output serviceevents.FindEventsOutput) (json.RawMessage, error) {
	raw, err := json.Marshal(output)
	if err != nil {
		return nil, toolerrs.NewInvalidRequest(err)
	}
	return raw, nil
}
