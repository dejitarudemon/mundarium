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
	_ tools.Tool = (*UpdateLocationDescriptionTool)(nil)
)

// UpdateLocationDescriptionTool replaces a location's description via the locations service.
type UpdateLocationDescriptionTool struct {
	service    *servicelocation.Service
	definition definition.Definition
	logger     logger.Logger
}

// UpdateLocationDescriptionArgs is the JSON payload accepted by update_location_description.
type UpdateLocationDescriptionArgs struct {
	ID          repositories.ID `json:"id"`
	Description string          `json:"description"`
}

// UpdateLocationDescriptionResult is the JSON payload returned by update_location_description.
type UpdateLocationDescriptionResult struct {
	ID          repositories.ID `json:"id"`
	Description string          `json:"description"`
}

// NewUpdateLocationDescriptionTool loads the tool definition from file and binds service.
// logger may be nil.
func NewUpdateLocationDescriptionTool(file string, service *servicelocation.Service, logger logger.Logger) (*UpdateLocationDescriptionTool, error) {
	if service == nil {
		return nil, toolerrs.ErrServiceIsNil
	}

	def, err := actions.LoadDefinition(file)
	if err != nil {
		return nil, err
	}

	return &UpdateLocationDescriptionTool{service: service, definition: def, logger: logger}, nil
}

// Definition returns a copy of the loaded tool schema.
func (t *UpdateLocationDescriptionTool) Definition() definition.Definition {
	return t.definition.Copy()
}

// IsIdempotent reports that update_location_description is safe to retry without journaling.
func (t *UpdateLocationDescriptionTool) IsIdempotent() bool {
	return true
}

// Execute unmarshals args, replaces location description, and returns JSON output.
func (t *UpdateLocationDescriptionTool) Execute(ctx execution.ExecutionContext[journal.Result], args json.RawMessage) (json.RawMessage, error) {
	in, err := t.unmarshal(args)
	if err != nil {
		errorf(t.logger, "failed to execute update_location_description", "type", "UpdateLocationDescriptionTool", "func", "Execute", "error", err)
		return nil, err
	}

	if err := t.service.UpdateLocationDescription(ctx, in.ID, in.Description); err != nil {
		err = toolerrs.From(err)
		errorf(t.logger, "failed to execute update_location_description", "type", "UpdateLocationDescriptionTool", "func", "Execute", "id", in.ID, "error", err)
		return nil, err
	}

	raw, err := t.marshal(UpdateLocationDescriptionResult{ID: in.ID, Description: in.Description})
	if err != nil {
		errorf(t.logger, "failed to execute update_location_description", "type", "UpdateLocationDescriptionTool", "func", "Execute", "id", in.ID, "error", err)
		return nil, err
	}

	info(t.logger, "update_location_description is executed", "type", "UpdateLocationDescriptionTool", "func", "Execute", "id", in.ID)
	return raw, nil
}

func (t *UpdateLocationDescriptionTool) unmarshal(args json.RawMessage) (UpdateLocationDescriptionArgs, error) {
	var result UpdateLocationDescriptionArgs
	if err := json.Unmarshal(args, &result); err != nil {
		return UpdateLocationDescriptionArgs{}, toolerrs.NewInvalidRequest(err)
	}
	if result.ID == 0 {
		return UpdateLocationDescriptionArgs{}, toolerrs.NewInvalidRequest(toolerrs.ErrLocationIDZero)
	}

	return result, nil
}

func (t *UpdateLocationDescriptionTool) marshal(output UpdateLocationDescriptionResult) (json.RawMessage, error) {
	raw, err := json.Marshal(output)
	if err != nil {
		return nil, toolerrs.NewInvalidRequest(err)
	}
	return raw, nil
}
