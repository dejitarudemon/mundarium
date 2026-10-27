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
	_ tools.Tool = (*UpdateConnectionDescriptionTool)(nil)
)

// UpdateConnectionDescriptionTool replaces a connection's description via the locations service.
type UpdateConnectionDescriptionTool struct {
	service    *servicelocation.Service
	definition definition.Definition
	logger     logger.Logger
}

// UpdateConnectionDescriptionArgs is the JSON payload accepted by update_connection_description.
type UpdateConnectionDescriptionArgs struct {
	ID          repositories.ID `json:"id"`
	Description string          `json:"description"`
}

// UpdateConnectionDescriptionResult is the JSON payload returned by update_connection_description.
type UpdateConnectionDescriptionResult struct {
	ID          repositories.ID `json:"id"`
	Description string          `json:"description"`
}

// NewUpdateConnectionDescriptionTool loads the tool definition from file and binds service.
// logger may be nil.
func NewUpdateConnectionDescriptionTool(file string, service *servicelocation.Service, logger logger.Logger) (*UpdateConnectionDescriptionTool, error) {
	if service == nil {
		return nil, toolerrs.ErrServiceIsNil
	}

	def, err := actions.LoadDefinition(file)
	if err != nil {
		return nil, err
	}

	return &UpdateConnectionDescriptionTool{service: service, definition: def, logger: logger}, nil
}

// Definition returns a copy of the loaded tool schema.
func (t *UpdateConnectionDescriptionTool) Definition() definition.Definition {
	return t.definition.Copy()
}

// IsIdempotent reports that update_connection_description is safe to retry without journaling.
func (t *UpdateConnectionDescriptionTool) IsIdempotent() bool {
	return true
}

// Execute unmarshals args, replaces connection description, and returns JSON output.
func (t *UpdateConnectionDescriptionTool) Execute(ctx execution.ExecutionContext[journal.Result], args json.RawMessage) (json.RawMessage, error) {
	in, err := t.unmarshal(args)
	if err != nil {
		errorf(t.logger, "failed to execute update_connection_description", "type", "UpdateConnectionDescriptionTool", "func", "Execute", "error", err)
		return nil, err
	}

	if err := t.service.PatchConnectionDescription(ctx, in.ID, in.Description); err != nil {
		err = toolerrs.From(err)
		errorf(t.logger, "failed to execute update_connection_description", "type", "UpdateConnectionDescriptionTool", "func", "Execute", "id", in.ID, "error", err)
		return nil, err
	}

	raw, err := t.marshal(UpdateConnectionDescriptionResult(in))
	if err != nil {
		errorf(t.logger, "failed to execute update_connection_description", "type", "UpdateConnectionDescriptionTool", "func", "Execute", "id", in.ID, "error", err)
		return nil, err
	}

	info(t.logger, "update_connection_description is executed", "type", "UpdateConnectionDescriptionTool", "func", "Execute", "id", in.ID)
	return raw, nil
}

func (t *UpdateConnectionDescriptionTool) unmarshal(args json.RawMessage) (UpdateConnectionDescriptionArgs, error) {
	var result UpdateConnectionDescriptionArgs
	if err := json.Unmarshal(args, &result); err != nil {
		return UpdateConnectionDescriptionArgs{}, toolerrs.NewInvalidRequest(err)
	}
	if result.ID == 0 {
		return UpdateConnectionDescriptionArgs{}, toolerrs.NewInvalidRequest(toolerrs.ErrConnectionIDZero)
	}

	return result, nil
}

func (t *UpdateConnectionDescriptionTool) marshal(output UpdateConnectionDescriptionResult) (json.RawMessage, error) {
	raw, err := json.Marshal(output)
	if err != nil {
		return nil, toolerrs.NewInvalidRequest(err)
	}
	return raw, nil
}
