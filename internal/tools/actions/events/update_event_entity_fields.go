package events

import (
	"encoding/json"

	"github.com/dejitarudemon/mundarium/internal/libs/execution"
	journal "github.com/dejitarudemon/mundarium/internal/libs/journals/execution"
	"github.com/dejitarudemon/mundarium/internal/libs/logger"
	"github.com/dejitarudemon/mundarium/internal/repositories"
	"github.com/dejitarudemon/mundarium/internal/repositories/entities"
	repoevents "github.com/dejitarudemon/mundarium/internal/repositories/events"
	serviceevents "github.com/dejitarudemon/mundarium/internal/services/events"
	"github.com/dejitarudemon/mundarium/internal/tools"
	"github.com/dejitarudemon/mundarium/internal/tools/actions"
	"github.com/dejitarudemon/mundarium/internal/tools/definition"
	toolerrs "github.com/dejitarudemon/mundarium/internal/tools/errs"
)

var (
	_ tools.Tool = (*UpdateEventEntityRoleTool)(nil)
	_ tools.Tool = (*UpdateEventEntityKnownAtTool)(nil)
	_ tools.Tool = (*UpdateEventEntityBeliefTool)(nil)
	_ tools.Tool = (*UpdateEventEntityDetailsTool)(nil)
)

func parseEventEntityKey(entityID string, eventID repoevents.ID) (entities.ID, repoevents.ID, error) {
	eid, err := parseEntityID(entityID)
	if err != nil {
		return entities.ID{}, repoevents.ID{}, err
	}
	if err := requireEventID(eventID); err != nil {
		return entities.ID{}, repoevents.ID{}, err
	}
	return eid, eventID, nil
}

// --- update_event_entity_role ---

// UpdateEventEntityRoleTool changes a participant role via the events service.
type UpdateEventEntityRoleTool struct {
	service    *serviceevents.Service
	definition definition.Definition
	logger     logger.Logger
}

// UpdateEventEntityRoleArgs is the JSON payload accepted by update_event_entity_role.
type UpdateEventEntityRoleArgs struct {
	EntityID string        `json:"entity_id"`
	EventID  repoevents.ID `json:"event_id"`
	Role     string        `json:"role"`
}

// UpdateEventEntityRoleResult is the JSON payload returned by update_event_entity_role.
type UpdateEventEntityRoleResult struct {
	EntityID string        `json:"entity_id"`
	EventID  repoevents.ID `json:"event_id"`
	Role     string        `json:"role"`
}

// NewUpdateEventEntityRoleTool loads the tool definition from file and binds service.
func NewUpdateEventEntityRoleTool(file string, service *serviceevents.Service, logger logger.Logger) (*UpdateEventEntityRoleTool, error) {
	if service == nil {
		return nil, toolerrs.ErrServiceIsNil
	}
	def, err := actions.LoadDefinition(file)
	if err != nil {
		return nil, err
	}
	return &UpdateEventEntityRoleTool{service: service, definition: def, logger: logger}, nil
}

func (t *UpdateEventEntityRoleTool) Definition() definition.Definition { return t.definition.Copy() }
func (t *UpdateEventEntityRoleTool) IsIdempotent() bool                { return true }

func (t *UpdateEventEntityRoleTool) Execute(ctx execution.ExecutionContext[journal.Result], args json.RawMessage) (json.RawMessage, error) {
	var in UpdateEventEntityRoleArgs
	if err := json.Unmarshal(args, &in); err != nil {
		err = toolerrs.NewInvalidRequest(err)
		errorf(t.logger, "failed to execute update_event_entity_role", "type", "UpdateEventEntityRoleTool", "func", "Execute", "error", err)
		return nil, err
	}
	eid, eventID, err := parseEventEntityKey(in.EntityID, in.EventID)
	if err != nil {
		errorf(t.logger, "failed to execute update_event_entity_role", "type", "UpdateEventEntityRoleTool", "func", "Execute", "error", err)
		return nil, err
	}
	if err := t.service.PatchEventEntityRole(ctx, eid, eventID, in.Role); err != nil {
		err = toolerrs.From(err)
		errorf(t.logger, "failed to execute update_event_entity_role", "type", "UpdateEventEntityRoleTool", "func", "Execute", "error", err)
		return nil, err
	}
	raw, err := json.Marshal(UpdateEventEntityRoleResult(in))
	if err != nil {
		return nil, toolerrs.NewInvalidRequest(err)
	}
	info(t.logger, "update_event_entity_role is executed", "type", "UpdateEventEntityRoleTool", "func", "Execute", "entity_id", in.EntityID, "event_id", in.EventID)
	return raw, nil
}

// --- update_event_entity_known_at ---

