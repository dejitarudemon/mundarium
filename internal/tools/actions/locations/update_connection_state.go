package locations

import (
	"encoding/json"

	"github.com/dejitarudemon/mundarium/internal/libs/execution"
	journal "github.com/dejitarudemon/mundarium/internal/libs/journals/execution"
	"github.com/dejitarudemon/mundarium/internal/libs/logger"
	"github.com/dejitarudemon/mundarium/internal/repositories"
	servicelocation "github.com/dejitarudemon/mundarium/internal/services/locations"
	"github.com/dejitarudemon/mundarium/internal/tools"
	"github.com/dejitarudemon/mundarium/internal/tools/actions"
	"github.com/dejitarudemon/mundarium/internal/tools/definition"
	toolerrs "github.com/dejitarudemon/mundarium/internal/tools/errs"
)

var (
	_ tools.Tool = (*UpdateConnectionStateTool)(nil)
)

// UpdateConnectionStateTool replaces a connection's state via the locations service.
type UpdateConnectionStateTool struct {
	service    *servicelocation.Service
	definition definition.Definition
	logger     logger.Logger
}

// UpdateConnectionStateArgs is the JSON payload accepted by update_connection_state.
type UpdateConnectionStateArgs struct {
	ID    repositories.ID   `json:"id"`
	State repositories.JSON `json:"state"`
}

// UpdateConnectionStateResult is the JSON payload returned by update_connection_state.
type UpdateConnectionStateResult struct {
	ID    repositories.ID   `json:"id"`
	State repositories.JSON `json:"state"`
}

// NewUpdateConnectionStateTool loads the tool definition from file and binds service.
// logger may be nil.
func NewUpdateConnectionStateTool(file string, service *servicelocation.Service, logger logger.Logger) (*UpdateConnectionStateTool, error) {
	if service == nil {
		return nil, toolerrs.ErrServiceIsNil
	}

	def, err := actions.LoadDefinition(file)
	if err != nil {
		return nil, err
	}

	return &UpdateConnectionStateTool{service: service, definition: def, logger: logger}, nil
}

// Definition returns a copy of the loaded tool schema.
func (t *UpdateConnectionStateTool) Definition() definition.Definition {
	return t.definition.Copy()
}

// IsIdempotent reports that update_connection_state is safe to retry without journaling.
func (t *UpdateConnectionStateTool) IsIdempotent() bool {
	return true
}

// Execute unmarshals args, replaces connection state, and returns JSON output.
func (t *UpdateConnectionStateTool) Execute(ctx execution.ExecutionContext[journal.Result], args json.RawMessage) (json.RawMessage, error) {
	in, err := t.unmarshal(args)
	if err != nil {
		errorf(t.logger, "failed to execute update_connection_state", "type", "UpdateConnectionStateTool", "func", "Execute", "error", err)
		return nil, err
	}

	if err := t.service.PatchConnectionState(ctx, in.ID, in.State); err != nil {
		err = toolerrs.From(err)
		errorf(t.logger, "failed to execute update_connection_state", "type", "UpdateConnectionStateTool", "func", "Execute", "id", in.ID, "error", err)
		return nil, err
	}

	raw, err := t.marshal(UpdateConnectionStateResult(in))
	if err != nil {
		errorf(t.logger, "failed to execute update_connection_state", "type", "UpdateConnectionStateTool", "func", "Execute", "id", in.ID, "error", err)
		return nil, err
	}

	info(t.logger, "update_connection_state is executed", "type", "UpdateConnectionStateTool", "func", "Execute", "id", in.ID)
	return raw, nil
}

func (t *UpdateConnectionStateTool) unmarshal(args json.RawMessage) (UpdateConnectionStateArgs, error) {
	var result UpdateConnectionStateArgs
	if err := json.Unmarshal(args, &result); err != nil {
		return UpdateConnectionStateArgs{}, toolerrs.NewInvalidRequest(err)
	}
	if result.ID == 0 {
		return UpdateConnectionStateArgs{}, toolerrs.NewInvalidRequest(toolerrs.ErrConnectionIDZero)
	}

	return result, nil
}

func (t *UpdateConnectionStateTool) marshal(output UpdateConnectionStateResult) (json.RawMessage, error) {
	raw, err := json.Marshal(output)
	if err != nil {
		return nil, toolerrs.NewInvalidRequest(err)
	}
	return raw, nil
}
