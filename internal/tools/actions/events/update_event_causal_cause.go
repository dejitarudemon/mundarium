package events

import (
	"encoding/json"

	"github.com/dejitarudemon/mundarium/internal/libs/execution"
	journal "github.com/dejitarudemon/mundarium/internal/libs/journals/execution"
	"github.com/dejitarudemon/mundarium/internal/libs/logger"
	repoevents "github.com/dejitarudemon/mundarium/internal/repositories/events"
	serviceevents "github.com/dejitarudemon/mundarium/internal/services/events"
	"github.com/dejitarudemon/mundarium/internal/tools"
	"github.com/dejitarudemon/mundarium/internal/tools/actions"
	"github.com/dejitarudemon/mundarium/internal/tools/definition"
	toolerrs "github.com/dejitarudemon/mundarium/internal/tools/errs"
)

var (
	_ tools.Tool = (*UpdateEventCausalCauseTool)(nil)
)

// UpdateEventCausalCauseTool updates causal edge text via the events service.
type UpdateEventCausalCauseTool struct {
	service    *serviceevents.Service
	definition definition.Definition
	logger     logger.Logger
}

// UpdateEventCausalCauseArgs is the JSON payload accepted by update_event_causal_cause.
type UpdateEventCausalCauseArgs struct {
	From  repoevents.ID `json:"from"`
	To    repoevents.ID `json:"to"`
	Cause string        `json:"cause"`
}

// UpdateEventCausalCauseResult is the JSON payload returned by update_event_causal_cause.
type UpdateEventCausalCauseResult struct {
	From  repoevents.ID `json:"from"`
	To    repoevents.ID `json:"to"`
	Cause string        `json:"cause"`
}

// NewUpdateEventCausalCauseTool loads the tool definition from file and binds service.
func NewUpdateEventCausalCauseTool(file string, service *serviceevents.Service, logger logger.Logger) (*UpdateEventCausalCauseTool, error) {
	if service == nil {
		return nil, toolerrs.ErrServiceIsNil
	}
	def, err := actions.LoadDefinition(file)
	if err != nil {
		return nil, err
	}
	return &UpdateEventCausalCauseTool{service: service, definition: def, logger: logger}, nil
}

func (t *UpdateEventCausalCauseTool) Definition() definition.Definition { return t.definition.Copy() }
func (t *UpdateEventCausalCauseTool) IsIdempotent() bool                { return true }

func (t *UpdateEventCausalCauseTool) Execute(ctx execution.ExecutionContext[journal.Result], args json.RawMessage) (json.RawMessage, error) {
	in, err := t.unmarshal(args)
	if err != nil {
		errorf(t.logger, "failed to execute update_event_causal_cause", "type", "UpdateEventCausalCauseTool", "func", "Execute", "error", err)
		return nil, err
	}

	if err := t.service.PatchEventCausalCause(ctx, in.From, in.To, in.Cause); err != nil {
		err = toolerrs.From(err)
		errorf(t.logger, "failed to execute update_event_causal_cause", "type", "UpdateEventCausalCauseTool", "func", "Execute", "error", err)
		return nil, err
	}

	raw, err := json.Marshal(UpdateEventCausalCauseResult(in))
	if err != nil {
		return nil, toolerrs.NewInvalidRequest(err)
	}
	info(t.logger, "update_event_causal_cause is executed", "type", "UpdateEventCausalCauseTool", "func", "Execute", "from", in.From, "to", in.To)
	return raw, nil
}

func (t *UpdateEventCausalCauseTool) unmarshal(args json.RawMessage) (UpdateEventCausalCauseArgs, error) {
	var result UpdateEventCausalCauseArgs
	if err := json.Unmarshal(args, &result); err != nil {
		return UpdateEventCausalCauseArgs{}, toolerrs.NewInvalidRequest(err)
	}
	if err := requireEventID(result.From); err != nil {
		return UpdateEventCausalCauseArgs{}, err
	}
	if err := requireEventID(result.To); err != nil {
		return UpdateEventCausalCauseArgs{}, err
	}
	return result, nil
}
