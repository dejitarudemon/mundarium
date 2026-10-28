package events

import (
	"encoding/json"

	"github.com/dejitarudemon/mundarium/internal/libs/execution"
	journal "github.com/dejitarudemon/mundarium/internal/libs/journals/execution"
	"github.com/dejitarudemon/mundarium/internal/libs/logger"
	"github.com/dejitarudemon/mundarium/internal/repositories"
	repoevents "github.com/dejitarudemon/mundarium/internal/repositories/events"
	serviceevents "github.com/dejitarudemon/mundarium/internal/services/events"
	"github.com/dejitarudemon/mundarium/internal/tools"
	"github.com/dejitarudemon/mundarium/internal/tools/actions"
	"github.com/dejitarudemon/mundarium/internal/tools/definition"
	toolerrs "github.com/dejitarudemon/mundarium/internal/tools/errs"
)

var (
	_ tools.Tool = (*UpdateEventLocationTool)(nil)
	_ tools.Tool = (*UpdateEventImportanceTool)(nil)
	_ tools.Tool = (*UpdateEventSummaryTool)(nil)
	_ tools.Tool = (*UpdateEventDetailsTool)(nil)
)

// --- update_event_location ---

// UpdateEventLocationTool sets an event's location via the events service.
type UpdateEventLocationTool struct {
	service    *serviceevents.Service
	definition definition.Definition
	logger     logger.Logger
}

// UpdateEventLocationArgs is the JSON payload accepted by update_event_location.
type UpdateEventLocationArgs struct {
	ID         repoevents.ID   `json:"id"`
	LocationID repositories.ID `json:"location_id"`
}

// UpdateEventLocationResult is the JSON payload returned by update_event_location.
type UpdateEventLocationResult struct {
	ID         repoevents.ID   `json:"id"`
	LocationID repositories.ID `json:"location_id"`
}

// NewUpdateEventLocationTool loads the tool definition from file and binds service.
func NewUpdateEventLocationTool(file string, service *serviceevents.Service, logger logger.Logger) (*UpdateEventLocationTool, error) {
	if service == nil {
		return nil, toolerrs.ErrServiceIsNil
	}
	def, err := actions.LoadDefinition(file)
	if err != nil {
		return nil, err
	}
	return &UpdateEventLocationTool{service: service, definition: def, logger: logger}, nil
}

func (t *UpdateEventLocationTool) Definition() definition.Definition { return t.definition.Copy() }
func (t *UpdateEventLocationTool) IsIdempotent() bool                { return true }

func (t *UpdateEventLocationTool) Execute(ctx execution.ExecutionContext[journal.Result], args json.RawMessage) (json.RawMessage, error) {
	in, err := unmarshalEventLocationArgs(args)
	if err != nil {
		errorf(t.logger, "failed to execute update_event_location", "type", "UpdateEventLocationTool", "func", "Execute", "error", err)
		return nil, err
	}
	if err := t.service.PatchEventLocation(ctx, in.ID, in.LocationID); err != nil {
		err = toolerrs.From(err)
		errorf(t.logger, "failed to execute update_event_location", "type", "UpdateEventLocationTool", "func", "Execute", "id", in.ID, "error", err)
		return nil, err
	}
	raw, err := json.Marshal(UpdateEventLocationResult(in))
	if err != nil {
		return nil, toolerrs.NewInvalidRequest(err)
	}
	info(t.logger, "update_event_location is executed", "type", "UpdateEventLocationTool", "func", "Execute", "id", in.ID)
	return raw, nil
}

func unmarshalEventLocationArgs(args json.RawMessage) (UpdateEventLocationArgs, error) {
	var result UpdateEventLocationArgs
	if err := json.Unmarshal(args, &result); err != nil {
		return UpdateEventLocationArgs{}, toolerrs.NewInvalidRequest(err)
	}
	if err := requireEventID(result.ID); err != nil {
		return UpdateEventLocationArgs{}, err
	}
	if result.LocationID == 0 {
		return UpdateEventLocationArgs{}, toolerrs.NewInvalidRequest(toolerrs.ErrLocationIDZero)
	}
	return result, nil
}

// --- update_event_importance ---

