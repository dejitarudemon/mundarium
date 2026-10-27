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
	_ tools.Tool = (*UpdateConnectionToTool)(nil)
)

// UpdateConnectionToTool changes a connection's destination via the locations service.
type UpdateConnectionToTool struct {
	service    *servicelocation.Service
	definition definition.Definition
	logger     logger.Logger
}

// UpdateConnectionToArgs is the JSON payload accepted by update_connection_to.
type UpdateConnectionToArgs struct {
	ID   repositories.ID `json:"id"`
	ToID repositories.ID `json:"to_id"`
}

// UpdateConnectionToResult is the JSON payload returned by update_connection_to.
type UpdateConnectionToResult struct {
	ID   repositories.ID `json:"id"`
	ToID repositories.ID `json:"to_id"`
}

// NewUpdateConnectionToTool loads the tool definition from file and binds service.
// logger may be nil.
func NewUpdateConnectionToTool(file string, service *servicelocation.Service, logger logger.Logger) (*UpdateConnectionToTool, error) {
	if service == nil {
		return nil, toolerrs.ErrServiceIsNil
	}

	def, err := actions.LoadDefinition(file)
	if err != nil {
		return nil, err
	}

	return &UpdateConnectionToTool{service: service, definition: def, logger: logger}, nil
}

// Definition returns a copy of the loaded tool schema.
func (t *UpdateConnectionToTool) Definition() definition.Definition {
	return t.definition.Copy()
}

// IsIdempotent reports that update_connection_to is safe to retry without journaling.
func (t *UpdateConnectionToTool) IsIdempotent() bool {
	return true
}

// Execute unmarshals args, patches destination, and returns JSON output.
func (t *UpdateConnectionToTool) Execute(ctx execution.ExecutionContext[journal.Result], args json.RawMessage) (json.RawMessage, error) {
	in, err := t.unmarshal(args)
	if err != nil {
		errorf(t.logger, "failed to execute update_connection_to", "type", "UpdateConnectionToTool", "func", "Execute", "error", err)
		return nil, err
	}

	if err := t.service.PatchConnectionTo(ctx, in.ID, in.ToID); err != nil {
		err = toolerrs.From(err)
		errorf(t.logger, "failed to execute update_connection_to", "type", "UpdateConnectionToTool", "func", "Execute", "id", in.ID, "error", err)
		return nil, err
	}

	raw, err := t.marshal(UpdateConnectionToResult(in))
	if err != nil {
		errorf(t.logger, "failed to execute update_connection_to", "type", "UpdateConnectionToTool", "func", "Execute", "id", in.ID, "error", err)
		return nil, err
	}

	info(t.logger, "update_connection_to is executed", "type", "UpdateConnectionToTool", "func", "Execute", "id", in.ID, "to_id", in.ToID)
	return raw, nil
}

func (t *UpdateConnectionToTool) unmarshal(args json.RawMessage) (UpdateConnectionToArgs, error) {
	var result UpdateConnectionToArgs
	if err := json.Unmarshal(args, &result); err != nil {
		return UpdateConnectionToArgs{}, toolerrs.NewInvalidRequest(err)
	}
	if result.ID == 0 {
		return UpdateConnectionToArgs{}, toolerrs.NewInvalidRequest(toolerrs.ErrConnectionIDZero)
	}
	if result.ToID == 0 {
		return UpdateConnectionToArgs{}, toolerrs.NewInvalidRequest(toolerrs.ErrToIDZero)
	}

	return result, nil
}

func (t *UpdateConnectionToTool) marshal(output UpdateConnectionToResult) (json.RawMessage, error) {
	raw, err := json.Marshal(output)
	if err != nil {
		return nil, toolerrs.NewInvalidRequest(err)
	}
	return raw, nil
}