// UpdateEventEntityKnownAtTool establishes knowledge via the events service.
type UpdateEventEntityKnownAtTool struct {
	service    *serviceevents.Service
	definition definition.Definition
	logger     logger.Logger
}

// UpdateEventEntityKnownAtArgs is the JSON payload accepted by update_event_entity_known_at.
type UpdateEventEntityKnownAtArgs struct {
	EntityID        string             `json:"entity_id"`
	EventID         repoevents.ID      `json:"event_id"`
	KnownAtGameTime repositories.Ticks `json:"known_at_game_time"`
	Belief          int8               `json:"belief"`
}

// UpdateEventEntityKnownAtResult is the JSON payload returned by update_event_entity_known_at.
type UpdateEventEntityKnownAtResult struct {
	EntityID        string             `json:"entity_id"`
	EventID         repoevents.ID      `json:"event_id"`
	KnownAtGameTime repositories.Ticks `json:"known_at_game_time"`
	Belief          int8               `json:"belief"`
}

// NewUpdateEventEntityKnownAtTool loads the tool definition from file and binds service.
func NewUpdateEventEntityKnownAtTool(file string, service *serviceevents.Service, logger logger.Logger) (*UpdateEventEntityKnownAtTool, error) {
	if service == nil {
		return nil, toolerrs.ErrServiceIsNil
	}
	def, err := actions.LoadDefinition(file)
	if err != nil {
		return nil, err
	}
	return &UpdateEventEntityKnownAtTool{service: service, definition: def, logger: logger}, nil
}

func (t *UpdateEventEntityKnownAtTool) Definition() definition.Definition { return t.definition.Copy() }
func (t *UpdateEventEntityKnownAtTool) IsIdempotent() bool                { return false }

func (t *UpdateEventEntityKnownAtTool) Execute(ctx execution.ExecutionContext[journal.Result], args json.RawMessage) (json.RawMessage, error) {
	var in UpdateEventEntityKnownAtArgs
	if err := json.Unmarshal(args, &in); err != nil {
		err = toolerrs.NewInvalidRequest(err)
		errorf(t.logger, "failed to execute update_event_entity_known_at", "type", "UpdateEventEntityKnownAtTool", "func", "Execute", "error", err)
		return nil, err
	}
	eid, eventID, err := parseEventEntityKey(in.EntityID, in.EventID)
	if err != nil {
		errorf(t.logger, "failed to execute update_event_entity_known_at", "type", "UpdateEventEntityKnownAtTool", "func", "Execute", "error", err)
		return nil, err
	}
	if err := t.service.PatchEventEntityKnownAtGameTime(ctx, eid, eventID, in.KnownAtGameTime, in.Belief); err != nil {
		err = toolerrs.From(err)
		errorf(t.logger, "failed to execute update_event_entity_known_at", "type", "UpdateEventEntityKnownAtTool", "func", "Execute", "error", err)
		return nil, err
	}
	raw, err := json.Marshal(UpdateEventEntityKnownAtResult(in))
	if err != nil {
		return nil, toolerrs.NewInvalidRequest(err)
	}
	info(t.logger, "update_event_entity_known_at is executed", "type", "UpdateEventEntityKnownAtTool", "func", "Execute", "entity_id", in.EntityID, "event_id", in.EventID)
	return raw, nil
}

// --- update_event_entity_belief ---

// UpdateEventEntityBeliefTool updates belief via the events service.
type UpdateEventEntityBeliefTool struct {
	service    *serviceevents.Service
	definition definition.Definition
	logger     logger.Logger
}

// UpdateEventEntityBeliefArgs is the JSON payload accepted by update_event_entity_belief.
type UpdateEventEntityBeliefArgs struct {
	EntityID string        `json:"entity_id"`
	EventID  repoevents.ID `json:"event_id"`
	Belief   int8          `json:"belief"`
}

// UpdateEventEntityBeliefResult is the JSON payload returned by update_event_entity_belief.
type UpdateEventEntityBeliefResult struct {
	EntityID string        `json:"entity_id"`
	EventID  repoevents.ID `json:"event_id"`
	Belief   int8          `json:"belief"`
}

// NewUpdateEventEntityBeliefTool loads the tool definition from file and binds service.
func NewUpdateEventEntityBeliefTool(file string, service *serviceevents.Service, logger logger.Logger) (*UpdateEventEntityBeliefTool, error) {
	if service == nil {
		return nil, toolerrs.ErrServiceIsNil
	}
	def, err := actions.LoadDefinition(file)
	if err != nil {
		return nil, err
	}
	return &UpdateEventEntityBeliefTool{service: service, definition: def, logger: logger}, nil
}