// UpdateEventImportanceTool sets an event's importance via the events service.
type UpdateEventImportanceTool struct {
	service    *serviceevents.Service
	definition definition.Definition
	logger     logger.Logger
}

// UpdateEventImportanceArgs is the JSON payload accepted by update_event_importance.
type UpdateEventImportanceArgs struct {
	ID         repoevents.ID `json:"id"`
	Importance int8          `json:"importance"`
}

// UpdateEventImportanceResult is the JSON payload returned by update_event_importance.
type UpdateEventImportanceResult struct {
	ID         repoevents.ID `json:"id"`
	Importance int8          `json:"importance"`
}

// NewUpdateEventImportanceTool loads the tool definition from file and binds service.
func NewUpdateEventImportanceTool(file string, service *serviceevents.Service, logger logger.Logger) (*UpdateEventImportanceTool, error) {
	if service == nil {
		return nil, toolerrs.ErrServiceIsNil
	}
	def, err := actions.LoadDefinition(file)
	if err != nil {
		return nil, err
	}
	return &UpdateEventImportanceTool{service: service, definition: def, logger: logger}, nil
}

func (t *UpdateEventImportanceTool) Definition() definition.Definition { return t.definition.Copy() }
func (t *UpdateEventImportanceTool) IsIdempotent() bool                { return true }

func (t *UpdateEventImportanceTool) Execute(ctx execution.ExecutionContext[journal.Result], args json.RawMessage) (json.RawMessage, error) {
	in, err := unmarshalEventImportanceArgs(args)
	if err != nil {
		errorf(t.logger, "failed to execute update_event_importance", "type", "UpdateEventImportanceTool", "func", "Execute", "error", err)
		return nil, err
	}
	if err := t.service.PatchEventImportance(ctx, in.ID, in.Importance); err != nil {
		err = toolerrs.From(err)
		errorf(t.logger, "failed to execute update_event_importance", "type", "UpdateEventImportanceTool", "func", "Execute", "id", in.ID, "error", err)
		return nil, err
	}
	raw, err := json.Marshal(UpdateEventImportanceResult(in))
	if err != nil {
		return nil, toolerrs.NewInvalidRequest(err)
	}
	info(t.logger, "update_event_importance is executed", "type", "UpdateEventImportanceTool", "func", "Execute", "id", in.ID)
	return raw, nil
}

func unmarshalEventImportanceArgs(args json.RawMessage) (UpdateEventImportanceArgs, error) {
	var result UpdateEventImportanceArgs
	if err := json.Unmarshal(args, &result); err != nil {
		return UpdateEventImportanceArgs{}, toolerrs.NewInvalidRequest(err)
	}
	if err := requireEventID(result.ID); err != nil {
		return UpdateEventImportanceArgs{}, err
	}
	return result, nil
}

// --- update_event_summary ---

// UpdateEventSummaryTool sets an event's summary via the events service.
type UpdateEventSummaryTool struct {
	service    *serviceevents.Service
	definition definition.Definition
	logger     logger.Logger
}

// UpdateEventSummaryArgs is the JSON payload accepted by update_event_summary.
type UpdateEventSummaryArgs struct {
	ID      repoevents.ID `json:"id"`
	Summary string        `json:"summary"`
}

// UpdateEventSummaryResult is the JSON payload returned by update_event_summary.
type UpdateEventSummaryResult struct {
	ID      repoevents.ID `json:"id"`
	Summary string        `json:"summary"`
}

// NewUpdateEventSummaryTool loads the tool definition from file and binds service.
func NewUpdateEventSummaryTool(file string, service *serviceevents.Service, logger logger.Logger) (*UpdateEventSummaryTool, error) {
	if service == nil {
		return nil, toolerrs.ErrServiceIsNil
	}
	def, err := actions.LoadDefinition(file)
	if err != nil {
		return nil, err
	}
	return &UpdateEventSummaryTool{service: service, definition: def, logger: logger}, nil
}

func (t *UpdateEventSummaryTool) Definition() definition.Definition { return t.definition.Copy() }
func (t *UpdateEventSummaryTool) IsIdempotent() bool                { return true }

