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
	_ tools.Tool = (*UpdateConnectionTool)(nil)
)

// UpdateConnectionTool fully replaces mutable fields of a location connection via the locations service.
type UpdateConnectionTool struct {
	service    *servicelocation.Service
	definition definition.Definition
	logger     logger.Logger
}

// UpdateConnectionResult is the JSON payload returned by update_connection.
type UpdateConnectionResult struct {
	ID repositories.ID `json:"id"`
}

// NewUpdateConnectionTool loads the tool definition from file and binds service.
// logger may be nil.
func NewUpdateConnectionTool(file string, service *servicelocation.Service, logger logger.Logger) (*UpdateConnectionTool, error) {
	if service == nil {
		return nil, toolerrs.ErrServiceIsNil
	}

	def, err := actions.LoadDefinition(file)
	if err != nil {
		return nil, err
	}

	return &UpdateConnectionTool{service: service, definition: def, logger: logger}, nil
}

// Definition returns a copy of the loaded tool schema.
func (t *UpdateConnectionTool) Definition() definition.Definition {
	return t.definition.Copy()
}

// IsIdempotent reports that update_connection is safe to retry without journaling.
func (t *UpdateConnectionTool) IsIdempotent() bool {
	return true
}

// Execute unmarshals update-connection args, calls the service, and returns JSON output.
func (t *UpdateConnectionTool) Execute(ctx execution.ExecutionContext[journal.Result], args json.RawMessage) (json.RawMessage, error) {
	in, err := t.unmarshal(args)
	if err != nil {
		errorf(t.logger, "failed to execute update_connection", "type", "UpdateConnectionTool", "func", "Execute", "error", err)
		return nil, err
	}

	if err := t.service.UpdateConnection(ctx, in); err != nil {
		err = toolerrs.From(err)
		errorf(t.logger, "failed to execute update_connection", "type", "UpdateConnectionTool", "func", "Execute", "id", in.ID, "error", err)
		return nil, err
	}

	raw, err := t.marshal(UpdateConnectionResult{ID: in.ID})
	if err != nil {
		errorf(t.logger, "failed to execute update_connection", "type", "UpdateConnectionTool", "func", "Execute", "id", in.ID, "error", err)
		return nil, err
	}

	info(t.logger, "update_connection is executed", "type", "UpdateConnectionTool", "func", "Execute", "id", in.ID)
	return raw, nil
}

func (t *UpdateConnectionTool) unmarshal(args json.RawMessage) (servicelocation.UpdateConnectionInput, error) {
	var result servicelocation.UpdateConnectionInput
	if err := json.Unmarshal(args, &result); err != nil {
		return servicelocation.UpdateConnectionInput{}, toolerrs.NewInvalidRequest(err)
	}
	if result.ID == 0 {
		return servicelocation.UpdateConnectionInput{}, toolerrs.NewInvalidRequest(toolerrs.ErrConnectionIDZero)
	}

	return result, nil
}

func (t *UpdateConnectionTool) marshal(output UpdateConnectionResult) (json.RawMessage, error) {
	raw, err := json.Marshal(output)
	if err != nil {
		return nil, toolerrs.NewInvalidRequest(err)
	}
	return raw, nil
}
