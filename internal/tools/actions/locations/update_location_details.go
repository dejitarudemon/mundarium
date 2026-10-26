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
	_ tools.Tool = (*UpdateLocationDetailsTool)(nil)
)

// UpdateLocationDetailsTool replaces a location's details via the locations service.
type UpdateLocationDetailsTool struct {
	service    *servicelocation.Service
	definition definition.Definition
	logger     logger.Logger
}

// UpdateLocationDetailsArgs is the JSON payload accepted by update_location_details.
type UpdateLocationDetailsArgs struct {
	ID      repositories.ID   `json:"id"`
	Details repositories.JSON `json:"details"`
}

// UpdateLocationDetailsResult is the JSON payload returned by update_location_details.
type UpdateLocationDetailsResult struct {
	ID      repositories.ID   `json:"id"`
	Details repositories.JSON `json:"details"`
}

// NewUpdateLocationDetailsTool loads the tool definition from file and binds service.
// logger may be nil.
func NewUpdateLocationDetailsTool(file string, service *servicelocation.Service, logger logger.Logger) (*UpdateLocationDetailsTool, error) {
	if service == nil {
		return nil, toolerrs.ErrServiceIsNil
	}

	def, err := actions.LoadDefinition(file)
	if err != nil {
		return nil, err
	}

	return &UpdateLocationDetailsTool{service: service, definition: def, logger: logger}, nil
}

// Definition returns a copy of the loaded tool schema.
func (t *UpdateLocationDetailsTool) Definition() definition.Definition {
	return t.definition.Copy()
}

// IsIdempotent reports that update_location_details is safe to retry without journaling.
func (t *UpdateLocationDetailsTool) IsIdempotent() bool {
	return true
}

// Execute unmarshals args, replaces location details, and returns JSON output.
func (t *UpdateLocationDetailsTool) Execute(ctx execution.ExecutionContext[journal.Result], args json.RawMessage) (json.RawMessage, error) {
	in, err := t.unmarshal(args)
	if err != nil {
		errorf(t.logger, "failed to execute update_location_details", "type", "UpdateLocationDetailsTool", "func", "Execute", "error", err)
		return nil, err
	}

	if err := t.service.UpdateLocationDetails(ctx, in.ID, in.Details); err != nil {
		err = toolerrs.From(err)
		errorf(t.logger, "failed to execute update_location_details", "type", "UpdateLocationDetailsTool", "func", "Execute", "id", in.ID, "error", err)
		return nil, err
	}

	raw, err := t.marshal(UpdateLocationDetailsResult{ID: in.ID, Details: in.Details})
	if err != nil {
		errorf(t.logger, "failed to execute update_location_details", "type", "UpdateLocationDetailsTool", "func", "Execute", "id", in.ID, "error", err)
		return nil, err
	}

	info(t.logger, "update_location_details is executed", "type", "UpdateLocationDetailsTool", "func", "Execute", "id", in.ID)
	return raw, nil
}

func (t *UpdateLocationDetailsTool) unmarshal(args json.RawMessage) (UpdateLocationDetailsArgs, error) {
	var result UpdateLocationDetailsArgs
	if err := json.Unmarshal(args, &result); err != nil {
		return UpdateLocationDetailsArgs{}, toolerrs.NewInvalidRequest(err)
	}
	if result.ID == 0 {
		return UpdateLocationDetailsArgs{}, toolerrs.NewInvalidRequest(toolerrs.ErrLocationIDZero)
	}

	return result, nil
}

func (t *UpdateLocationDetailsTool) marshal(output UpdateLocationDetailsResult) (json.RawMessage, error) {
	raw, err := json.Marshal(output)
	if err != nil {
		return nil, toolerrs.NewInvalidRequest(err)
	}
	return raw, nil
}