func (t *UpdateEventSummaryTool) Execute(ctx execution.ExecutionContext[journal.Result], args json.RawMessage) (json.RawMessage, error) {
	in, err := unmarshalEventSummaryArgs(args)
	if err != nil {
		errorf(t.logger, "failed to execute update_event_summary", "type", "UpdateEventSummaryTool", "func", "Execute", "error", err)
		return nil, err
	}
	if err := t.service.PatchEventSummary(ctx, in.ID, in.Summary); err != nil {
		err = toolerrs.From(err)
		errorf(t.logger, "failed to execute update_event_summary", "type", "UpdateEventSummaryTool", "func", "Execute", "id", in.ID, "error", err)
		return nil, err
	}
	raw, err := json.Marshal(UpdateEventSummaryResult(in))
	if err != nil {
		return nil, toolerrs.NewInvalidRequest(err)
	}
	info(t.logger, "update_event_summary is executed", "type", "UpdateEventSummaryTool", "func", "Execute", "id", in.ID)
	return raw, nil
}

func unmarshalEventSummaryArgs(args json.RawMessage) (UpdateEventSummaryArgs, error) {
	var result UpdateEventSummaryArgs
	if err := json.Unmarshal(args, &result); err != nil {
		return UpdateEventSummaryArgs{}, toolerrs.NewInvalidRequest(err)
	}
	if err := requireEventID(result.ID); err != nil {
		return UpdateEventSummaryArgs{}, err
	}
	return result, nil
}

// --- update_event_details ---

// UpdateEventDetailsTool replaces an event's details via the events service.
type UpdateEventDetailsTool struct {
	service    *serviceevents.Service
	definition definition.Definition
	logger     logger.Logger
}

// UpdateEventDetailsArgs is the JSON payload accepted by update_event_details.
type UpdateEventDetailsArgs struct {
	ID      repoevents.ID     `json:"id"`
	Details repositories.JSON `json:"details"`
}

// UpdateEventDetailsResult is the JSON payload returned by update_event_details.
type UpdateEventDetailsResult struct {
	ID      repoevents.ID     `json:"id"`
	Details repositories.JSON `json:"details"`
}

// NewUpdateEventDetailsTool loads the tool definition from file and binds service.
func NewUpdateEventDetailsTool(file string, service *serviceevents.Service, logger logger.Logger) (*UpdateEventDetailsTool, error) {
	if service == nil {
		return nil, toolerrs.ErrServiceIsNil
	}
	def, err := actions.LoadDefinition(file)
	if err != nil {
		return nil, err
	}
	return &UpdateEventDetailsTool{service: service, definition: def, logger: logger}, nil
}

func (t *UpdateEventDetailsTool) Definition() definition.Definition { return t.definition.Copy() }
func (t *UpdateEventDetailsTool) IsIdempotent() bool                { return true }

func (t *UpdateEventDetailsTool) Execute(ctx execution.ExecutionContext[journal.Result], args json.RawMessage) (json.RawMessage, error) {
	in, err := unmarshalEventDetailsArgs(args)
	if err != nil {
		errorf(t.logger, "failed to execute update_event_details", "type", "UpdateEventDetailsTool", "func", "Execute", "error", err)
		return nil, err
	}
	if err := t.service.PatchEventDetails(ctx, in.ID, in.Details); err != nil {
		err = toolerrs.From(err)
		errorf(t.logger, "failed to execute update_event_details", "type", "UpdateEventDetailsTool", "func", "Execute", "id", in.ID, "error", err)
		return nil, err
	}
	raw, err := json.Marshal(UpdateEventDetailsResult(in))
	if err != nil {
		return nil, toolerrs.NewInvalidRequest(err)
	}
	info(t.logger, "update_event_details is executed", "type", "UpdateEventDetailsTool", "func", "Execute", "id", in.ID)
	return raw, nil
}

func unmarshalEventDetailsArgs(args json.RawMessage) (UpdateEventDetailsArgs, error) {
	var result UpdateEventDetailsArgs
	if err := json.Unmarshal(args, &result); err != nil {
		return UpdateEventDetailsArgs{}, toolerrs.NewInvalidRequest(err)
	}
	if err := requireEventID(result.ID); err != nil {
		return UpdateEventDetailsArgs{}, err
	}
	return result, nil
}
