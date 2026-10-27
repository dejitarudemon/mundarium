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
	_ tools.Tool = (*UpdateConnectionDetailsTool)(nil)
)

// UpdateConnectionDetailsTool replaces a connection's details via the locations service.
type UpdateConnectionDetailsTool struct {
	service    *servicelocation.Service
	definition definition.Definition
	logger     logger.Logger
}

// UpdateConnectionDetailsArgs is the JSON payload accepted by update_connection_details.
type UpdateConnectionDetailsArgs struct {
	ID      repositories.ID   `json:"id"`
	Details repositories.JSON `json:"details"`
}

// UpdateConnectionDetailsResult is the JSON payload returned by update_connection_details.
type UpdateConnectionDetailsResult struct {
	ID      repositories.ID   `json:"id"`
	Details repositories.JSON `json:"details"`
}

// NewUpdateConnectionDetailsTool loads the tool definition from file and binds service.
// logger may be nil.
func NewUpdateConnectionDetailsTool(file string, service *servicelocation.Service, logger logger.Logger) (*UpdateConnectionDetailsTool, error) {
	if service == nil {
		return nil, toolerrs.ErrServiceIsNil
	}

	def, err := actions.LoadDefinition(file)
	if err != nil {
		return nil, err
	}

	return &UpdateConnectionDetailsTool{service: service, definition: def, logger: logger}, nil
}

// Definition returns a copy of the loaded tool schema.
func (t *UpdateConnectionDetailsTool) Definition() definition.Definition {
	return t.definition.Copy()
}

// IsIdempotent reports that update_connection_details is safe to retry without journaling.
func (t *UpdateConnectionDetailsTool) IsIdempotent() bool {
	return true
}

// Execute unmarshals args, replaces connection details, and returns JSON output.
func (t *UpdateConnectionDetailsTool) Execute(ctx execution.ExecutionContext[journal.Result], args json.RawMessage) (json.RawMessage, error) {
	in, err := t.unmarshal(args)
	if err != nil {
		errorf(t.logger, "failed to execute update_connection_details", "type", "UpdateConnectionDetailsTool", "func", "Execute", "error", err)
		return nil, err
	}

	if err := t.service.PatchConnectionDetails(ctx, in.ID, in.Details); err != nil {
		err = toolerrs.From(err)
		errorf(t.logger, "failed to execute update_connection_details", "type", "UpdateConnectionDetailsTool", "func", "Execute", "id", in.ID, "error", err)
		return nil, err
	}

	raw, err := t.marshal(UpdateConnectionDetailsResult{ID: in.ID, Details: in.Details})
	if err != nil {
		errorf(t.logger, "failed to execute update_connection_details", "type", "UpdateConnectionDetailsTool", "func", "Execute", "id", in.ID, "error", err)
		return nil, err
	}

	info(t.logger, "update_connection_details is executed", "type", "UpdateConnectionDetailsTool", "func", "Execute", "id", in.ID)
	return raw, nil
}

func (t *UpdateConnectionDetailsTool) unmarshal(args json.RawMessage) (UpdateConnectionDetailsArgs, error) {
	var result UpdateConnectionDetailsArgs
	if err := json.Unmarshal(args, &result); err != nil {
		return UpdateConnectionDetailsArgs{}, toolerrs.NewInvalidRequest(err)
	}
	if result.ID == 0 {
		return UpdateConnectionDetailsArgs{}, toolerrs.NewInvalidRequest(toolerrs.ErrConnectionIDZero)
	}

	return result, nil
}

func (t *UpdateConnectionDetailsTool) marshal(output UpdateConnectionDetailsResult) (json.RawMessage, error) {
	raw, err := json.Marshal(output)
	if err != nil {
		return nil, toolerrs.NewInvalidRequest(err)
	}
	return raw, nil
}