func (t *UpdateEventEntityBeliefTool) Definition() definition.Definition { return t.definition.Copy() }
func (t *UpdateEventEntityBeliefTool) IsIdempotent() bool                { return true }

func (t *UpdateEventEntityBeliefTool) Execute(ctx execution.ExecutionContext[journal.Result], args json.RawMessage) (json.RawMessage, error) {
	var in UpdateEventEntityBeliefArgs
	if err := json.Unmarshal(args, &in); err != nil {
		err = toolerrs.NewInvalidRequest(err)
		errorf(t.logger, "failed to execute update_event_entity_belief", "type", "UpdateEventEntityBeliefTool", "func", "Execute", "error", err)
		return nil, err
	}
	eid, eventID, err := parseEventEntityKey(in.EntityID, in.EventID)
	if err != nil {
		errorf(t.logger, "failed to execute update_event_entity_belief", "type", "UpdateEventEntityBeliefTool", "func", "Execute", "error", err)
		return nil, err
	}
	if err := t.service.PatchEventEntityBelief(ctx, eid, eventID, in.Belief); err != nil {
		err = toolerrs.From(err)
		errorf(t.logger, "failed to execute update_event_entity_belief", "type", "UpdateEventEntityBeliefTool", "func", "Execute", "error", err)
		return nil, err
	}
	raw, err := json.Marshal(UpdateEventEntityBeliefResult(in))
	if err != nil {
		return nil, toolerrs.NewInvalidRequest(err)
	}
	info(t.logger, "update_event_entity_belief is executed", "type", "UpdateEventEntityBeliefTool", "func", "Execute", "entity_id", in.EntityID, "event_id", in.EventID)
	return raw, nil
}

// --- update_event_entity_details ---

// UpdateEventEntityDetailsTool replaces participant details via the events service.
type UpdateEventEntityDetailsTool struct {
	service    *serviceevents.Service
	definition definition.Definition
	logger     logger.Logger
}

// UpdateEventEntityDetailsArgs is the JSON payload accepted by update_event_entity_details.
type UpdateEventEntityDetailsArgs struct {
	EntityID string            `json:"entity_id"`
	EventID  repoevents.ID     `json:"event_id"`
	Details  repositories.JSON `json:"details"`
}

// UpdateEventEntityDetailsResult is the JSON payload returned by update_event_entity_details.
type UpdateEventEntityDetailsResult struct {
	EntityID string            `json:"entity_id"`
	EventID  repoevents.ID     `json:"event_id"`
	Details  repositories.JSON `json:"details"`
}

// NewUpdateEventEntityDetailsTool loads the tool definition from file and binds service.
func NewUpdateEventEntityDetailsTool(file string, service *serviceevents.Service, logger logger.Logger) (*UpdateEventEntityDetailsTool, error) {
	if service == nil {
		return nil, toolerrs.ErrServiceIsNil
	}
	def, err := actions.LoadDefinition(file)
	if err != nil {
		return nil, err
	}
	return &UpdateEventEntityDetailsTool{service: service, definition: def, logger: logger}, nil
}

func (t *UpdateEventEntityDetailsTool) Definition() definition.Definition { return t.definition.Copy() }
func (t *UpdateEventEntityDetailsTool) IsIdempotent() bool                { return true }

func (t *UpdateEventEntityDetailsTool) Execute(ctx execution.ExecutionContext[journal.Result], args json.RawMessage) (json.RawMessage, error) {
	var in UpdateEventEntityDetailsArgs
	if err := json.Unmarshal(args, &in); err != nil {
		err = toolerrs.NewInvalidRequest(err)
		errorf(t.logger, "failed to execute update_event_entity_details", "type", "UpdateEventEntityDetailsTool", "func", "Execute", "error", err)
		return nil, err
	}
	eid, eventID, err := parseEventEntityKey(in.EntityID, in.EventID)
	if err != nil {
		errorf(t.logger, "failed to execute update_event_entity_details", "type", "UpdateEventEntityDetailsTool", "func", "Execute", "error", err)
		return nil, err
	}
	if err := t.service.PatchEventEntityDetails(ctx, eid, eventID, in.Details); err != nil {
		err = toolerrs.From(err)
		errorf(t.logger, "failed to execute update_event_entity_details", "type", "UpdateEventEntityDetailsTool", "func", "Execute", "error", err)
		return nil, err
	}
	raw, err := json.Marshal(UpdateEventEntityDetailsResult(in))
	if err != nil {
		return nil, toolerrs.NewInvalidRequest(err)
	}
	info(t.logger, "update_event_entity_details is executed", "type", "UpdateEventEntityDetailsTool", "func", "Execute", "entity_id", in.EntityID, "event_id", in.EventID)
	return raw, nil
}
